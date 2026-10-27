package subcmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aceeric/ociregistry/api"
	"github.com/aceeric/ociregistry/impl"
	"github.com/aceeric/ociregistry/impl/auth"
	"github.com/aceeric/ociregistry/impl/cache"
	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/metrics"
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

// listener is the server's listener, set when the server starts. It's read by unit tests from
// another goroutine (to find the port when serving on port 0), hence the mutex.
var (
	listenerMu sync.Mutex
	listener   net.Listener
)

// Serve runs the OCI distribution server, blocking until stopped with CTRL-C
// or via the command REST API.
func Serve(buildVer string, buildDtm string) error {
	// before anything reads or writes the cache: preload, the pruner, and the cache load
	if err := migrateOnStartup(config.GetImagePath()); err != nil {
		return err
	}
	if config.GetPreloadImages() != "" {
		if err := preload.Load(config.GetPreloadImages(), config.GetResolveRef()); err != nil {
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
	for ta := range config.UpstreamAuthProviders {
		log.Infof("initializing auth provider %q", ta.Provider)

		if isInitted, err := auth.IsInitialized(ta.Provider); err != nil {
			return fmt.Errorf("error validating auth provider: %s", err)
		} else if !isInitted {
			if auth.Init(ta.Provider, ta.ProviderOpts, ta.Expiry) != nil {
				return fmt.Errorf("error initializing auth provider: %s", err)
			}
		}
	}

	// clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

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

	metrics.InitMetrics(config.GetMetrics())

	if err := cache.Load(config.GetImagePath()); err != nil {
		return fmt.Errorf("error loading the image cache: %s", err)
	}

	fmt.Fprintf(os.Stderr, startupBanner, buildVer, buildDtm, time.Unix(0, time.Now().UnixNano()), config.GetPort(),
		os.Getuid(), os.Getgid(), os.Getpid(), tlsMsg(), strings.Join(os.Args, " "))

	// Create the listener here rather than letting Echo create it in its goroutine: Echo uses a
	// listener that is already set, so nothing has to wait or poll for it, nothing reads it
	// while Echo writes it, and a port that can't be bound is an ordinary startup error.
	addr := net.JoinHostPort(config.GetHost(), strconv.Itoa(int(config.GetPort())))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("unable to listen on %s: %w", addr, err)
	}
	if tlsCfg != nil {
		e.TLSListener = tls.NewListener(ln, tlsCfg)
		setListener(e.TLSListener)
	} else {
		e.Listener = ln
		setListener(ln)
	}

	// start the API server
	go func() {
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

	go health(config.GetHealth())

	log.Info("server is running")

	select {
	case <-stopCh:
		log.Infof("received stop command - stopping")
		break
	case <-shutdownCh:
		log.Infof("received shutdown signal - stopping")
		break
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Server.Shutdown(ctx); err != nil {
		log.Errorf("echo server shutdown encountered an error: %s", err)
	}

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
func health(port int) {
	if port != 0 {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}
}

// setListener records the server's listener.
func setListener(l net.Listener) {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	listener = l
}

// GetListener supports unit testing.
func GetListener() net.Listener {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	return listener
}

// InitListener supports unit testing.
func InitListener() {
	setListener(nil)
}
