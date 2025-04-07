package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ociregistry/api"
	"ociregistry/impl"
	"ociregistry/impl/cache"
	"ociregistry/impl/config"
	"ociregistry/impl/globals"
	"ociregistry/impl/preload"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type cmdLine struct {
	logLevel         string
	imagePath        string
	port             string
	configPath       string
	loadImages       string
	preloadImages    string
	arch             string
	os               string
	pullTimeout      int
	listCache        bool
	prune            string
	pruneBefore      string
	dryRun           bool
	concurrent       int
	version          bool
	alwaysPullLatest bool
	buildVer         string
	buildDtm         string
	fix              string // DELETEME
}

const startupBanner = `----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Version: %s, build date: %s
Started: %s (port %s)
Running as (uid:gid) %d:%d
Process id: %d
Command line: %v
----------------------------------------------------------------------
`

// set by the compiler (see the Makefile):
var (
	buildVer string
	buildDtm string
)

func main() {
	args := parseCmdline()
	postprocessArgs(args)

	globals.ConfigureLogging(args.logLevel)
	imgpull.SetConcurrentBlobs(args.pullTimeout * 1000)

	cliCommands(args)

	fmt.Fprintf(os.Stderr, startupBanner, args.buildVer, args.buildDtm,
		time.Unix(0, time.Now().UnixNano()), args.port,
		os.Getuid(), os.Getgid(), os.Getpid(),
		strings.Join(os.Args, " "))

	if args.preloadImages != "" {
		err := preload.Preload(args.preloadImages, args.imagePath, args.arch, args.os, args.pullTimeout, args.concurrent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error preloading images: %s\n", err)
			os.Exit(1)
		}
	}

	swagger, err := api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading swagger spec: %s\n", err)
		os.Exit(1)
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	ociRegistry := impl.NewOciRegistry(args.imagePath, args.pullTimeout, args.alwaysPullLatest)

	// Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	api.RegisterHandlers(e, ociRegistry)

	// have Echo use the global logging
	e.Use(globals.GetEchoLoggingFunc())

	go config.ConfigLoader(args.configPath, 30)

	// use Open API middleware to check all requests against the OpenAPI schema
	// for now, don't do this until I add the cmd api to the Swagger spec
	//e.Use(middleware.OapiRequestValidator(swagger))

	// load cached image metadata into mem
	cache.Load(args.imagePath)

	// set up the command API
	shutdownCh := make(chan bool)
	cmdApi(e, shutdownCh)

	// start the API server
	go func() {
		if err := e.Start(net.JoinHostPort("0.0.0.0", args.port)); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()
	log.Info("server is running")
	<-shutdownCh
	log.Infof("received stop command - stopping")
	e.Server.Shutdown(context.Background())
	log.Infof("stopped")
}

// parseCmdline defines configuration defaults, parses the command line to
// potentially override defaults and returns the resulting program configuration.
func parseCmdline() cmdLine {
	args := cmdLine{}
	flag.StringVar(&args.logLevel, "log-level", "error", "Log level. Defaults to 'error'")
	flag.StringVar(&args.imagePath, "image-path", "/var/lib/ociregistry", "Path for the image store. Defaults to '/var/lib/ociregistry'")
	flag.StringVar(&args.configPath, "config-path", "", "Remote registry configuration file. Defaults to empty string (all remotes anonymous)")
	flag.StringVar(&args.port, "port", "8080", "Port for server. Defaults to 8080")
	flag.StringVar(&args.loadImages, "load-images", "", "Loads images enumerated in the specified file into cache and then exits")
	flag.StringVar(&args.preloadImages, "preload-images", "", "Loads images enumerated in the specified file into cache at startup and then continues to serve")
	flag.StringVar(&args.arch, "arch", "amd64", "Architecture for the --load-images and --preload-images args")
	flag.StringVar(&args.os, "os", "linux", "Operating system for the --load-images and --preload-images args")
	flag.IntVar(&args.concurrent, "concurrent", 1, "Specify --concurrent=n for --load-images and --preload-images args to use multiple goroutines")
	flag.IntVar(&args.pullTimeout, "pull-timeout", 60000, "Max time in millis to pull an image from an upstream. Defaults to one minute")
	flag.BoolVar(&args.listCache, "list-cache", false, "Lists the cached images and exits")
	flag.StringVar(&args.prune, "prune", "", "Prunes from the cache matching comma-separated pattern(s)")
	flag.StringVar(&args.pruneBefore, "prune-before", "", "Prunes from the cache created earlier than the specified datetime")
	flag.BoolVar(&args.dryRun, "dry-run", false, "Runs other commands in dry-run mode")
	flag.BoolVar(&args.version, "version", false, "Displays the version and exits")
	flag.BoolVar(&args.alwaysPullLatest, "always-pull-latest", false, "Never cache images pulled with the 'latest' tag")
	flag.StringVar(&args.fix, "fix", "", "Soon to be deleted...") // DELETEME
	flag.Parse()
	args.buildDtm = buildDtm
	args.buildVer = buildVer
	return args
}

// cmdApi implements the command API. Presently it consists of:
//
//	GET /cmd/stop to shutdown the server
//	GET /health (intended for k8s)
func cmdApi(e *echo.Echo, ch chan bool) {
	e.GET("/cmd/stop",
		func(ctx echo.Context) error {
			ch <- true
			return nil
		})
	e.GET("/health",
		func(ctx echo.Context) error {
			return ctx.NoContent(http.StatusOK)
		})
}

// postProcessArgs does some modification to the args. If anything fails, the
// program is terminated
func postprocessArgs(args cmdLine) {
	absPath, err := makeDirs(args.imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing image directory: %s, error: %s\n", args.imagePath, err)
		os.Exit(1)
	} else {
		args.imagePath = absPath
	}
}

// makeDirs creates all directories up to and including the passed directory.
// The passed directory can be relative or absolute.
func makeDirs(path string) (string, error) {
	if absPath, err := filepath.Abs(path); err == nil {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return "", err
		}
		return absPath, nil
	} else {
		return "", err
	}
}
