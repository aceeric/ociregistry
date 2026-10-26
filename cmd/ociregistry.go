package main

import (
	"fmt"
	"os"

	"github.com/aceeric/ociregistry/cmd/subcmd"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/preload"
	"github.com/aceeric/ociregistry/impl/serialize"

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
	migrateCmd string = "migrate-cache"
	versionCmd string = "version"
	// emptyCmd means no command was invoked so the CLI parser will display
	// help and so there's nothing to do.
	emptyCmd string = ""
)

// main is the entry point
func main() {
	os.Exit(realMain())
}

// realMain allows deferred functions to run and also to return an exit code
// to the OS.
func realMain() int {
	command, err := getCfg()
	writeable := true
	if command == listCmd {
		writeable = false
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting configuration: %s\n", err)
		return 1
	} else if command == emptyCmd {
		return 0
	} else if command == versionCmd {
		fmt.Fprintf(os.Stderr, "ociregistry version: %s build date: %s\n", buildVer, buildDtm)
		return 0
	}

	// before anything touches the cache, including creating its directories
	switch command {
	case loadCmd, listCmd, pruneCmd:
		if needs, err := serialize.NeedsMigration(config.GetImagePath()); err != nil {
			fmt.Fprintf(os.Stderr, "unable to check whether the cache needs migration: %s\n", err)
			return 1
		} else if needs {
			fmt.Fprintln(os.Stderr, subcmd.NotMigratedMsg)
			return 1
		}
	}

	if config.GetHelloWorld() {
		if tmpDir, err := helloWorldMode(); err != nil {
			fmt.Fprintf(os.Stderr, "error configuring hello-world mode: %s\n", err)
			return 1
		} else {
			defer os.RemoveAll(tmpDir)
		}
	} else if command == migrateCmd && config.GetMigrateConfig().DryRun {
		// a dry run writes nothing, not even the directories
	} else if err := serialize.CreateDirs(config.GetImagePath(), writeable); err != nil {
		fmt.Fprintf(os.Stderr, "unable to verify image path: %s\n", err)
		return 1
	}

	globals.ConfigureLogging(config.GetLogLevel(), config.GetLogFile())
	imgpull.SetConcurrentBlobs(int(config.GetPullTimeout()) * 1000)

	switch command {
	case loadCmd:
		if err := preload.Load(config.GetImageFile(), config.GetResolveRef()); err != nil {
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
	case migrateCmd:
		if err := subcmd.MigrateCache(); err != nil {
			fmt.Fprintf(os.Stderr, "error migrating the cache: %s\n", err)
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
