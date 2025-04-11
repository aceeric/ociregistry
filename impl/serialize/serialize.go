package serialize

import (
	"encoding/json"
	"ociregistry/impl/globals"
	"ociregistry/impl/helpers"
	"os"
	"path/filepath"

	"github.com/aceeric/imgpull/pkg/imgpull"
	log "github.com/sirupsen/logrus"
)

const (
	// fatPath is the subdirectory under the image cache root where the "fat" manifests are
	// stored (meaning the manifests that are lists of image manifests)
	fatPath = "fat"
	// imgPath is the subdirectory under the image cache root where the image manifests are stored
	imgPath = "img"
)

// CacheEntryHandler defines a function that can act on a ManifestHolder instance
// from the metadata cache
type CacheEntryHandler func(imgpull.ManifestHolder, os.FileInfo) error

// MhFromFilesystem gets a ManifestHolder from the file system at the passed path.
// If not found, returns an empty ManifestHolder and false, else the ManifestHolder
// from the file system and true
func MhFromFilesystem(digest string, isImageManifest bool, imagePath string) (imgpull.ManifestHolder, bool) {
	var subdir = fatPath
	if isImageManifest {
		subdir = imgPath
	}
	digest = helpers.GetDigestFrom(digest)
	fname := filepath.Join(imagePath, subdir, digest)
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

// MhToFilesystem writes a ManifestHolder to the file system
func MhToFilesystem(mh imgpull.ManifestHolder, imagePath string, replace bool) error {
	var subdir = fatPath
	if mh.IsImageManifest() {
		subdir = imgPath
	}
	fname := filepath.Join(imagePath, subdir, mh.Digest)
	if !replace {
		if _, err := os.Stat(fname); err == nil {
			log.Infof("manifest already in cache %q", fname)
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		log.Errorf("unable to create directory %q, error: %q", filepath.Dir(fname), err)
		return err
	}
	mb, _ := json.Marshal(mh)
	if err := os.WriteFile(fname, mb, 0755); err != nil {
		log.Errorf("error serializing manifest for %q, error: %q", mh.ImageUrl, err)
		return err
	}
	return nil
}

// WalkTheCache walks the image cache and provides each de-serialized ManifestHolder
// to the passed function.
func WalkTheCache(imagePath string, handler CacheEntryHandler) error {
	for _, subpath := range []string{fatPath, imgPath} {
		mfpath := filepath.Join(imagePath, subpath)
		err := filepath.Walk(mfpath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
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
	blobPath := filepath.Join(imagePath, globals.BlobsDir, digest)
	if _, err := os.Stat(blobPath); err == nil {
		return os.Remove(blobPath)
	}
	return nil
}

// BlobExists returns true if the passed blob is on the file system, else false.
func BlobExists(imagePath string, digest string) bool {
	blobPath := filepath.Join(imagePath, globals.BlobsDir, digest)
	if _, err := os.Stat(blobPath); err == nil {
		return true
	}
	return false
}

// RmManifest removes the passed manifest from the fle system. If the manifest file does not exist,
// no error is returned.
func RmManifest(imagePath string, mh imgpull.ManifestHolder) error {
	subPath := fatPath
	if mh.IsImageManifest() {
		subPath = imgPath
	}
	mPath := filepath.Join(imagePath, subPath, mh.Digest)
	if _, err := os.Stat(mPath); err == nil {
		return os.Remove(mPath)
	}
	return nil
}

// GetAllBlobs returns a map of all blobs on the filesystem with a ref counter initialized to zero.
func GetAllBlobs(imagePath string) map[string]int {
	blobMap := make(map[string]int)
	if entries, err := os.ReadDir(filepath.Join(imagePath, globals.BlobsDir)); err != nil {
		return nil
	} else {
		for _, entry := range entries {
			blobMap[entry.Name()] = 0
		}
	}
	return blobMap
}
