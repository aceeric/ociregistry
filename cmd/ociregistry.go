package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ociregistry/cmd/subcmd"
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

// main is the entry point
func main() {
	command, err := getCfg()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting configuration: %s\n", err)
		os.Exit(1)
	}
	if command != versionCmd {
		if err := ensureImagePaths(); err != nil {
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
		if err := subcmd.ListCache(); err != nil {
			fmt.Printf("error listing the cache: %s\n", err)
		}
	case pruneCmd:
		if err := subcmd.Prune(); err != nil {
			fmt.Printf("error listing the cache: %s\n", err)
		}
	case versionCmd:
		fmt.Printf("ociregistry version: %s build date: %s\n", buildVer, buildDtm)
	case serveCmd:
		subcmd.Serve(buildVer, buildDtm)
	}
}

// ensureImagePaths ensures that the configured image cache directories exist or
// returns an error.
func ensureImagePaths() error {
	for _, subDir := range []string{"fat", "img", "blob"} {
		if absPath, err := filepath.Abs(filepath.Join(config.GetImagePath(), subDir)); err == nil {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}
