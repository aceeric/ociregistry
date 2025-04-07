package main

import (
	"encoding/json"
	"fmt"
	"ociregistry/impl/helpers"
	"ociregistry/impl/preload"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// magic numbers from 'format.go' in package 'time'
const dateFormat = "2006-01-02T15:04:05"

// cliCmd defines a function that can run a command represented by a 'cmdLine' struct
type cliCmd func(cmdLine) (bool, error)

// match holds a ManifestHolder matching a prune search
type match struct {
	mh imgpull.ManifestHolder
}

// cmdList is a list of CLI commands
var cmdList []cliCmd = []cliCmd{preloadCache, listCache, showVer, prunePattern, pruneBefore, fix}

// cliCommands loops through the 'cmdList' array and provides each command with the passed
// 'cmdLine'. If the command executes (meaning the command line matched what the command needs
// on the command line) then the command is expected to return true in the first arg. In that
// case the function terminates the running process using 'os.Exit()' and will therefore not
// return. If the function returns then that means none of the CLI commands supported by the
// server were invoked. This implements using the server binary as a CLI rather than as a
// distribution server.
func cliCommands(args cmdLine) {
	for _, f := range cmdList {
		ran, err := f(args)
		if ran {
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
}

// showVer shows the version number
func showVer(args cmdLine) (bool, error) {
	if !args.version {
		return false, nil
	}
	fmt.Printf("ociregistry version: %s build date: %s\n", args.buildVer, args.buildDtm)
	return true, nil
}

// preloadCache loads the image cache
func preloadCache(args cmdLine) (bool, error) {
	if args.loadImages == "" {
		return false, nil
	}
	return true, preload.Preload(args.loadImages, args.imagePath, args.arch, args.os, args.pullTimeout, args.concurrent)
}

// listCache lists the image cache
func listCache(args cmdLine) (bool, error) {
	if !args.listCache {
		return false, nil
	}
	images := []struct {
		ImageUrl     string
		ManifestType string
		modtime      time.Time
	}{}
	serialize.WalkTheCache(args.imagePath, func(mh imgpull.ManifestHolder, info os.FileInfo) error {
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

	for _, img := range images {
		fmt.Printf("%s %s %s\n", img.ImageUrl, img.ManifestType, img.modtime.Format(dateFormat))
	}
	return true, nil
}

// pruneBefore removes manifests and blobs matching the command line arg '--prune-before'.
// E.g.: '--prune-before 2025-02-28T12:59:59'.
func pruneBefore(args cmdLine) (bool, error) {
	if args.pruneBefore == "" {
		return false, nil
	}
	timestamp, err := time.ParseInLocation(dateFormat, args.pruneBefore, time.Local)
	if err != nil {
		return true, err
	}
	// build a list of matching manifests
	matches := make(map[string]match)
	serialize.WalkTheCache(args.imagePath, func(mh imgpull.ManifestHolder, fi os.FileInfo) error {
		if fi.ModTime().Before(timestamp) {
			matches[mh.ImageUrl] = struct {
				mh imgpull.ManifestHolder
			}{mh}
		}
		return nil
	})
	return true, doPrune(args.imagePath, args.dryRun, matches)
}

// prunePattern removes manifests and blobs matching the command line arg '--prune'.
// E.g.: '--prune cilium:v1.15.1' or '--prune cilium,coredns'.
func prunePattern(args cmdLine) (bool, error) {
	if args.prune == "" {
		return false, nil
	}
	// handle multiple search expressions separated by comma
	srchs := []*regexp.Regexp{}
	for _, ref := range strings.Split(args.prune, ",") {
		fmt.Printf("Compiling search regex: %q\n", ref)
		if exp, err := regexp.Compile(ref); err == nil {
			srchs = append(srchs, exp)
		} else {
			return true, fmt.Errorf("regex did not compile: %q", ref)
		}
	}
	// build a list of matching manifests
	matches := make(map[string]match)
	serialize.WalkTheCache(args.imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
		for _, srch := range srchs {
			if srch.MatchString(mh.ImageUrl) {
				matches[mh.ImageUrl] = struct {
					mh imgpull.ManifestHolder
				}{mh}
			}
		}
		return nil
	})
	return true, doPrune(args.imagePath, args.dryRun, matches)
}

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

// DELETEME
func fix(args cmdLine) (bool, error) {
	if args.fix == "" {
		return false, nil
	}
	mh, found := serialize.MhFromFilesystem(args.fix, true, args.imagePath)
	if !found {
		return true, fmt.Errorf("manifest not found for digest: %q", args.fix)
	}
	json.Unmarshal(mh.Bytes, &mh.V1ociManifest)
	path := filepath.Join(args.imagePath, "img", args.fix)
	bytes, _ := json.Marshal(mh)
	os.WriteFile(path, bytes, 0755)
	return true, nil
}
