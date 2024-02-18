package serialize

import (
	"encoding/json"
	"ociregistry/impl/memcache"
	"ociregistry/impl/upstream"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

func ToFilesystem(mh upstream.ManifestHolder, imagePath string) {
	var subdir = "mflst"
	if mh.IsImageManifest() {
		subdir = "imgmf"
	}
	fname := filepath.Join(imagePath, subdir, mh.Digest)
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		log.Errorf("unable to create directory %s", fname)
	}
	_, err := os.Stat(fname)
	if err != nil {
		mb, _ := json.Marshal(mh)
		if os.WriteFile(fname, mb, 0644) != nil {
			log.Errorf("rerror serializing manifest for %s", mh.ImageUrl)
		}
		return
	}
}

func FromFilesystem(prc *memcache.PRCache, imagePath string) error {
	start := time.Now()
	log.Infof("load in-mem cache from file system")
	itemcnt := 0
	for _, subpath := range []string{"mflst", "imgmf"} {
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
			memcache.AddToCacheWithoutLock(mh.Pr, mh)
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
