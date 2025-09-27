package serialize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/helpers"

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
// arg is true. If the 'replace' is false then the function checks the file system first
// and if the manifest already exists, nothing is done. The manifests aren't compared.
// Its a simple "file exists" check. If the manifest does not exist it is written.
func MhToFilesystem(mh imgpull.ManifestHolder, imagePath string, replace bool) error {
	isLatest, err := mh.IsLatest()
	if err != nil {
		return err
	}
	subDir := subDirs[isLatest]
	fname := filepath.Join(imagePath, subDir, mh.Digest)
	if !replace {
		if _, err := os.Stat(fname); err == nil {
			log.Infof("manifest already in cache %q", fname)
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		log.Errorf("unable to create directory %s, error: %s", filepath.Dir(fname), err)
		return err
	}
	mb, err := json.Marshal(mh)
	if err != nil {
		log.Errorf("error marshalling manifest for %q, error: %q", mh.ImageUrl, err)
		return err
	}
	if err := os.WriteFile(fname, mb, 0755); err != nil {
		log.Errorf("error serializing manifest for %q, error: %q", mh.ImageUrl, err)
		return err
	}
	return nil
}

// WalkTheCache walks the image cache and provides each de-serialized ManifestHolder
// to the passed function.
func WalkTheCache(imagePath string, handler CacheEntryHandler) error {
	for _, subpath := range []string{globals.LtsPath, globals.ImgPath} {
		mfpath := filepath.Join(imagePath, subpath)
		err := filepath.Walk(mfpath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			mh := imgpull.ManifestHolder{}
			err = json.Unmarshal(b, &mh)
			if err != nil {
				return err
			}
			err = handler(mh, info)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// RmBlob removes the blob with the passed digest. If the blob file does not exist,
// no error is returned.
func RmBlob(imagePath string, digest string) error {
	blobPath := filepath.Join(imagePath, globals.BlobPath, digest)
	if _, err := os.Stat(blobPath); err == nil {
		return os.Remove(blobPath)
	}
	return nil
}

// BlobExists returns true if the passed blob is on the file system, else false.
func BlobExists(imagePath string, digest string) bool {
	blobPath := filepath.Join(imagePath, globals.BlobPath, digest)
	if _, err := os.Stat(blobPath); err == nil {
		return true
	}
	return false
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
	if _, err := os.Stat(mPath); err == nil {
		return os.Remove(mPath)
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
			blobMap[entry.Name()] = 0
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
