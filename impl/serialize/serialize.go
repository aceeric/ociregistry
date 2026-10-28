package serialize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/metrics"
	"github.com/aceeric/ociregistry/impl/pullrequest"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// manifestName matches the name of a manifest file: the SHA-256 hex of a canonical image url.
// Anything else in the manifests directory (temp files, the migration report and marker) is
// not a manifest.
var manifestName = regexp.MustCompile(`^[a-f0-9]{64}$`)

// CacheEntryHandler defines a function that can act on a ManifestHolder instance
// from the metadata cache
type CacheEntryHandler func(imgpull.ManifestHolder, os.FileInfo) error

// CanonicalUrl returns the canonical form of the passed image url, which is the form used to
// name manifest files. See pullrequest.PullRequest for the canonicalization rules.
func CanonicalUrl(url string) (string, error) {
	pr, err := pullrequest.NewPullRequestFromUrl(url)
	if err != nil {
		return "", err
	}
	return pr.Url(), nil
}

// manifestFile returns the path of the file that holds the manifest for the passed canonical
// image url: the SHA-256 hex of the url, in the manifests directory. Hashing the url avoids
// every problem with using it as a path: repository/tag collisions, path traversal from
// request input, case-insensitive file systems and file name length limits. Because the name
// depends on the exact canonical url, changing the canonicalization rules requires a cache
// migration.
func manifestFile(imagePath string, url string) string {
	return filepath.Join(imagePath, globals.ManifestPath, UrlDigest(url))
}

// UrlDigest returns the SHA-256 hex of the passed canonical image url, which is the name of
// the file that holds the manifest for the url.
func UrlDigest(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}

// MhFromFilesystem gets the ManifestHolder for the passed image url from the file system.
// The url is canonicalized first, so for example docker.io/hello-world:latest finds the
// manifest cached as docker.io/library/hello-world:latest. If not found, returns an empty
// ManifestHolder and false, else the ManifestHolder from the file system and true.
func MhFromFilesystem(url string, imagePath string) (imgpull.ManifestHolder, bool) {
	canonical, err := CanonicalUrl(url)
	if err != nil {
		return imgpull.ManifestHolder{}, false
	}
	b, err := os.ReadFile(manifestFile(imagePath, canonical))
	if err != nil {
		return imgpull.ManifestHolder{}, false
	}
	mh := imgpull.ManifestHolder{}
	if err := json.Unmarshal(b, &mh); err != nil {
		return imgpull.ManifestHolder{}, false
	}
	return mh, true
}

// MhToFilesystem writes the passed ManifestHolder to the file system, in the file for its
// ImageUrl, if the 'replace' arg is true. If the 'replace' arg is false then the function
// checks the file system first and if the manifest already exists, nothing is done. The
// ImageUrl is canonicalized before writing, so the file name and the stored url always agree.
// The write is atomic: the file holds either the previous content or the new content, never
// a partial write.
func MhToFilesystem(mh imgpull.ManifestHolder, imagePath string, replace bool) error {
	canonical, err := CanonicalUrl(mh.ImageUrl)
	if err != nil {
		return err
	}
	mh.ImageUrl = canonical
	fname := manifestFile(imagePath, canonical)
	existingMfstBytes := 0
	if fi, err := os.Stat(fname); err == nil {
		if !replace {
			log.Infof("manifest already in cache %q", canonical)
			return nil
		}
		existingMfstBytes = int(fi.Size())
	}
	mb, err := json.Marshal(mh)
	if err != nil {
		log.Errorf("error marshalling manifest for %q, error: %q", mh.ImageUrl, err)
		return err
	}
	if err := writeFileAtomic(fname, mb, 0644); err != nil {
		log.Errorf("error serializing manifest for %q, error: %q", mh.ImageUrl, err)
		return err
	}
	metrics.DeltaManifestBytesOnDisk(float64(len(mb) - existingMfstBytes))
	return nil
}

