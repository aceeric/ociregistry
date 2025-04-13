package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	command, err := getCfg()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error establishing configuration: %s\n", err)
		os.Exit(1)
	}

	globals.ConfigureLogging(config.GetLogLevel())
	imgpull.SetConcurrentBlobs(int(config.GetPullTimeout()) * 1000)

	switch command {
	case "load":
		if err := preload.Preload(); err != nil {
			fmt.Printf("error loading images: %s\n", err)
		}
	case "list":
		if err := listCache(); err != nil {
			fmt.Printf("error listing the cache: %s\n", err)
		}
	case "prune":
		fmt.Printf("TODO\n")
	case "version":
		fmt.Printf("ociregistry version: %s build date: %s\n", buildVer, buildDtm)
	case "serve":
		serve()
	}
}

// serve runs the OCI distribution server, blocking until stopped with CTRL-C
// or via the command API.
func serve() {
	if config.GetPreloadImages() != "" {
		if err := preload.Preload(); err != nil {
			fmt.Printf("error pre-loading images: %s\n", err)
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, startupBanner, buildVer, buildDtm,
		time.Unix(0, time.Now().UnixNano()), config.GetPort(),
		os.Getuid(), os.Getgid(), os.Getpid(),
		strings.Join(os.Args, " "))

	if config.GetPreloadImages() != "" {
		err := preload.Preload()
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

	ociRegistry := impl.NewOciRegistry()

	// Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	api.RegisterHandlers(e, ociRegistry)

	// have Echo use the global logging
	e.Use(globals.GetEchoLoggingFunc())

	if err := cache.RunPruner(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting the pruner: %s\n", err)
		os.Exit(1)
	}

	// use Open API middleware to check all requests against the OpenAPI schema
	// for now, don't do this until I add the cmd api to the Swagger spec
	//e.Use(middleware.OapiRequestValidator(swagger))

	// load cached image metadata into mem
	cache.Load(config.GetImagePath())

	// set up the command API
	shutdownCh := make(chan bool)
	cmdApi(e, shutdownCh)

	// start the API server
	go func() {
		if err := e.Start(net.JoinHostPort("0.0.0.0", strconv.Itoa(int(config.GetPort())))); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()
	log.Info("server is running")
	<-shutdownCh
	log.Infof("received stop command - stopping")
	e.Server.Shutdown(context.Background())
	log.Infof("stopped")
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

// makeDirs ensures that the configured image path exists or returns an error
// if it cannot.
func makeDirs() error {
	if absPath, err := filepath.Abs(config.GetImagePath()); err == nil {
		return os.MkdirAll(absPath, 0755)
	}
	return nil
}
