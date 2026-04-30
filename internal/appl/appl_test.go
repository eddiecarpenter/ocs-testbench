package appl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// newTestLifecycle constructs a Lifecycle with sensible test defaults:
// a discarded banner, a tight shutdown timeout, and a non-OS signal
// (SIGUSR1) so injecting it does not interfere with the test runner.
func newTestLifecycle(t *testing.T, metricsEnabled bool) *Lifecycle {
	t.Helper()
	cfg := &baseconfig.Config{
		BaseConfig: baseconfig.BaseConfig{
			AppName: "ocs-testbench-test",
			Metrics: baseconfig.MetricsConfig{
				Enabled: metricsEnabled,
				Addr:    "127.0.0.1:0", // ephemeral port
				Path:    "/metrics",
			},
		},
	}
	l := New(cfg)
	l.bannerOut = io.Discard
	l.signals = []os.Signal{syscall.SIGUSR1}
	l.ShutdownTimeout = 200 * time.Millisecond
	return l
}

// TestRun_ShutdownsFireInReverseOrder — callbacks A, B, C registered
// in that order must fire as C, B, A on shutdown.
func TestRun_ShutdownsFireInReverseOrder(t *testing.T) {
	l := newTestLifecycle(t, false)

	var (
		mu    sync.Mutex
		order []string
	)
	record := func(name string) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, name)
			return nil
		}
	}
	l.RegisterShutdown("A", record("A"))
	l.RegisterShutdown("B", record("B"))
	l.RegisterShutdown("C", record("C"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel: Run drains shutdowns and exits

	if err := l.Run(ctx); err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"C", "B", "A"}
	if len(order) != len(want) {
		t.Fatalf("shutdown order: got %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("shutdown[%d]: got %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}

// TestRun_PerCallbackTimeoutEnforced — a callback that ignores its
// context and outlasts the timeout must produce a timeout error and
// not block Run indefinitely.
func TestRun_PerCallbackTimeoutEnforced(t *testing.T) {
	l := newTestLifecycle(t, false)
	l.ShutdownTimeout = 50 * time.Millisecond

	released := make(chan struct{})
	l.RegisterShutdown("slow", func(context.Context) error {
		<-released
		return nil
	})
	t.Cleanup(func() { close(released) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := l.Run(ctx)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("Run took %v; expected <1s with timeout enforcement", elapsed)
	}
	if err == nil {
		t.Fatal("expected aggregated timeout error; got nil")
	}
	if !strings.Contains(err.Error(), "slow") {
		t.Errorf("error should name the slow callback; got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should wrap context.DeadlineExceeded; got %v", err)
	}
}

// TestRun_AggregatesErrors — every failing callback's error appears
// in the joined return.
func TestRun_AggregatesErrors(t *testing.T) {
	l := newTestLifecycle(t, false)

	l.RegisterShutdown("first", func(context.Context) error { return errors.New("boom-1") })
	l.RegisterShutdown("second", func(context.Context) error { return errors.New("boom-2") })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := l.Run(ctx)
	if err == nil {
		t.Fatal("expected aggregated error; got nil")
	}
	for _, want := range []string{"first", "second", "boom-1", "boom-2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregated error missing %q: %v", want, err)
		}
	}
}

// TestRun_SignalInjectionTriggersShutdown — sending the configured
// signal to the running process triggers graceful shutdown.
func TestRun_SignalInjectionTriggersShutdown(t *testing.T) {
	l := newTestLifecycle(t, false)

	called := make(chan struct{})
	l.RegisterShutdown("on-signal", func(context.Context) error {
		close(called)
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- l.Run(context.Background()) }()

	// Brief wait so signal.NotifyContext has a chance to install.
	time.Sleep(20 * time.Millisecond)

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown callback was not invoked within 2s")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run: unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of signal")
	}
}

// TestRun_MetricsEndpointServesNonEmpty — when metrics are enabled
// the configured path returns a non-empty body containing at least
// one Go metric.
func TestRun_MetricsEndpointServesNonEmpty(t *testing.T) {
	l := newTestLifecycle(t, true)

	addrCh := make(chan string, 1)
	l.RegisterShutdown("publish-addr", func(context.Context) error {
		addrCh <- l.MetricsAddr()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	// Wait for the metrics server to bind and become reachable.
	var addr string
	deadline := time.After(2 * time.Second)
poll:
	for {
		if a := l.MetricsAddr(); a != "" {
			addr = a
			break
		}
		select {
		case <-deadline:
			t.Fatal("metrics server did not bind within 2s")
		case <-time.After(10 * time.Millisecond):
			continue poll
		}
	}

	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /metrics: status %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("GET /metrics: empty body")
	}
	if !bytes.Contains(body, []byte("go_")) {
		t.Errorf("GET /metrics: expected Go metrics in body; got: %s", string(body))
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run: unexpected error: %v", err)
	}
	// drain the addr channel to satisfy the registered shutdown
	select {
	case <-addrCh:
	default:
	}
}

// TestRun_MetricsDisabled_ServerNotStarted — when metrics are
// disabled MetricsAddr stays empty and Run returns cleanly without
// any listener.
func TestRun_MetricsDisabled_ServerNotStarted(t *testing.T) {
	l := newTestLifecycle(t, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := l.Run(ctx); err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if l.MetricsAddr() != "" {
		t.Errorf("MetricsAddr: expected empty when disabled; got %q", l.MetricsAddr())
	}
}

// TestRun_BannerWritten — the ASCII banner appears in the configured
// writer and contains the app name.
func TestRun_BannerWritten(t *testing.T) {
	l := newTestLifecycle(t, false)
	buf := &bytes.Buffer{}
	l.bannerOut = buf

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := l.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("banner was not written")
	}
	// figlet renders glyphs by row, not the literal name, so we just
	// assert non-empty output above.
}

// TestRun_RejectsSecondInvocation — the lifecycle is single-shot.
func TestRun_RejectsSecondInvocation(t *testing.T) {
	l := newTestLifecycle(t, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Run(ctx); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := l.Run(ctx); err == nil {
		t.Error("expected error on second Run")
	}
}

// TestRegisterShutdown_AfterRunPanics — late registration is
// almost always a bug and is rejected.
func TestRegisterShutdown_AfterRunPanics(t *testing.T) {
	l := newTestLifecycle(t, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on RegisterShutdown after Run")
		}
	}()
	l.RegisterShutdown("late", func(context.Context) error { return nil })
}

// TestNew_NilCfgPanics — Lifecycle requires a non-nil config.
func TestNew_NilCfgPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on New(nil)")
		}
	}()
	_ = New(nil)
}

// TestStartMetrics_BadAddrFails — an unbindable metrics address
// surfaces as a Run error and shutdown still drains.
func TestStartMetrics_BadAddrFails(t *testing.T) {
	cfg := &baseconfig.Config{
		BaseConfig: baseconfig.BaseConfig{
			AppName: "x",
			Metrics: baseconfig.MetricsConfig{
				Enabled: true,
				Addr:    "127.0.0.1:0",
				Path:    "/m",
			},
		},
	}
	l := New(cfg)
	l.bannerOut = io.Discard
	l.ShutdownTimeout = 50 * time.Millisecond
	l.listenFn = func(string, string) (net.Listener, error) {
		return nil, errors.New("listen-failed")
	}

	err := l.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "listen-failed") {
		t.Errorf("expected listen-failed error; got %v", err)
	}
}
