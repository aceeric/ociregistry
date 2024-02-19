package main

import (
	"fmt"
	"ociregistry/impl/preload"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"slices"
)

type cliCmd func(cmdLine) (bool, error)

var cmdList []cliCmd = []cliCmd{preloadCache, listCache}

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

func preloadCache(args cmdLine) (bool, error) {
	if args.loadImages == "" {
		return false, nil
	}
	return true, preload.Preload(args.loadImages, args.imagePath, args.arch, args.os, args.pullTimeout)
}

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
