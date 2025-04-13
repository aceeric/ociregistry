package main

import (
	"fmt"
	"ociregistry/impl/helpers"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"os"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// magic numbers from 'format.go' in package 'time'
const dateFormat = "2006-01-02T15:04:05"

// match holds a ManifestHolder matching a prune search
type match struct {
	mh imgpull.ManifestHolder
}

// // pruneBefore removes manifests and blobs matching the command line arg '--prune-before'.
// // E.g.: '--prune-before 2025-02-28T12:59:59'.
// func pruneBefore() (bool, error) {
// 	timestamp, err := time.ParseInLocation(dateFormat, args.pruneBefore, time.Local)
// 	if err != nil {
// 		return true, err
// 	}
// 	// build a list of matching manifests
// 	matches := make(map[string]match)
// 	serialize.WalkTheCache(args.imagePath, func(mh imgpull.ManifestHolder, fi os.FileInfo) error {
// 		if fi.ModTime().Before(timestamp) {
// 			matches[mh.ImageUrl] = struct {
// 				mh imgpull.ManifestHolder
// 			}{mh}
// 		}
// 		return nil
// 	})
// 	return true, doPrune(args.imagePath, args.dryRun, matches)
// }

// // prunePattern removes manifests and blobs matching the command line arg '--prune'.
// // E.g.: '--prune cilium:v1.15.1' or '--prune cilium,coredns'.
// func prunePattern() (bool, error) {
// 	// handle multiple search expressions separated by comma
// 	srchs := []*regexp.Regexp{}
// 	for _, ref := range strings.Split(args.prune, ",") {
// 		fmt.Printf("Compiling search regex: %q\n", ref)
// 		if exp, err := regexp.Compile(ref); err == nil {
// 			srchs = append(srchs, exp)
// 		} else {
// 			return true, fmt.Errorf("regex did not compile: %q", ref)
// 		}
// 	}
// 	// build a list of matching manifests
// 	matches := make(map[string]match)
// 	serialize.WalkTheCache(args.imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
// 		for _, srch := range srchs {
// 			if srch.MatchString(mh.ImageUrl) {
// 				matches[mh.ImageUrl] = struct {
// 					mh imgpull.ManifestHolder
// 				}{mh}
// 			}
// 		}
// 		return nil
// 	})
// 	return true, doPrune(args.imagePath, args.dryRun, matches)
// }

// doPrune removes manifests in the passed 'matches' map, along with any blobs that
// can safely be removed for the image manifests in the map. A blob can be safely
// removed if it is not referenced by any image manifest after the manifest(s) in
// the map are removed.
func doPrune(imagePath string, dryRun bool, matches map[string]match) error {
	// build a list of all blobs. This allows pruning orphaned blobs for free
	blobs := serialize.GetAllBlobs(imagePath)

	// tally the blob ref counts of all cached images into the 'blobs' map
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

	// for all image *list* manifests in the match list, find cached *image* manifests.
	// Handles the case where a narrow search like '--prune nginx:1.14-4' would find
	// only a manifest *list*, but the intent is to prune all cached *images*.
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

	// for all matching image manifests (that are about to be deleted), decrement
	// the blob count in the 'blobs' map. Those blobs that dec to zero refs will be
	// removed below.
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
