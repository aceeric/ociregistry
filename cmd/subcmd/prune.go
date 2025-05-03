package subcmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/helpers"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// magic numbers from 'format.go' in package 'time'
const dateFormat = "2006-01-02T15:04:05"

// match holds a ManifestHolder matching a prune search
type match struct {
	mh imgpull.ManifestHolder
}

// matches has the map of manifests that matched the prune criteria
var matches = make(map[string]match)

// Prune prunes the cache on the file system. It is intended for use when the server is not
// running. It supports prune by date, or by go regex. Prune by date selects manifests for
// deletion whose create date is earlier than a specified date/time. The date/time
// must be formatted like: '2025-02-28T12:59:59'. Prune by regex accepts a comma-separated
// list of patterns and selects manifests whose urls match the any of the patterns. E.g.:
// 'cert-manger' or 'cilium:v1.15.1' or 'cilium,coredns'.
//
// When an image list manifest is pruned, all related image manifests are also pruned. For
// example, pruning 'nginx:1.14-4' will remove an image list manifest. The pruner will then
// get all the image manifest digests from the image list and if any of those images have
// been cached, they will also be removed.
//
// When an image manifest is removed, then for each blob in the manifest - if no other image
// manifest in cache references that blob, then the blob is also removed from the filesystem.
func Prune() error {
	var handler serialize.CacheEntryHandler
	var err error

	pruneCfg := config.GetPruneConfig()
	switch pruneCfg.Type {
	case "pattern":
		handler, err = patternHandler(pruneCfg.Expr)
	case "date":
		handler, err = dateHandler(pruneCfg.Expr)
	default:
		return fmt.Errorf("unsupported prune type: %q", pruneCfg.Type)
	}
	if err != nil {
		return err
	}

	// populate the 'matches' variable
	if err = serialize.WalkTheCache(config.GetImagePath(), handler); err != nil {
		return err
	}
	return doPrune(config.GetImagePath(), pruneCfg.DryRun, matches)
}

// dateHandler finds manifests whose create date is earlier than the passed date/time
// in the format 'YYYY-MM-DDTHH:MM:SS'.
func dateHandler(date string) (serialize.CacheEntryHandler, error) {
	cutoffDate, err := time.ParseInLocation(dateFormat, date, time.Local)
	if err != nil {
		return nil, err
	}
	return func(mh imgpull.ManifestHolder, fi os.FileInfo) error {
		if mh.Created == "" {
			return nil
		}
		createDate, err := time.ParseInLocation(dateFormat, mh.Created, time.Local)
		if err != nil {
			return err
		}
		if createDate.Before(cutoffDate) {
			matches[mh.ImageUrl] = match{mh}
		}
		return nil

	}, nil
}

// patternHandler finds manifests whose urls match the passed pattern(s). E.g.:
// 'cilium:v1.15.1'. Multiple search patterns are accepted separated by commas.
// E.g.: 'cilium,coredns'.
func patternHandler(pattern string) (serialize.CacheEntryHandler, error) {
	srchs := []*regexp.Regexp{}
	for _, ref := range strings.Split(pattern, ",") {
		fmt.Printf("Compiling search regex: %q\n", ref)
		if exp, err := regexp.Compile(ref); err == nil {
			srchs = append(srchs, exp)
		} else {
			return nil, fmt.Errorf("regex did not compile: %q", ref)
		}
	}
	return func(mh imgpull.ManifestHolder, fi os.FileInfo) error {
		for _, srch := range srchs {
			if srch.MatchString(mh.ImageUrl) {
				matches[mh.ImageUrl] = match{mh}
				break
			}
		}
		return nil
	}, nil
}

// doPrune removes manifests in the passed 'matches' map, along with any blobs that
// can safely be removed for the image manifests in the map. A blob can be safely
// removed if it is not referenced by any image manifest after all the manifest(s)
// in the passed map are removed.
func doPrune(imagePath string, dryRun bool, matches map[string]match) error {
	blobs := serialize.GetAllBlobs(imagePath)
	tallyBlobCount(blobs, imagePath)
	if err := addImageManifests(matches, imagePath); err != nil {
		return err
	}
	if err := decBlobCounts(matches, blobs); err != nil {
		return err
	}
	dryRunMsg := ""
	if dryRun {
		dryRunMsg = " (dry run)"
	}
	fmt.Printf("Prune blobs%s:\n", dryRunMsg)
	for blob, cnt := range blobs {
		if cnt == 0 {
			fmt.Println(blob)
			if !dryRun {
				if err := serialize.RmBlob(imagePath, blob); err != nil {
					return err
				}
			}
		}
	}
	fmt.Printf("Prune manifests%s:\n", dryRunMsg)
	for _, match := range matches {
		fmt.Printf("%s\n", match.mh.ImageUrl)
		if !dryRun {
			if err := serialize.RmManifest(imagePath, match.mh); err != nil {
				return err
			}
		}
	}
	return nil
}

// tallyBlobCount computes the ref counts for all blobs based on cached
// image manifests. The counts are tallied into the passed 'blobs' map.
func tallyBlobCount(blobs map[string]int, imagePath string) {
	serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
		if mh.IsImageManifest() {
			for _, layer := range mh.Layers() {
				digest := helpers.GetDigestFrom(layer.Digest)
				if cnt, exists := blobs[digest]; exists {
					blobs[digest] = cnt + 1
				} else {
					fmt.Printf("warning: blob %q ref'd by manifest URL %q does not exist in the blobs dir\n", digest, mh.ImageUrl)
				}
			}
		}
		return nil
	})
}

// addImageManifests finds all image *list* manifests in the passed 'matches' map, and for
// each, finds cached *image* manifests. This handles the case where a narrow search like
// 'nginx:1.14-4' would find only a manifest *list*, but the intent is to prune all cached
// *images*. The image manifests found by the function are added to the passed matches map.
func addImageManifests(matches map[string]match, imagePath string) error {
	for _, match := range matches {
		if !match.mh.IsImageManifest() {
			for _, sha256 := range match.mh.ImageManifestDigests() {
				pr, err := pullrequest.NewPullRequestFromUrl(match.mh.ImageUrl)
				if err != nil {
					return err
				}
				url := pr.UrlWithDigest(sha256)
				if _, exists := matches[url]; !exists {
					if mh, found := serialize.MhFromFilesystem(sha256, true, imagePath); found {
						matches[url] = struct {
							mh imgpull.ManifestHolder
						}{mh}
					}
				}
			}
		}
	}
	return nil
}

// decBlobCounts iterates image manifests in the passed map (that are about to be deleted), and decrements
// the blob count in the 'blobs' map for those manifests. Those blobs that dec to zero refs can
// be removed.
func decBlobCounts(matches map[string]match, blobs map[string]int) error {
	for _, match := range matches {
		if match.mh.IsImageManifest() {
			for _, layer := range match.mh.Layers() {
				digest := helpers.GetDigestFrom(layer.Digest)
				if cnt, exists := blobs[digest]; exists {
					blobs[digest] = cnt - 1
					if blobs[digest] < 0 {
						return fmt.Errorf("blob %q decremented negative (should never happen)", digest)
					}
				} else {
					return fmt.Errorf("blob %q for manifest %q not found (should never happen)", digest, match.mh.ImageUrl)
				}
			}
		}
	}
	return nil
}
