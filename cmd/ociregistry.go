package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"ociregistry/api"
	"ociregistry/globals"
	"ociregistry/impl"
	"ociregistry/impl/memcache"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"

	"github.com/labstack/echo/v4"
	middleware "github.com/oapi-codegen/echo-middleware"
	log "github.com/sirupsen/logrus"
)

type cmdLine struct {
	logLevel   string
	imagePath  string
	port       string
	configPath string
}

const startupBanner = `----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Started: %s (port %s)
----------------------------------------------------------------------
`

func main() {
	args := parseCmdline()
	fmt.Fprintf(os.Stderr, startupBanner, time.Unix(0, time.Now().UnixNano()), args.port)

	swagger, err := api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec: %s\n", err)
		os.Exit(1)
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	ociRegistry := impl.NewOciRegistry(args.imagePath)

	// Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	api.RegisterHandlers(e, &ociRegistry)

	globals.SetLogLevel(args.logLevel)

	// have Echo use the global logging
	e.Use(globals.GetEchoLoggingFunc())

	upstream.ConfigLoader(args.configPath)

	// use Open API middleware to check all requests against the OpenAPI schema
	e.Use(middleware.OapiRequestValidator(swagger))

	// set up the ability to handle image tarballs placed in the images dir
	// NEED TO REWORK THIS... go importer.Importer(args.imagePath)

	// load cached image metadata into mem
	serialize.FromFilesystem(memcache.GetCache(), args.imagePath)

	// start the API server
	err = e.Start(net.JoinHostPort("0.0.0.0", args.port))
	if err != nil {
		log.Error(err.Error())
	}
}

// parseCmdline defines configuration defaults, parses the command line to
// potentially override defaults and returns the resulting program configuration.
func parseCmdline() cmdLine {
	args := cmdLine{}
	flag.StringVar(&args.logLevel, "log-level", "error", "Log level. Defaults to 'error'")
	flag.StringVar(&args.imagePath, "image-path", "/var/lib/ociregistry", "Path for the image store. Defaults to '/var/lib/ociregistry'")
	flag.StringVar(&args.configPath, "config-path", "", "Remote registry configuration file. Defaults to empty string (all remotes anonymous)")
	flag.StringVar(&args.port, "port", "8080", "Port for server. Defaults to 8080")
	flag.Parse()
	return args
}
