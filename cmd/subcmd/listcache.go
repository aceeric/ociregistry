package subcmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// listCache lists the image cache to the console as it appears on the file system.
func ListCache() error {
	listCfg := config.GetListConfig()
	srchs := []*regexp.Regexp{}
	for _, ref := range strings.Split(listCfg.Expr, ",") {
		if exp, err := regexp.Compile(ref); err == nil {
			srchs = append(srchs, exp)
		} else {
			return fmt.Errorf("regex did not compile: %q", ref)
		}
	}
	images := []struct {
		imageUrl     string
		manifestType string
		created      string
		pulled       string
	}{}
	err := serialize.WalkTheCache(config.GetImagePath(), func(mh imgpull.ManifestHolder, info os.FileInfo) error {
		if len(srchs) != 0 {
			matches := false
			for _, srch := range srchs {
				if srch.MatchString(mh.ImageUrl) {
					matches = true
					break
				}
			}
			if !matches {
				return nil
			}
		}
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
