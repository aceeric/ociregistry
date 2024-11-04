package main

import (
	"fmt"
	"ociregistry/impl/preload"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"slices"
)

// cliCmd defines a function that can run a command represented by a 'cmdLine' struct
type cliCmd func(cmdLine) (bool, error)

// cmdList is a list of CLI commands
var cmdList []cliCmd = []cliCmd{preloadCache, listCache, showVer}

// cliCommands loops through the 'cmdList' array and provides each command with the passed
// 'cmdLine'. If the command executes (meaning the command line matched what the command needs
// on the command line) then the command is expected to return true in the first arg. In that
// case the function terminates the running process using 'os.Exit()'. This implements using the
// server binary as a CLI rather than as a distribution server.
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
	fmt.Printf("image cache:\n\n")
	images := []string{}
	serialize.WalkTheCache(args.imagePath, func(mh upstream.ManifestHolder) error {
		images = append(images, mh.ImageUrl)
		return nil
	})
	slices.Sort(images)
	for _, img := range images {
		fmt.Println(img)
	}
	return true, nil
}