// WalkTheCache walks the manifests directory and provides each de-serialized ManifestHolder
// to the passed function. Only manifest files are read (see manifestName). Files that can't
// be read or parsed are logged and skipped so one bad file doesn't prevent the rest of the
// cache from loading. An error returned by the handler stops the walk and is returned.
func WalkTheCache(imagePath string, handler CacheEntryHandler) error {
	dir := filepath.Join(imagePath, globals.ManifestPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !manifestName.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			log.Errorf("skipping unreadable manifest file %q: %s", path, err)
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			log.Errorf("skipping unreadable manifest file %q: %s", path, err)
			continue
		}
		mh := imgpull.ManifestHolder{}
		if err := json.Unmarshal(b, &mh); err != nil {
			log.Errorf("skipping invalid manifest file %q: %s", path, err)
			continue
		}
		if err := handler(mh, info); err != nil {
			return err
		}
	}
	return nil
}

// RemoveTempFiles deletes temp files left by interrupted atomic writes in the manifest
// and blob directories. Call it at startup, before loading the cache or serving requests.
// The legacy manifest directories are never modified.
func RemoveTempFiles(imagePath string) error {
	for _, sub := range []string{globals.ManifestPath, globals.BlobPath} {
		matches, err := filepath.Glob(filepath.Join(imagePath, sub, "*.tmp"))
		if err != nil {
			return err
		}
		for _, m := range matches {
			if err := os.Remove(m); err != nil {
				return err
			}
			log.Infof("removed leftover temp file %q", m)
		}
	}
	return nil
}

// RmBlob removes the blob with the passed digest. If the blob file does not exist,
// no error is returned. Blob metrics are maintained by the cache ref counts, not here.
func RmBlob(imagePath string, digest string) error {
	err := os.Remove(filepath.Join(imagePath, globals.BlobPath, digest))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// BlobExists returns true if the passed blob is on the file system, else false.
func BlobExists(imagePath string, digest string) (bool, int) {
	blobPath := filepath.Join(imagePath, globals.BlobPath, digest)
	if fi, err := os.Stat(blobPath); err == nil {
		return true, int(fi.Size())
	}
	return false, 0
}

// RmManifest removes the file for the passed manifest's ImageUrl from the file system. If the
// manifest file does not exist, no error is returned.
func RmManifest(imagePath string, mh imgpull.ManifestHolder) error {
	canonical, err := CanonicalUrl(mh.ImageUrl)
	if err != nil {
		return err
	}
	mPath := manifestFile(imagePath, canonical)
	if fi, err := os.Stat(mPath); err == nil {
		if err := os.Remove(mPath); err != nil {
			return err
		}
		metrics.DeltaManifestBytesOnDisk(float64(0 - fi.Size()))
	}
	return nil
}

// GetAllBlobs returns a map of all blobs on the filesystem with a ref counter initialized to zero.
func GetAllBlobs(imagePath string) map[string]int {
	blobMap := make(map[string]int)
	if entries, err := os.ReadDir(filepath.Join(imagePath, globals.BlobPath)); err != nil {
		return nil
	} else {
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".tmp") {
				blobMap[entry.Name()] = 0
			}
		}
	}
	return blobMap
}

// CreateDirs creates the required subdirectory structure under the passed root if they do
// not already exists, and if writeable is true, then ensures that the root is writeable
// by the current process. The legacy manifest directories are not created.
func CreateDirs(root string, writeable bool) error {
	for _, subDir := range []string{globals.ManifestPath, globals.BlobPath} {
		if absPath, err := filepath.Abs(filepath.Join(root, subDir)); err == nil {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return err
			}
		}
	}
	if writeable {
		pt := filepath.Join(root, ".permtest")
		defer os.Remove(pt)
		if _, err := os.Create(pt); err != nil {
			return fmt.Errorf("directory %s is not writable", root)
		}
	}
	return nil
}

// writeFileAtomic writes data to a temp file in the same directory as fname, then renames
// it to fname. On any error the temp file is removed and fname is left untouched.
func writeFileAtomic(fname string, data []byte, perm os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(fname), filepath.Base(fname)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(perm); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), fname)
}
