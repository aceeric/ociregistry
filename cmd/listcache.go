package main

import (
	"fmt"
	"ociregistry/impl/config"
	"ociregistry/impl/serialize"
	"os"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// listCache lists the image cache to the console.
func listCache() error {
	images := []struct {
		ImageUrl     string
		ManifestType string
		modtime      time.Time
	}{}
	err := serialize.WalkTheCache(config.GetImagePath(), func(mh imgpull.ManifestHolder, info os.FileInfo) error {
		mt := "list"
		if mh.IsImageManifest() {
			mt = "image"
		}
		images = append(images, struct {
			ImageUrl     string
			ManifestType string
			modtime      time.Time
		}{
			mh.ImageUrl, mt, info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("error listing the cache: %s", err)
	}

	for _, img := range images {
		fmt.Printf("%s %s %s\n", img.ImageUrl, img.ManifestType, img.modtime.Format(dateFormat))
	}
	return nil
}
