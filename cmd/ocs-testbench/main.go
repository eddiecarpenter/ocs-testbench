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
//  5. Load AVP dictionaries (built-in + active custom) via
//     internal/diameter/dictionary.Loader, then construct and start
//     the multi-peer Diameter Manager — peers come from the store
//     via StorePeerProvider; AutoConnect=true peers initiate their
//     lifecycle inside Start. Build the engine-facing Sender as a
//     protocol.Behaviour-wrapped messaging.Sender so the
//     protocol-mandated CCA behaviours (FUI-TERMINATE,
//     Validity-Time, 5xxx) fire automatically.
//  6. (TODO) Mount the REST API router — placeholder until Feature lands.
//  7. Build the chi HTTP router with the request-logging middleware
//     and mount the embedded frontend handler as the not-found
//     fallback (so API routes added in step 6 take precedence).
//  8. Start the metrics server via appl.Lifecycle.StartMetrics, placed
//     between the diameter-protocol shutdown registration and the
//     HTTP-shutdown registration so the reverse-of-registration drain
//     order yields
//     HTTP → metrics → diameter-protocol → diameter-manager → store.
//  9. Start the HTTP server on cfg.Server.Addr.
//  10. Optionally auto-open the default browser (gated by
//     cfg.Frontend.AutoOpenBrowser && !cfg.Headless).
//  11. Block on lifecycle.Run, which installs the SIGTERM/SIGINT/SIGQUIT
//     handler and waits for any of them (or ctx cancellation).
//  12. Shutdown drains the registered callbacks in reverse order:
//     HTTP server → metrics server → diameter-protocol →
//     diameter-manager → store close.
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
	"path/filepath"

	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddiecarpenter/ocs-testbench/db"
	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/appl"
	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/dictionary"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/manager"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/protocol"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	tmpl "github.com/eddiecarpenter/ocs-testbench/internal/template"
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
	if err := store.RunMigrations(cfg.DatabaseURL, db.Migrations); err != nil {
		logging.Error("run migrations", "err", err)
		os.Exit(1)
	}
	s := store.NewStore(pool)

	if err := runWith(ctx, cfg, s, web.FS, nil); err != nil {
		logging.Error("application exited with errors", "err", err)
		os.Exit(1)
	}
}

