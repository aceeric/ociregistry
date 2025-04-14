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

	globals.ConfigureLogging(config.GetLogLevel())
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
	for _, subDir := range []string{"fat", "img", "blobs"} {
		if absPath, err := filepath.Abs(filepath.Join(config.GetImagePath(), subDir)); err == nil {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

//go:embed hello_world/config.json
var configJson []byte

//go:embed hello_world/blob.tar.gz
var blobTarGz []byte

//go:embed hello_world/imageManifest.json
var imageManifest []byte

//go:embed hello_world/manifestList.json
var manifestList []byte

// helloWorldMode builds an image cache in the system temp directory from embedded files. The
// resulting files look exactly as if you pulled and cached docker.io/library/hello-world:latest
// then copied the image cache into the hello_world directory. It sets the server into air-gapped
// mode and serves only that one image. Its for testing.
func helloWorldMode() (string, error) {
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	config.SetAirGapped(true)
	config.SetPreloadImages("")
	config.SetImagePath(tmpdir)
	if err := ensureImagePaths(); err != nil {
		return "", err
	}
	filelist := []struct {
		Name  string
		Dir   string
		Bytes *[]byte
	}{
		{Name: "424f1f86cdf501deb591ace8d14d2f40272617b51b374915a87a2886b2025ece", Dir: "fat", Bytes: &manifestList},
		{Name: "03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c", Dir: "img", Bytes: &imageManifest},
		{Name: "74cc54e27dc41bb10dc4b2226072d469509f2f22f1a3ce74f4a59661a1d44602", Dir: "blobs", Bytes: &configJson},
		{Name: "e6590344b1a5dc518829d6ea1524fc12f8bcd14ee9a02aa6ad8360cce3a9a9e9", Dir: "blobs", Bytes: &blobTarGz},
	}
	for _, file := range filelist {
		if err := os.WriteFile(filepath.Join(tmpdir, file.Dir, file.Name), *file.Bytes, 0600); err != nil {
			return "", err
		}
	}
	return tmpdir, nil
}
