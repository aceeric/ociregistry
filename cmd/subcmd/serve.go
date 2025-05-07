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
Tls: %s
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
	tlsCfg, err := globals.ParseTls()
	if err != nil {
		return fmt.Errorf("error parsing TLS configuration: %s", err)
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
		os.Getuid(), os.Getgid(), os.Getpid(), tlsMsg(), strings.Join(os.Args, " "))

	go health()

	// start the API server
	go func() {
		addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(int(config.GetPort())))
		if tlsCfg != nil {
			s := http.Server{
				Addr:      addr,
				Handler:   e,
				TLSConfig: tlsCfg,
			}
			if err := e.StartServer(&s); err != http.ErrServerClosed {
				e.Logger.Fatal("shutting down the server. error:", err)
			}
		} else {
			if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
				e.Logger.Fatal("shutting down the server. error:", err)
			}
		}
	}()
	err = waitForEchoListener(e)
	if err != nil {
		return errors.New("timed out waiting for Echo listener")
	}
	listener = getEchoListener(e)
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

// tlsMsg formats the server TLS configuration for the startup banner
func tlsMsg() string {
	msg := "none"
	tlsCfg := config.GetServerTlsCfg()
	if tlsCfg.Cert != "" && tlsCfg.Key != "" {
		msg = fmt.Sprintf("cert=%s, key=%s", tlsCfg.Cert, tlsCfg.Key)
	}
	if tlsCfg.CA != "" {
		msg = fmt.Sprintf("%s, ca=%s", msg, tlsCfg.CA)
	}
	if msg != "none" {
		return fmt.Sprintf("%s, client verify=%s", msg, tlsCfg.ClientAuth)
	}
	return "none"
}

// health handles the /health endpoint always on plain HTTP and is not part of the
// server itself, hence a separate goroutine running an http server.
func health() {
	if config.GetHealth() != 0 {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		http.ListenAndServe(fmt.Sprintf(":%d", config.GetHealth()), nil)
	}
}

// getEchoListener gets the Echo listener. Supports unit testing.
func getEchoListener(e *echo.Echo) net.Listener {
	if e.Listener != nil {
		return e.Listener
	}
	return e.TLSListener
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
			if e.Listener != nil || e.TLSListener != nil {
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
