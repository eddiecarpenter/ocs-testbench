// Command ocs-testbench is the entry point for the OCS Testbench
// application. It wires together the configuration, logging,
// persistence, and lifecycle layers, embeds the React/TypeScript
// frontend via go:embed, and exposes the application over HTTP with a
// graceful-shutdown contract.
//
// The startup sequence (per docs/ARCHITECTURE.md §18 and Feature #24
// scope):
//
//  1. ASCII banner + logging.Bootstrap() — operator-visible identity
//     plus an early-stage logger so config-load failures surface
//     cleanly.
//  2. Resolve the config path (CONFIG_FILE env → CLI arg →
//     ./config.yaml → cmd/ocs-testbench/config.yaml) and load it
//     via baseconfig.Load.
//  3. Reconfigure logging from cfg.Logging.
//  4. Open the production store (pgx pool) and register its close as
//     the first shutdown callback.
//  5. (TODO) Wire the Diameter stack — placeholder until Feature lands.
//  6. (TODO) Mount the REST API router — placeholder until Feature lands.
//  7. Build the chi HTTP router with the request-logging middleware
//     and mount the embedded frontend handler as the not-found
//     fallback (so API routes added in step 6 take precedence).
//  8. Start the metrics server via appl.Lifecycle.StartMetrics, placed
//     between the store-shutdown registration and the HTTP-shutdown
//     registration so the reverse-of-registration drain order yields
//     HTTP → metrics → store.
//  9. Start the HTTP server on cfg.Server.Addr.
//  10. Optionally auto-open the default browser (gated by
//      cfg.Frontend.AutoOpenBrowser && !cfg.Headless).
//  11. Block on lifecycle.Run, which installs the SIGTERM/SIGINT/SIGQUIT
//      handler and waits for any of them (or ctx cancellation).
//  12. Shutdown drains the registered callbacks in reverse order:
//      HTTP server → metrics server → store close.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddiecarpenter/ocs-testbench/internal/appl"
	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	"github.com/eddiecarpenter/ocs-testbench/web"
)

// defaultConfigPath is the in-tree fall-back when neither the CLI flag
// nor CONFIG_FILE is set. Containerised deployments override the path
// via CONFIG_FILE.
const defaultConfigPath = "cmd/ocs-testbench/config.yaml"

// openBrowser is the function variable that runWith uses to launch
// the operating system's default browser. Production wires it to
// appl.OpenBrowser; tests override it to assert that the auto-open
// gate (AutoOpenBrowser && !Headless) is honoured without spawning a
// real subprocess.
var openBrowser = appl.OpenBrowser

func main() {
	// Banner first — operators see what binary launched before any
	// log output. PrintBanner uses figlet rendering directly; no
	// logger required.
	appl.PrintBanner(os.Stdout, "ocs-testbench")

	logging.Bootstrap()

	configPath := flag.String("config", "", "path to config YAML (overrides CONFIG_FILE env)")
	flag.Parse()

	cfg, err := baseconfig.Load(resolveConfigPath(*configPath))
	if err != nil {
		logging.Error("load config", "err", err)
		os.Exit(1)
	}

	if err := logging.Configure(cfg.Logging); err != nil {
		logging.Error("configure logging", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logging.Error("connect database", "err", err)
		os.Exit(1)
	}
	s := store.NewStore(pool)

	if err := runWith(ctx, cfg, s, web.FS, nil); err != nil {
		logging.Error("application exited with errors", "err", err)
		os.Exit(1)
	}
}

// resolveConfigPath returns the config path to hand to baseconfig.Load.
// CLI flag value takes precedence over the in-tree default; baseconfig.Load
// itself overrides either with CONFIG_FILE when that env var is set, so
// the operational precedence is CONFIG_FILE > flag > default.
func resolveConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return defaultConfigPath
}

// runWith is the testable entry point. It wires the lifecycle around
// an already-opened store and an already-resolved frontend filesystem,
// blocks on the lifecycle's Run, and returns the aggregated shutdown
// error.
//
// Tests inject a fake store (via store.NewTestStore) and a fake
// frontend FS (via fstest.MapFS) so the bootstrap path can be
// exercised without a real database or a built frontend.
//
// addrCh, when non-nil, receives the bound HTTP address as soon as
// the listener binds. Tests use this to discover the port when
// cfg.Server.Addr ends in :0.
func runWith(ctx context.Context, cfg *baseconfig.Config, s store.Store, embedded fs.FS, addrCh chan<- string) error {
	if cfg == nil {
		return errors.New("ocs-testbench: cfg is nil")
	}
	if s == nil {
		return errors.New("ocs-testbench: store is nil")
	}

	lc := appl.New(cfg)

	// Resolve the embedded frontend FS to the dist sub-directory so
	// the FrontendHandler sees index.html at root. The embed pivot in
	// the web/ package roots at "dist/".
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		return fmt.Errorf("ocs-testbench: resolve embedded frontend: %w", err)
	}
	frontHandler, err := appl.FrontendHandler(dist)
	if err != nil {
		return fmt.Errorf("ocs-testbench: frontend handler: %w", err)
	}

	router := chi.NewRouter()
	router.Use(logging.RequestLogger)

	// TODO(feature: diameter): wire the Diameter stack here when
	// internal/diameter lands. Expected shape:
	//   stack, err := diameter.New(cfg.Peers, ...)
	//   lc.RegisterShutdown("diameter", stack.Close)

	// TODO(feature: api): mount the REST + SSE routes here when
	// internal/api lands. Expected shape:
	//   api.Mount(router, s, ...)

	// SPA fallback last — chi's NotFound handler runs after every
	// explicit route declared above (none yet, by design).
	router.NotFound(frontHandler.ServeHTTP)

	server := &http.Server{
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	ln, err := net.Listen("tcp", cfg.Server.Addr)
	if err != nil {
		return fmt.Errorf("ocs-testbench: listen %q: %w", cfg.Server.Addr, err)
	}
	if addrCh != nil {
		addrCh <- ln.Addr().String()
	}
	logging.Info("http server listening", "addr", ln.Addr().String())

	go func() {
		if serveErr := server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logging.Error("http server exited", "err", serveErr)
		}
	}()

	// Shutdown registration order is the REVERSE of desired drain
	// order. Drain order per Feature scope: HTTP → metrics → store.
	// Therefore registration order: store, metrics, HTTP.
	lc.RegisterShutdown("store", func(context.Context) error {
		s.Close()
		return nil
	})
	if err := lc.StartMetrics(); err != nil {
		// Best-effort drain of what's been registered so far before
		// returning the error.
		s.Close()
		return fmt.Errorf("ocs-testbench: start metrics: %w", err)
	}
	lc.RegisterShutdown("http-server", func(shutCtx context.Context) error {
		return server.Shutdown(shutCtx)
	})

	if cfg.Frontend.AutoOpenBrowser && !cfg.Headless {
		url := browserURL(ln.Addr().String())
		if err := openBrowser(url); err != nil {
			logging.Warn("auto-open browser failed; continuing", "url", url, "err", err)
		}
	}

	return lc.Run(ctx)
}

// browserURL formats an HTTP URL for the auto-open helper. The bound
// address may be of the form "[::]:8080" or "0.0.0.0:8080" — neither
// is meaningful for a local browser. Use localhost in those cases so
// the operator's browser opens to a reachable URL.
func browserURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

