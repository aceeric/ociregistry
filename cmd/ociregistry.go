package main

import (
	_ "embed"
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
	os.Exit(realMain())
}

// this allows deferred functions to run
func realMain() int {
	command, err := getCfg()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting configuration: %s\n", err)
		return 1
	}
	if command == versionCmd {
		fmt.Fprintf(os.Stderr, "ociregistry version: %s build date: %s\n", buildVer, buildDtm)
		return 0
	}
	if config.GetHelloWorld() {
		if tmpDir, err := helloWorldMode(); err != nil {
			fmt.Fprintf(os.Stderr, "error configuring hello-world mode: %s\n", err)
			return 1
		} else {
			defer os.RemoveAll(tmpDir)
		}
	} else if err := ensureImagePaths(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to verify image path: %s\n", err)
		return 1
	}

	globals.ConfigureLogging(config.GetLogLevel(), config.GetLogFile())
	imgpull.SetConcurrentBlobs(int(config.GetPullTimeout()) * 1000)

	switch command {
	case loadCmd:
		if err := preload.Load(config.GetImageFile()); err != nil {
			fmt.Fprintf(os.Stderr, "error loading images: %s\n", err)
			return 1
		}
	case listCmd:
		if err := subcmd.ListCache(); err != nil {
			fmt.Fprintf(os.Stderr, "error listing the cache: %s\n", err)
			return 1
		}
	case pruneCmd:
		if err := subcmd.Prune(); err != nil {
			fmt.Fprintf(os.Stderr, "error pruning the cache: %s\n", err)
			return 1
		}
	case serveCmd:
		if err := subcmd.Serve(buildVer, buildDtm); err != nil {
			fmt.Fprintf(os.Stderr, "error starting the server: %s\n", err)
			return 1
		}
	}
	return 0
}

// ensureImagePaths ensures that the configured image cache directories exist or
// returns an error.
func ensureImagePaths() error {
	// all these MkdirAll are nop if dirs exist
	for _, subDir := range []string{"fat", "img", "blobs"} {
		if absPath, err := filepath.Abs(filepath.Join(config.GetImagePath(), subDir)); err == nil {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return err
			}
		}
	}
	pt := filepath.Join(config.GetImagePath(), ".permtest")
	defer os.Remove(pt)
	if _, err := os.Create(pt); err != nil {
		return fmt.Errorf("directory %s is not writable", config.GetImagePath())
	}
	return nil
}
