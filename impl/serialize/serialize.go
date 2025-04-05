package serialize

import (
	"encoding/json"
	"ociregistry/impl/globals"
	"ociregistry/impl/memcache"
	"ociregistry/impl/upstream"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// CacheEntryHandler defines a function that can act on a 'ManifestHolder' instance
// from the metadata cache
type CacheEntryHandler func(upstream.ManifestHolder, os.FileInfo) error

// MhFromFileSystem gets a 'ManifestHolder' from the file system at the passed path.
// If not found, returns an empty 'ManifestHolder' and false, else the 'ManifestHolder'
// from the file system and true
func MhFromFileSystem(digest string, isImageManifest bool, imagePath string) (upstream.ManifestHolder, bool) {
	var subdir = fatPath
	if isImageManifest {
		subdir = imgPath
	}
	if strings.HasPrefix(digest, "sha256:") {
		digest = strings.Split(digest, ":")[1]
	}
	fname := filepath.Join(imagePath, subdir, digest)
	_, err := os.Stat(fname)
	if err == nil {
		b, err := os.ReadFile(fname)
		if err != nil {
			return upstream.ManifestHolder{}, false
		}
		mh := upstream.ManifestHolder{}
		err = json.Unmarshal(b, &mh)
		if err == nil {
			return mh, true
		}
	}
	return upstream.ManifestHolder{}, false
}

// TODO DELETE OLD
func ToFilesystemNEW(mh imgpull.ManifestHolder, imagePath string) error {
	var subdir = fatPath
	// TODO mh.IsImageManifest()
	if !mh.IsManifestList() {
		subdir = imgPath
	}
	fname := filepath.Join(imagePath, subdir, mh.Digest)
	if _, err := os.Stat(fname); err == nil {
		log.Infof("manifest already in cache %s", fname)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		log.Errorf("unable to create directory %s, error: %s", filepath.Dir(fname), err)
		return err
	}
	mb, _ := json.Marshal(mh)
	if err := os.WriteFile(fname, mb, 0755); err != nil {
		log.Errorf("error serializing manifest for %s, error: %s", mh.ImageUrl, err)
		return err
	}
	return nil
}

// ToFilesystem serializes the passed 'ManifestHolder' to the file system at
// the passed image path. If the file already exists, no action is taken.
func ToFilesystem(mh upstream.ManifestHolder, imagePath string) error {
	var subdir = fatPath
	if mh.IsImageManifest() {
		subdir = imgPath
	}
	fname := filepath.Join(imagePath, subdir, mh.Digest)
	if _, err := os.Stat(fname); err == nil {
		log.Infof("manifest already in cache %s", fname)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		log.Errorf("unable to create directory %s, error: %s", filepath.Dir(fname), err)
		return err
	}
	mb, _ := json.Marshal(mh)
	if err := os.WriteFile(fname, mb, 0755); err != nil {
		log.Errorf("error serializing manifest for %s, error: %s", mh.ImageUrl, err)
		return err
	}
	return nil
}

// FromFilesystem reads all the manifests from the file system and adds them
// the the in-memory data structure that represents the cache in memory.
func FromFilesystem(imagePath string) error {
	start := time.Now()
	log.Infof("load in-mem cache from file system")
	itemcnt := 0
	WalkTheCache(imagePath, func(mh upstream.ManifestHolder, _ os.FileInfo) error {
		memcache.AddToCache(mh.Pr, mh, false)
		log.Debugf("loading manifest for %s", mh.ImageUrl)
		itemcnt++
		return nil
	})
	log.Infof("loaded %d manifest(s) from the file system in %s", itemcnt, time.Since(start))
	return nil
}

// WalkTheCache walks the image cache and provides each de-serialized 'ManifestHolder'
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
			mh := upstream.ManifestHolder{}
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

// RmManifest removes the passed manifest. If the manifest file does not exist,
// no error is returned.
func RmManifest(imagePath string, mh upstream.ManifestHolder) error {
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

// GetAllBlobs returns a map of blobs with a counter (set to zero). The intent is
// for the caller to tally blob reference counts into the map.
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