// resolveConfigPath returns the config path to hand to baseconfig.Load.
//
// Precedence (highest → lowest):
//  1. CONFIG_FILE env var (handled inside baseconfig.Load itself)
//  2. CLI -config flag
//  3. File next to the binary — covers macOS .app bundles where the CWD is
//     not the repository root (e.g. Contents/MacOS/config.yaml).
//  4. In-tree default "cmd/ocs-testbench/config.yaml" — works when running
//     from the repository root during development.
func resolveConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	// Check for a config.yaml next to the running binary. os.Executable
	// returns the real path of the executable even when called from inside
	// a macOS .app bundle (Contents/MacOS/<binary>).
	if exe, err := os.Executable(); err == nil {
		adjacent := filepath.Join(filepath.Dir(exe), "config.yaml")
		if _, err := os.Stat(adjacent); err == nil {
			return adjacent
		}
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

	// Dev mode (only compiled with `-tags dev`) returns a reverse-proxy
	// handler aimed at a Vite dev server, and registers Vite's lifecycle
	// on `lc`. In prod builds devFrontendHandler returns (nil, nil) and
	// the embedded FS path runs.
	frontHandler, err := devFrontendHandler(lc, dist)
	if err != nil {
		return fmt.Errorf("ocs-testbench: dev frontend: %w", err)
	}
	if frontHandler == nil {
		frontHandler, err = appl.FrontendHandler(dist)
		if err != nil {
			return fmt.Errorf("ocs-testbench: frontend handler: %w", err)
		}
	}

	// Load AVP dictionaries (built-in + active custom) before any
	// peer connection is opened. Per Feature #17 design plan and
	// AC-7/AC-8/AC-9: built-ins are loaded by go-diameter's package
	// init(); the loader extends dict.Default with active custom
	// dictionaries from the store and fails-soft on per-record XML
	// errors.
	loader := dictionary.NewLoader(dictionary.NewStoreSource(s))
	if _, err := loader.Load(ctx); err != nil {
		// A loader fatal error (built-ins missing, store list
		// failed) is non-recoverable — fail startup. Per-record
		// XML errors are logged-and-skipped inside Load and never
		// surface here.
		s.Close()
		return fmt.Errorf("ocs-testbench: load diameter dictionaries: %w", err)
	}

	// Construct and start the Diameter Manager. Peers come from
	// the store via StorePeerProvider; auto-connect peers initiate
	// their lifecycle inside Start. Manager.Stop is registered as
	// a shutdown callback so the reverse-drain order is
	// HTTP → manager → metrics → store.
	dmgr := manager.New(dict.Default, manager.NewStorePeerProvider(s))
	if err := dmgr.Start(ctx); err != nil {
		s.Close()
		return fmt.Errorf("ocs-testbench: start diameter manager: %w", err)
	}

	// Build the engine-facing Sender: a messaging.Sender wrapped
	// by protocol.Behaviour. The engine layer (when it lands)
	// receives the wrapped sender; protocol-mandated CCA actions
	// (FUI-TERMINATE → CCR-T, Validity-Time → CCR-U, 5xxx →
	// terminate) happen behind the scenes.
	//
	// The CCRTerminate / CCRUpdate builders are not yet wired —
	// they require engine-side per-session state (Service-Context-
	// Id, Subscription-Id, MSCC) which lands with the engine
	// itself. The Behaviour still tracks session state and marks
	// terminated sessions correctly; the auto out-of-band CCRs
	// are no-ops until the builders are supplied.
	sender := messaging.NewSender(dmgr)
	behaviour := protocol.New(sender, protocol.Options{})

	dictAdapter := tmpl.NewDictAdapter(dict.Default)
	execEngine := api.NewSessionManager(s, dmgr, behaviour, dictAdapter)

	// Build the MCP server handler and mount it at /mcp alongside the
	// REST API. The MCP server is a thin adapter over the same
	// store/PeerManager/ExecutionEngine interfaces used by the REST API.
	// dict.Default is the loaded Diameter AVP dictionary; it is passed so
	// the list_avps tool can enumerate all known AVPs.
	mcpHandler := internalmcp.NewServer(s, dmgr, execEngine, dictAdapter, dict.Default, cfg)

	// Bind the listener before constructing the AI agent so the agent
	// knows the real port when cfg.Server.Addr ends in :0.
	ln, err := net.Listen("tcp", cfg.Server.Addr)
	if err != nil {
		return fmt.Errorf("ocs-testbench: listen %q: %w", cfg.Server.Addr, err)
	}
	if addrCh != nil {
		addrCh <- ln.Addr().String()
	}

	// Construct the AI agent. NewAgent returns nil when the LLM endpoint is
	// not configured — the Router's nil guard returns 503 for AI endpoints.
	// The agent calls MCP tools via the HTTP server's /mcp path, so the
	// mcpBaseURL must resolve to the bound address.
	mcpBaseURL := "http://" + ln.Addr().String()
	aiAgent := ai.NewAgent(cfg.AI, mcpBaseURL)
	aiSessions := ai.NewSessionManager()

	apiRouter := api.Router(s, dmgr, execEngine, dictAdapter, dict.Default, aiAgent, aiSessions, Version)

	// Mount the API router at /api. All routes within api.Router are
	// relative to the router's root; the Mount prefix adds /api.
	router := chi.NewRouter()
	router.Mount("/api", apiRouter)
	router.Mount("/mcp", mcpHandler)

	// SPA fallback: any request not handled by the API routes above
	// is served by the frontend handler (React SPA).
	router.NotFound(frontHandler.ServeHTTP)

	server := &http.Server{
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}
	logging.Info("http server listening", "addr", ln.Addr().String())

	go func() {
		if serveErr := server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logging.Error("http server exited", "err", serveErr)
		}
	}()

	// Shutdown registration order is the REVERSE of desired drain
	// order. Drain order:
	//   HTTP → metrics → diameter-protocol → diameter-manager → store.
	// Therefore registration order: store, diameter-manager,
	// diameter-protocol, metrics, HTTP. The protocol Behaviour
	// stops first (cancels outstanding re-auth timers) so the
	// manager can then drain transports cleanly before the store
	// is closed.
	lc.RegisterShutdown("store", func(context.Context) error {
		s.Close()
		return nil
	})
	lc.RegisterShutdown("diameter-manager", func(context.Context) error {
		dmgr.Stop()
		return nil
	})
	lc.RegisterShutdown("diameter-protocol", func(context.Context) error {
		behaviour.Stop()
		return nil
	})
	if err := lc.StartMetrics(); err != nil {
		// Best-effort drain of what's been registered so far before
		// returning the error.
		behaviour.Stop()
		dmgr.Stop()
		s.Close()
		return fmt.Errorf("ocs-testbench: start metrics: %w", err)
	}
	lc.RegisterShutdown("http-server", func(shutCtx context.Context) error {
		return server.Shutdown(shutCtx)
	})
	// AI session manager — stop background TTL cleanup goroutine after the
	// HTTP server drains (registered after http-server so it drains first).
	lc.RegisterShutdown("ai-sessions", func(context.Context) error {
		aiSessions.Stop()
		return nil
	})

	if !cfg.Headless {
		url := browserURL(ln.Addr().String())

		if cfg.Frontend.AutoOpenBrowser {
			// Browser mode: open the operator's default browser and block on the
			// lifecycle signal handler. AutoOpenBrowser=true opts out of the
			// native Wails window so the full browser tooling (DevTools,
			// extensions, multiple tabs) is available.
			if err := openBrowser(url); err != nil {
				logging.Warn("auto-open browser failed; continuing", "url", url, "err", err)
			}
		} else {
			// Native window mode: launch the Wails window on the main OS thread.
			// lc.Run (signal handling + shutdown) moves to a goroutine so the
			// main thread is free for the Cocoa event loop.
			//
			// A derived cancellable context is used for lc.Run so that when
			// wails.Run returns (window closed, Cmd+Q, runtime.Quit) we can
			// unblock lc.Run immediately — otherwise the signal handler goroutine
			// waits forever and the process hangs requiring a force-kill.
			lcCtx, lcCancel := context.WithCancel(ctx)
			lcErrCh := make(chan error, 1)
			go func() { lcErrCh <- lc.Run(lcCtx) }()

			app := newWailsApp()
			wailsErr := wails.Run(&options.App{
				Title:                    "OCS Testbench",
				Width:                    1400,
				Height:                   900,
				MinWidth:                 900,
				MinHeight:                600,
				Menu:                     app.buildMenu(),
				OnStartup:                app.startup,
				EnableDefaultContextMenu: true,
				// Route all webview requests through our existing chi router so
				// the SPA and all API endpoints (including SSE) share one handler.
				// The HTTP server continues to run on cfg.Server.Addr for external
				// REST clients — both paths coexist independently.
				AssetServer: &assetserver.Options{
					Handler: router,
				},
				Mac: &mac.Options{
					TitleBar: mac.TitleBarDefault(),
					About: &mac.AboutInfo{
						Title:   "OCS Testbench",
						Message: "Diameter Gy Credit-Control testing tool",
					},
				},
			})

			// Window closed — cancel the lifecycle context so lc.Run unblocks,
			// then drain the channel before returning.
			lcCancel()
			if lcErr := <-lcErrCh; lcErr != nil && !errors.Is(lcErr, context.Canceled) {
				if wailsErr == nil {
					return lcErr
				}
			}
			return wailsErr
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
