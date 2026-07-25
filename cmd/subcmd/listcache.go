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
	for ref := range strings.SplitSeq(listCfg.Expr, ",") {
		if exp, err := regexp.Compile(ref); err == nil {
			srchs = append(srchs, exp)
		} else {
			return fmt.Errorf("regex did not compile: %q", ref)
		}
	}
	images := []struct {
		image        string
		tagOrDigest  string
		manifestType string
		created      string
		pulled       string
		size         float32
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
		size := float32(0)
		if mh.IsImageManifest() {
			mt = "image"
			for _, layer := range mh.Layers() {
				size += float32(layer.Size)
			}
		}
		image, tagordigest := splitImageRef(mh.ImageUrl)
		if len(tagordigest) == 64 && listCfg.ShortDigest {
			tagordigest = tagordigest[:10]
		}
		images = append(images, struct {
			image        string
			tagOrDigest  string
			manifestType string
			created      string
			pulled       string
			size         float32
		}{
			image, tagordigest, mt, mh.Created, mh.Pulled, size,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("error listing the cache: %s", err)
	}
	if listCfg.Header {
		fmt.Println("IMAGE TAG/DIGEST TYPE CREATED PULLED SIZE(MB)")
	}
	for _, img := range images {
		size := fmt.Sprintf("%.1f", img.size/1000000)
		if img.size == 0 {
			size = "n/a"
		}
		fmt.Printf("%s %s %s %s %s %s\n", img.image, img.tagOrDigest, img.manifestType, img.created, img.pulled, size)
	}
	return nil
}

// splitImageRef splits "docker.io/coredns/coredns:1.11.1" into "docker.io/coredns/coredns" and "1.11.1", and also splits
// "docker.io/coredns/coredns@sha256:2169b3b9..." into "docker.io/coredns/coredns" and "2169b3b9...".
func splitImageRef(ref string) (repo string, tagOrDigest string) {
	if idx := strings.Index(ref, "@"); idx != -1 {
		// digest form: repo@sha256:abcdef...
		repo = ref[:idx]
		digest := ref[idx+1:]
		digest = strings.TrimPrefix(digest, "sha256:")
		return repo, digest
	}

	// tag form: repo:tag  (split on last colon in case repo has a port, e.g. host:5000/repo:tag)
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		repo = ref[:idx]
		tagOrDigest = ref[idx+1:]
		return repo, tagOrDigest
	}

	// no tag or digest present
	return ref, ""
}
