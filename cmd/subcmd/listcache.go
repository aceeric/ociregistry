package subcmd

import (
	"fmt"
	"ociregistry/impl/config"
	"ociregistry/impl/serialize"
	"os"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// listCache lists the image cache to the console.
func ListCache() error {
	listCfg := config.GetListConfig()
	images := []struct {
		imageUrl     string
		manifestType string
		created      string
		pulled       string
	}{}
	err := serialize.WalkTheCache(config.GetImagePath(), func(mh imgpull.ManifestHolder, info os.FileInfo) error {
		mt := "list"
		if mh.IsImageManifest() {
			mt = "image"
		}
		images = append(images, struct {
			imageUrl     string
			manifestType string
			created      string
			pulled       string
		}{
			mh.ImageUrl, mt, mh.Created, mh.Pulled,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("error listing the cache: %s", err)
	}
	if listCfg.Header {
		fmt.Println("URL TYPE CREATED PULLED")
	}
	for _, img := range images {
		fmt.Printf("%s %s %s %s\n", img.imageUrl, img.manifestType, img.created, img.pulled)
	}
	return nil
}
