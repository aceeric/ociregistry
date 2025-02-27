package main

import (
	"fmt"
	"ociregistry/impl/preload"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"regexp"
	"strings"
	"time"
)

// cliCmd defines a function that can run a command represented by a 'cmdLine' struct
type cliCmd func(cmdLine) (bool, error)

// cmdList is a list of CLI commands
var cmdList []cliCmd = []cliCmd{preloadCache, listCache, showVer, pruneCache}

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
	serialize.WalkTheCache(args.imagePath, func(mh upstream.ManifestHolder, info os.FileInfo) error {
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
		fmt.Printf("%s %s %s\n", img.ImageUrl, img.ManifestType, img.modtime.Format(time.RFC3339))
	}
	return true, nil
}

// pruneCache removes manifests and blobs matching the command line arg '--prune'.
// E.g.: '--prune cilium:v1.15.1'.
func pruneCache(args cmdLine) (bool, error) {
	if args.prune == "" {
		return false, nil
	}
	matches := make(map[string]struct{ mh upstream.ManifestHolder })

	// handle multiple search expressions separated by comma
	srchs := []*regexp.Regexp{}
	for _, ref := range strings.Split(args.prune, ",") {
		if exp, err := regexp.Compile(ref); err == nil {
			srchs = append(srchs, exp)
		} else {
			return true, fmt.Errorf("regex did not compile: %q", ref)
		}
	}
	// build a list of matching manifests
	serialize.WalkTheCache(args.imagePath, func(mh upstream.ManifestHolder, _ os.FileInfo) error {
		for _, srch := range srchs {
			if srch.MatchString(mh.ImageUrl) {
				matches[mh.ImageUrl] = struct {
					mh upstream.ManifestHolder
				}{mh}
			}
		}
		return nil
	})
	return true, doPrune(args.imagePath, args.dryRun, matches)
}

// doPrune removes manifests in the passed 'matches' map, along with any blobs that
// can safely be removed for the image manifests in the map. A blob can be safely
// deleted if it is not referenced by any image manifest except the manifest(s)
// in the map.
func doPrune(imagePath string, dryRun bool, matches map[string]struct{ mh upstream.ManifestHolder }) error {
	blobs := make(map[string]int)

	// tally the blob counts into the 'blobs' map
	serialize.WalkTheCache(imagePath, func(mh upstream.ManifestHolder, _ os.FileInfo) error {
		if mh.IsImageManifest() {
			for _, blob := range mh.ManifestBlobs() {
				if cnt, exists := blobs[blob]; exists {
					blobs[blob] = cnt + 1
				} else {
					blobs[blob] = 1
				}
			}
		}
		return nil
	})

	// for all image *list* manifests in the matches, find cached *image* manifests.
	// Handles the case where a narrow search like '--prune nginx:1.14-4' would find
	// only a manifest *list*, but the intent is to prune all cached *images*.
	for _, match := range matches {
		if !match.mh.IsImageManifest() {
			for _, sha256 := range match.mh.ImageManifestDigests() {
				url := match.mh.Pr.UrlWithDigest(sha256)
				if _, exists := matches[url]; !exists {
					if mh, found := serialize.MhFromFileSystem(sha256, true, imagePath); found {
						matches[url] = struct {
							mh upstream.ManifestHolder
						}{mh}
					}
				}
			}
		}
	}

	// for all matching image manifests (that are about to be deleted), decrement
	// the blob count in the 'blobs' map.
	for _, match := range matches {
		if match.mh.IsImageManifest() {
			for _, blob := range match.mh.ManifestBlobs() {
				if cnt, exists := blobs[blob]; exists {
					blobs[blob] = cnt - 1
				} else {
					return fmt.Errorf("blob %q for manifest %q not found (should never happen)", blob, match.mh.ImageUrl)
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
