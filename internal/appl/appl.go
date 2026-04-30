// Package appl is the application-lifecycle layer of the OCS Testbench.
//
// A Lifecycle owns three responsibilities:
//
//   - The Prometheus metrics server (chi router + promhttp) when
//     enabled in configuration.
//   - The OS signal handler (SIGTERM, SIGINT, SIGQUIT) that triggers
//     graceful shutdown.
//   - The ordered drain of registered shutdown callbacks (HTTP server,
//     Diameter stack, store, etc.) — fired in reverse-registration
//     order with a bounded per-callback timeout.
//
// On Run, the Lifecycle prints an ASCII banner identifying the
// binary, starts the metrics server, then blocks until either a
// trapped signal arrives, the caller cancels the parent context, or
// the metrics server fails fatally. Shutdown then drains the
// registered callbacks in reverse order, aggregating any errors into
// a single returned error.
package appl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	figure "github.com/common-nighthawk/go-figure"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// DefaultShutdownTimeout is the per-callback timeout enforced during
// graceful shutdown when no explicit value is set on the Lifecycle.
const DefaultShutdownTimeout = 15 * time.Second

// shutdownEntry is one registered shutdown callback.
type shutdownEntry struct {
	name string
	fn   func(context.Context) error
}

// Lifecycle orchestrates startup, runtime, and graceful shutdown of
// the binary. Construct via New, register shutdown callbacks before
// Run, then call Run.
//
// Lifecycle is single-shot: a second call to Run on the same instance
// is rejected.
type Lifecycle struct {
	cfg *baseconfig.Config

	// ShutdownTimeout is the bounded per-callback timeout enforced
	// during graceful shutdown. Callers MAY override before Run;
	// unchanged it defaults to DefaultShutdownTimeout.
	ShutdownTimeout time.Duration

	mu        sync.Mutex
	shutdowns []shutdownEntry
	running   bool

	// signals is the set the lifecycle traps. Production sets
	// SIGTERM/SIGINT/SIGQUIT via New; tests override via the
	// internal seam to avoid colliding with the test runner.
	signals []os.Signal

	// bannerOut is the writer for the ASCII banner. Defaults to
	// os.Stdout; tests redirect it.
	bannerOut io.Writer

	// listenFn binds the metrics server's listener. Production uses
	// net.Listen; tests inject a hook to bind on ":0" for ephemeral
	// ports.
	listenFn func(network, addr string) (net.Listener, error)

	metricsAddrMu sync.RWMutex
	metricsAddr   string
}

// New constructs a Lifecycle bound to cfg. cfg must be non-nil.
func New(cfg *baseconfig.Config) *Lifecycle {
	if cfg == nil {
		panic("appl.New: cfg is nil")
	}
	return &Lifecycle{
		cfg:             cfg,
		ShutdownTimeout: DefaultShutdownTimeout,
		signals:         []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT},
		bannerOut:       os.Stdout,
		listenFn:        net.Listen,
	}
}

// RegisterShutdown adds a named callback fired in reverse-registration
// order during shutdown. Call before Run. Calling after Run starts
// panics — late registration is almost always a bug.
func (l *Lifecycle) RegisterShutdown(name string, fn func(context.Context) error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		panic("appl.Lifecycle: RegisterShutdown after Run")
	}
	l.shutdowns = append(l.shutdowns, shutdownEntry{name: name, fn: fn})
}

// MetricsAddr returns the actual address the metrics server bound to.
// Empty when the metrics server is not running (either disabled in
// configuration or not yet started). Useful in tests that bind to
// ":0" and need to discover the ephemeral port.
func (l *Lifecycle) MetricsAddr() string {
	l.metricsAddrMu.RLock()
	defer l.metricsAddrMu.RUnlock()
	return l.metricsAddr
}

