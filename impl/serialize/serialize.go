package serialize

import (
	"encoding/json"
	"ociregistry/impl/memcache"
	"ociregistry/impl/upstream"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	fatPath = "fat"
	imgPath = "img"
)

type CacheEntryHandler func(upstream.ManifestHolder) error

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

func ToFilesystem(mh upstream.ManifestHolder, imagePath string) error {
	var subdir = fatPath
	if mh.IsImageManifest() {
		subdir = imgPath
	}
	fname := filepath.Join(imagePath, subdir, mh.Digest)
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		log.Errorf("unable to create directory %s", filepath.Dir(fname))
	}
	_, err := os.Stat(fname)
	if err != nil {
		mb, _ := json.Marshal(mh)
		err = os.WriteFile(fname, mb, 0755)
		if err != nil {
			log.Errorf("error serializing manifest for %s", mh.ImageUrl)
		}
		return err
	} else {
		log.Infof("manifest already in cache %s", fname)
	}
	return nil
}

// FromFilesystem reads the manifests from the file system and adds them
// the the in-memory data structure that represents the cache in memory.
func FromFilesystem(imagePath string) error {
	start := time.Now()
	log.Infof("load in-mem cache from file system")
	itemcnt := 0
	WalkTheCache(imagePath, func(mh upstream.ManifestHolder) error {
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
			err = handler(mh)
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
