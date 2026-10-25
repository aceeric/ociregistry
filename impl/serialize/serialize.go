package serialize

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/metrics"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

// subDirs allows getting the correct subdirectory name for manifests based on whether
// a manifest is (or is not) "latest".
var subDirs = map[bool]string{true: globals.LtsPath, false: globals.ImgPath}

// CacheEntryHandler defines a function that can act on a ManifestHolder instance
// from the metadata cache
type CacheEntryHandler func(imgpull.ManifestHolder, os.FileInfo) error

// MhFromFilesystem gets a ManifestHolder from the file system at the passed path.
// If not found, returns an empty ManifestHolder and false, else the ManifestHolder
// from the file system and true.
func MhFromFilesystem(digest string, isLatest bool, imagePath string) (imgpull.ManifestHolder, bool) {
	subDir := subDirs[isLatest]
	digest = helpers.GetDigestFrom(digest)
	fname := filepath.Join(imagePath, subDir, digest)
	if _, err := os.Stat(fname); err == nil {
		b, err := os.ReadFile(fname)
		if err != nil {
			return imgpull.ManifestHolder{}, false
		}
		mh := imgpull.ManifestHolder{}
		err = json.Unmarshal(b, &mh)
		if err == nil {
			return mh, true
		}
	}
	return imgpull.ManifestHolder{}, false
}

// MhToFilesystem writes the passed ManifestHolder to the file system if the 'replace'
// arg is true. If the 'replace' arg is false then the function checks the file system first
// and if the manifest already exists, nothing is done. The write is atomic: the file holds
// either the previous content or the new content, never a partial write.
func MhToFilesystem(mh imgpull.ManifestHolder, imagePath string, replace bool) error {
	isLatest, err := mh.IsLatest()
	if err != nil {
		return err
	}
	fname := filepath.Join(imagePath, subDirs[isLatest], mh.Digest)
	existingMfstBytes := 0
	if fi, err := os.Stat(fname); err == nil {
		if !replace {
			log.Infof("manifest already in cache %q", fname)
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

// WalkTheCache walks the image cache and provides each de-serialized ManifestHolder
// to the passed function. Files that can't be read or parsed are logged and skipped
// so one bad file doesn't prevent the rest of the cache from loading.
func WalkTheCache(imagePath string, handler CacheEntryHandler) error {
	for _, subpath := range []string{globals.LtsPath, globals.ImgPath} {
		mfpath := filepath.Join(imagePath, subpath)
		err := filepath.Walk(mfpath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || strings.HasSuffix(path, ".tmp") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				log.Errorf("skipping unreadable manifest file %q: %s", path, err)
				return nil
			}
			mh := imgpull.ManifestHolder{}
			if err := json.Unmarshal(b, &mh); err != nil {
				log.Errorf("skipping invalid manifest file %q: %s", path, err)
				return nil
			}
			return handler(mh, info)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveTempFiles deletes temp files left by interrupted atomic writes in the manifest
// and blob directories. Call it at startup, before loading the cache or serving requests.
func RemoveTempFiles(imagePath string) error {
	for _, sub := range []string{globals.LtsPath, globals.ImgPath, globals.BlobPath} {
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

// RmManifest removes the passed manifest from the fle system. If the manifest file does not exist,
// no error is returned.
func RmManifest(imagePath string, mh imgpull.ManifestHolder) error {
	isLatest, err := mh.IsLatest()
	if err != nil {
		return err
	}
	subDir := subDirs[isLatest]
	mPath := filepath.Join(imagePath, subDir, mh.Digest)
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
// by the current process.
func CreateDirs(root string, writeable bool) error {
	for _, subDir := range []string{globals.LtsPath, globals.ImgPath, globals.BlobPath} {
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
