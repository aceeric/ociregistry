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

//func hack(args cmdLine) bool {
//	d := []string{
//		"072a13fa7539abff5e30d352024b982462c9bde02c4835da0526cb70d1571b32",
//		"4ac9df488c1a0a7dbd4c4dabd67391c92c9704f33f144b999362bfdbec1b9645",
//		"634495ad47e6330378da8bee68b8d17d77d0cb69af2e8c6b005a65df387f3d52",
//		"6e75a10070b0fcb0bead763c5118a369bc7cc30dfc1b0749c491bbb21f15c3c7",
//		"a1fbaea309fa27bad418200539a69cffb4c9336fe1a6b0af23874cd15293c8f8",
//		"ac3313a42b06f781de83dd48552f096205e3c43d9e41c32388af1e7b9a9a794d",
//		"e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57",
//	}
//	for _, dd := range d {
//		mh, _ := serialize.MhFromFileSystem(dd, true, args.imagePath)
//		json.Unmarshal(mh.Bytes, &mh.V1ociManifest)
//		path := "/home/eace/tmp/" + dd
//		bytes, _ := json.Marshal(mh)
//		os.WriteFile(path, bytes, 0755)
//	}
//	return true
//}

func pruneCache(args cmdLine) (bool, error) {
	if args.prune == "" {
		return false, nil
	}
	matches := make(map[string]struct{ mh upstream.ManifestHolder })
	blobs := make(map[string]int)

	// TODO GOING TO NEED THE FILE INFO AND ALSO ADDED TO MhFromFileSystem !!

	// process comma-separated search patterns like --prune cert-manager,e2e-test-images
	srchs := []*regexp.Regexp{}
	for _, ref := range strings.Split(args.prune, ",") {
		if exp, err := regexp.Compile(ref); err == nil {
			srchs = append(srchs, exp)
		} else {
			return true, fmt.Errorf("regex did not compile: %q", ref)
		}
	}

	// build a list of all manifests that match all searches
	serialize.WalkTheCache(args.imagePath, func(mh upstream.ManifestHolder, _ os.FileInfo) error {
		// if image manifest, tally the blob count
		if mh.IsImageManifest() {
			for _, blob := range mh.ManifestBlobs() {
				if cnt, exists := blobs[blob]; exists {
					blobs[blob] = cnt + 1

				} else {
					blobs[blob] = 1
				}
			}
		}
		// build a list of matching manifests
		for _, srch := range srchs {
			if srch.MatchString(mh.ImageUrl) {
				matches[mh.ImageUrl] = struct {
					mh upstream.ManifestHolder
				}{mh}
			}
		}
		return nil
	})

	// for all image *list* manifests in the results, find cached *image* manifests.
	// Handles the case where a narrow search like '--prune nginx:1.14-4' would find
	// only a manifest *list*, but the intent is to prune all cached *images*.
	for _, match := range matches {
		if !match.mh.IsImageManifest() {
			for _, sha256 := range match.mh.ImageManifestDigests() {
				url := match.mh.Pr.UrlWithDigest(sha256)
				if _, exists := matches[url]; !exists {
					if mh, found := serialize.MhFromFileSystem(sha256, true, args.imagePath); found {
						matches[url] = struct {
							mh upstream.ManifestHolder
						}{mh}
					}
				}
			}
		}
	}

	// for all matching image manifests (that are about to be deletes), decrement
	// the blob count
	for _, match := range matches {
		if match.mh.IsImageManifest() {
			for _, blob := range match.mh.ManifestBlobs() {
				if cnt, exists := blobs[blob]; exists {
					blobs[blob] = cnt - 1
				} else {
					return true, fmt.Errorf("blob %q for manifest %q not found (should never happen)", blob, match.mh.ImageUrl)
				}
			}
		}
	}

	dryRun := ""
	if args.dryRun {
		dryRun = " (dry run)"
	}
	fmt.Printf("Prune blobs%s:\n", dryRun)
	for blob, cnt := range blobs {
		if cnt == 0 {
			fmt.Println(blob)
			if !args.dryRun {
				// actually delete blob
			}
		}
	}
	fmt.Printf("Prune manifests%s:\n", dryRun)
	for _, match := range matches {
		fmt.Printf("%s\n", match.mh.ImageUrl)
		if !args.dryRun {
			// actually delete manifest
		}
	}
	return true, nil
}
