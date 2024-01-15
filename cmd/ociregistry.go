// runs the registry server
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"ociregistry/api"

	"github.com/labstack/gommon/log"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	middleware "github.com/oapi-codegen/echo-middleware"
)

func main() {
	// parse args
	var level, port string
	flag.StringVar(&level, "log-level", string(log.ERROR), "Log level")
	flag.StringVar(&port, "port", "8080", "Port for test HTTP server")
	flag.Parse()

	swagger, err := api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec\n: %s", err)
		os.Exit(1)
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	// set the path where all image metadata and blobs are stored
	api.SetImagePath(filepath.Join(filepath.Dir(ex), "..", "images"))

	// create an instance of our handler which satisfies the generated interface
	ociRegistry := api.NewOciRegistry()

	// this is how you set up a basic Echo router
	e := echo.New()

	// log all requests
	e.Use(echomiddleware.Logger())
	e.Logger.SetLevel(xlatLogLevel(level))

	// use our validation middleware to check all requests against the OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	// register our OCI Registry above as the handler for the interface
	api.RegisterHandlers(e, ociRegistry)

	// serve HTTP until the world ends
	e.Logger.Fatal(e.Start(net.JoinHostPort("0.0.0.0", port)))

	// TODO TLS
	//e.Logger.Fatal(e.StartTLS(net.JoinHostPort("0.0.0.0", *port), "ca.pem", "ca-key.pem"))
}

func xlatLogLevel(level string) log.Lvl {
	switch level {
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
