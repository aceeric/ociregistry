package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ociregistry/impl/config"
	"ociregistry/impl/globals"
	"ociregistry/impl/preload"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// set by the compiler (see the Makefile):
var (
	buildVer string
	buildDtm string // UTC
)

const (
	serveCmd   string = "serve"
	loadCmd    string = "load"
	listCmd    string = "list"
	pruneCmd   string = "prune"
	versionCmd string = "version"
)

// main is the entry point for the program
func main() {
	if command, err := getCfg(); err != nil {
		fmt.Fprintf(os.Stderr, "error getting configuration: %s\n", err)
		os.Exit(1)
	} else {
		if command != versionCmd {
			if err := ensureImagePath(); err != nil {
				fmt.Fprintf(os.Stderr, "unable to verify image path: %s\n", err)
				os.Exit(1)
			}
			globals.ConfigureLogging(config.GetLogLevel())
			imgpull.SetConcurrentBlobs(int(config.GetPullTimeout()) * 1000)
		}
		switch command {
		case loadCmd:
			if err := preload.Load(config.GetImageFile()); err != nil {
				fmt.Printf("error loading images: %s\n", err)
			}
		case listCmd:
			if err := listCache(); err != nil {
				fmt.Printf("error listing the cache: %s\n", err)
			}
		case pruneCmd:
			fmt.Printf("in progress..\n")
		case versionCmd:
			fmt.Printf("ociregistry version: %s build date: %s\n", buildVer, buildDtm)
		case serveCmd:
			serve(buildVer, buildDtm)
		}
	}
}

// ensureImagePath ensures that the configured image path exists or returns an error
// if it cannot be created.
func ensureImagePath() error {
	if absPath, err := filepath.Abs(config.GetImagePath()); err == nil {
		return os.MkdirAll(absPath, 0755)
	} else {
		return err
	}
}
