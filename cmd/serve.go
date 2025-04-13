package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"ociregistry/api"
	"ociregistry/impl"
	"ociregistry/impl/cache"
	"ociregistry/impl/config"
	"ociregistry/impl/globals"
	"ociregistry/impl/preload"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const startupBanner = `----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Version: %s, build date: %s
Started: %s (port %d)
Running as (uid:gid) %d:%d
Process id: %d
Command line: %v
----------------------------------------------------------------------
`

// serve runs the OCI distribution server, blocking until stopped with CTRL-C
// or via the command REST API.
func serve(buildVer string, buildDtm string) {
	if config.GetPreloadImages() != "" {
		if err := preload.Preload(config.GetPreloadImages()); err != nil {
			fmt.Printf("error pre-loading images: %s\n", err)
			os.Exit(0)
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

	fmt.Fprintf(os.Stderr, startupBanner, buildVer, buildDtm, time.Unix(0, time.Now().UnixNano()), config.GetPort(),
		os.Getuid(), os.Getgid(), os.Getpid(), strings.Join(os.Args, " "))

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
