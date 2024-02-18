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

func IsOnFilesystem(digest string, isImageManifest bool, imagePath string) bool {
	var subdir = fatPath
	if isImageManifest {
		subdir = imgPath
	}
	if strings.HasPrefix(digest, "sha256:") {
		digest = strings.Split(digest, ":")[1]
	}
	fname := filepath.Join(imagePath, subdir, digest)
	_, err := os.Stat(fname)
	return err == nil
}

// todo return error
func ToFilesystem(mh upstream.ManifestHolder, imagePath string) {
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
		if os.WriteFile(fname, mb, 0644) != nil {
			log.Errorf("error serializing manifest for %s", mh.ImageUrl)
		}
		return
	} else {
		log.Infof("manifest already in cache %s", fname)
	}
}

func FromFilesystem(prc *memcache.PRCache, imagePath string) error {
	start := time.Now()
	log.Infof("load in-mem cache from file system")
	itemcnt := 0
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
			memcache.AddToCache(mh.Pr, mh, false)
			log.Debugf("loading manifest for %s", mh.ImageUrl)
			itemcnt++
			return nil
		})
		if err != nil {
			return err
		}
	}
	log.Infof("loaded %d manifest(s) from the file system in %s", itemcnt, time.Since(start))
	return nil
}