// Run prints the ASCII banner, starts the metrics server (when
// enabled), blocks on the first trapped signal or ctx cancellation,
// then drains the registered shutdown callbacks in reverse order
// with a bounded per-callback timeout. Returns the aggregated
// shutdown error (nil when every callback succeeded).
//
// Run is single-shot — a second invocation on the same Lifecycle
// returns an error without doing any work.
func (l *Lifecycle) Run(ctx context.Context) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return errors.New("appl.Lifecycle: Run already called")
	}
	l.running = true
	l.mu.Unlock()

	// Banner first — operators want to see what binary launched
	// before any other log output.
	fig := figure.NewFigure(l.cfg.AppName, "standard", true)
	figure.Write(l.bannerOut, fig)

	// Signal-aware context: any trapped signal cancels sigCtx.
	sigCtx, stopSig := signal.NotifyContext(ctx, l.signals...)
	defer stopSig()

	// Start the metrics server when enabled.
	var (
		metricsServer *http.Server
		metricsErrCh  chan error
	)
	if l.cfg.Metrics.Enabled {
		ms, errCh, err := l.startMetricsServer()
		if err != nil {
			return fmt.Errorf("appl: start metrics server: %w", err)
		}
		metricsServer = ms
		metricsErrCh = errCh
	}

	// Block until the signal-aware context is canceled or the metrics
	// server fails fatally. A nil channel blocks forever in select,
	// so the disabled-metrics case naturally falls through to the
	// signal branch.
	var runErr error
	select {
	case <-sigCtx.Done():
		// Graceful path — either a signal or the parent ctx was
		// canceled.
	case err := <-metricsErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("appl: metrics server: %w", err)
		}
	}

	// Stop the metrics server first; its shutdown is bounded by the
	// same per-callback timeout as user shutdowns.
	if metricsServer != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), l.shutdownTimeout())
		_ = metricsServer.Shutdown(stopCtx)
		cancel()
	}

	// Drain registered shutdowns in reverse-registration order.
	l.mu.Lock()
	entries := make([]shutdownEntry, len(l.shutdowns))
	copy(entries, l.shutdowns)
	l.mu.Unlock()

	var errs []error
	if runErr != nil {
		errs = append(errs, runErr)
	}

	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if err := l.runShutdown(e); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.name, err))
		}
	}

	return errors.Join(errs...)
}

// runShutdown invokes one shutdown callback under a fresh per-callback
// timeout, racing the callback's completion against the deadline. A
// callback that ignores the context and outlasts the timeout has its
// goroutine left to finish in the background — the process is in the
// middle of exiting, so leaking is acceptable.
func (l *Lifecycle) runShutdown(e shutdownEntry) error {
	ctx, cancel := context.WithTimeout(context.Background(), l.shutdownTimeout())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- e.fn(ctx) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out after %s: %w", l.shutdownTimeout(), ctx.Err())
	}
}

// shutdownTimeout returns the configured per-callback timeout,
// falling back to the default when unset.
func (l *Lifecycle) shutdownTimeout() time.Duration {
	if l.ShutdownTimeout > 0 {
		return l.ShutdownTimeout
	}
	return DefaultShutdownTimeout
}

// startMetricsServer binds a chi router exposing promhttp at the
// configured path on the configured address, runs the server in a
// goroutine, and returns the *http.Server and the goroutine's error
// channel. The actual bound address is captured on the Lifecycle so
// MetricsAddr can return it (useful for ephemeral-port tests).
func (l *Lifecycle) startMetricsServer() (*http.Server, chan error, error) {
	path := l.cfg.Metrics.Path
	if strings.TrimSpace(path) == "" {
		path = "/metrics"
	}

	r := chi.NewRouter()
	r.Method(http.MethodGet, path, promhttp.Handler())

	ln, err := l.listenFn("tcp", l.cfg.Metrics.Addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen %q: %w", l.cfg.Metrics.Addr, err)
	}

	l.metricsAddrMu.Lock()
	l.metricsAddr = ln.Addr().String()
	l.metricsAddrMu.Unlock()

	server := &http.Server{Handler: r}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(ln)
	}()
	return server, errCh, nil
}
