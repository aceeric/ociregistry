package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"ociregistry/api"
	"ociregistry/impl"
	"ociregistry/impl/globals"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type cmdLine struct {
	logLevel    string
	imagePath   string
	port        string
	configPath  string
	loadImages  string
	arch        string
	os          string
	pullTimeout int
	listCache   bool
	version     bool
	buildVer    string
	buildDtm    string
}

const startupBanner = `----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Started: %s (port %s)
----------------------------------------------------------------------
`

var (
	buildVer string
	buildDtm string
)

func main() {
	args := parseCmdline()
	globals.ConfigureLogging(args.logLevel)

	cliCommands(args)

	fmt.Fprintf(os.Stderr, startupBanner, time.Unix(0, time.Now().UnixNano()), args.port)

	swagger, err := api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading swagger spec: %s\n", err)
		os.Exit(1)
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	ociRegistry := impl.NewOciRegistry(args.imagePath, args.pullTimeout)

	// Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	api.RegisterHandlers(e, &ociRegistry)

	// have Echo use the global logging
	e.Use(globals.GetEchoLoggingFunc())

	upstream.ConfigLoader(args.configPath)

	// use Open API middleware to check all requests against the OpenAPI schema
	// for now, don't do this until I add the cmd api to the Swagger spec
	//e.Use(middleware.OapiRequestValidator(swagger))

	// load cached image metadata into mem
	serialize.FromFilesystem(args.imagePath)

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
	flag.StringVar(&args.loadImages, "load-images", "", "load images in the specified file into cache and then exit")
	flag.StringVar(&args.arch, "arch", "amd64", "architecture for the --load-images arg")
	flag.StringVar(&args.os, "os", "linux", "os for the --load-images arg")
	flag.IntVar(&args.pullTimeout, "pull-timeout", 60000, "max time in millis to pull an image from an upstream. Defaults to one minute")
	flag.BoolVar(&args.listCache, "list-cache", false, "Lists the cached images and exits")
	flag.BoolVar(&args.version, "version", false, "Displays the version and exits")
	flag.Parse()
	args.buildDtm = buildDtm
	args.buildVer = buildVer
	return args
}

// cmdApi implements the command API. Presently it consists only of:
// GET /cmd/stop
func cmdApi(e *echo.Echo, ch chan bool) {
	e.GET("/cmd/stop",
		func(c echo.Context) error {
			ch <- true
			return nil
		})
}
