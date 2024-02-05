package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"ociregistry/api"
	"ociregistry/apiimpl"
	"ociregistry/importer"
	"ociregistry/pullsync"

	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"

	"github.com/labstack/echo/v4"
	middleware "github.com/oapi-codegen/echo-middleware"
)

func main() {
	// parse args
	var logLevel, imagePath, port, configPath string

	flag.StringVar(&logLevel, "log-level", string(log.ERROR), "Log level")
	flag.StringVar(&imagePath, "image-path", "", "Image path")
	flag.StringVar(&configPath, "config-path", "", "Image path")
	flag.StringVar(&port, "port", "8080", "Port for server")
	flag.Parse()

	if configPath == "" {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		configPath = filepath.Dir(ex)
	}
	swagger, err := api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec\n: %s", err)
		os.Exit(1)
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	// if --image-path arg not supplied then use ""../images" (expecting that
	// this binary is running in <project root>/bin)
	if imagePath == "" {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		imagePath = filepath.Join(filepath.Dir(ex), "..", "images")
	}

	// set the path where all image metadata and blobs are stored
	apiimpl.SetImagePath(imagePath)

	// create an instance of our handler which implements the generated interface
	ociRegistry := apiimpl.NewOciRegistry()

	// this is how you set up a basic Echo router
	e := echo.New()

	// API calls are logged somehow with a different logger than e.Logger ??
	apiLogging := echomiddleware.LoggerConfig{
		Skipper: echomiddleware.DefaultSkipper,
		Format: `${time_rfc3339} REST echo-server -- IP:${remote_ip} ` +
			`HOST:${host} ${method}:${uri} UA:${user_agent} ${status}` + "\n",
	}
	e.Use(echomiddleware.LoggerWithConfig(apiLogging))
	e.Logger.SetLevel(xlatLogLevel(logLevel))
	e.Logger.SetHeader("${time_rfc3339} ${level} ${short_file}:${line} --")

	pullsync.ConfigLoader(configPath, e.Logger)

	// use Open API middleware to check all requests against the OpenAPI schema
	e.Use(middleware.OapiRequestValidator(swagger))

	// register our OCI Registry above as the handler for the interface
	api.RegisterHandlers(e, ociRegistry)

	// set up the ability to handle image tarballs placed in the images dir
	go importer.Importer(imagePath, e.Logger)

	// start the server
	e.Logger.Fatal(e.Start(net.JoinHostPort("0.0.0.0", port)))
}

func xlatLogLevel(level string) log.Lvl {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return log.DEBUG
	case "INFO":
		return log.INFO
	case "WARN":
		return log.WARN
	case "OFF":
		return log.OFF
	case "ERROR":
		return log.ERROR
	}
	return log.ERROR
}
