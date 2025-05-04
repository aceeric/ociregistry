package subcmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aceeric/ociregistry/api"
	"github.com/aceeric/ociregistry/impl"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/preload"

	"github.com/labstack/echo/v4"
	middleware "github.com/oapi-codegen/echo-middleware"
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

// listener will be initialized with the Echo listener once the Echo server
// is started.
var listener net.Listener

// Serve runs the OCI distribution server, blocking until stopped with CTRL-C
// or via the command REST API.
func Serve(buildVer string, buildDtm string) error {
	if config.GetPreloadImages() != "" {
		if err := preload.Load(config.GetPreloadImages()); err != nil {
			return fmt.Errorf("error pre-loading images: %s", err)
		}
	}
	swagger, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("error loading swagger spec: %s", err)
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	shutdownCh := make(chan bool)
	stopPruneCh := make(chan bool)
	pruneStoppedCh := make(chan bool)
	ociRegistry := impl.NewOciRegistry(shutdownCh)

	// Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Use our validation middleware to check all requests against the OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	api.RegisterHandlers(e, ociRegistry)

	e.Use(globals.GetEchoLoggingFunc())

	if err := cache.RunPruner(stopPruneCh, pruneStoppedCh); err != nil {
		return fmt.Errorf("error starting the pruner: %s", err)
	}

	if err := cache.Load(config.GetImagePath()); err != nil {
		return fmt.Errorf("error loading the image cache: %s", err)
	}

	fmt.Fprintf(os.Stderr, startupBanner, buildVer, buildDtm, time.Unix(0, time.Now().UnixNano()), config.GetPort(),
		os.Getuid(), os.Getgid(), os.Getpid(), strings.Join(os.Args, " "))

	// start the API server
	go func() {
		if err := e.Start(net.JoinHostPort("0.0.0.0", strconv.Itoa(int(config.GetPort())))); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server. error:", err)
		}
	}()
	err = waitForEchoListener(e)
	if err != nil {
		return errors.New("timed out waiting for Echo listener")
	}
	listener = e.Listener
	log.Info("server is running")

	<-shutdownCh
	log.Infof("received stop command - stopping")
	e.Server.Shutdown(context.Background())
	if config.GetPruneConfig().Enabled {
		stopPruneCh <- true
		log.Infof("waiting for pruner to stop")
		<-pruneStoppedCh
		log.Infof("pruner stopped")
	}
	cache.WaitPulls()
	log.Infof("stopped")
	return nil
}

// waitForEchoListener waits for the Listener in the Echo server to be initialized. This
// is only used in unit testing so that the unit tests can start the server on ":0" and let
// the http package assign a random port number. Supports unit testing.
func waitForEchoListener(e *echo.Echo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if e.Listener != nil {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// GetListener supports unit testing.
func GetListener() net.Listener {
	return listener
}

// InitListener supports unit testing.
func InitListener() {
	listener = nil
}
