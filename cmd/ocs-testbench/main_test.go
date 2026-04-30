// `runWith` is a bootstrap test that walks the full lifecycle on a
// fake store + fake embedded FS. Under `-tags dev`, runWith spawns
// a Vite subprocess via the dev frontend hook — that's intentional
// for the runtime path but breaks tests (no npm or web/ dir context
// to spawn from). Tag this file `!dev` so the dev-mode build skips
// it; the prod build (default `go test`) keeps full coverage.
//
//go:build !dev

package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// fakeFrontendFS mirrors the layout the production embed pivots on:
// "dist/index.html" plus a sample asset. Tests read it via fs.Sub on
// "dist", matching what runWith does in main.
func fakeFrontendFS() fstest.MapFS {
	return fstest.MapFS{
		"dist/index.html":      {Data: []byte("<!doctype html><body>spa-placeholder</body>")},
		"dist/assets/app.js":   {Data: []byte("console.log('app');")},
	}
}

func smokeConfig() *baseconfig.Config {
	return &baseconfig.Config{
		BaseConfig: baseconfig.BaseConfig{
			AppName:     "ocs-testbench-smoke",
			DatabaseURL: "irrelevant-for-smoke", // bypassed by injected store
			Logging:     baseconfig.LogConfig{Format: "text", Level: "warn"},
			Metrics:     baseconfig.MetricsConfig{Enabled: false},
		},
		Server: baseconfig.ServerConfig{
			Addr:         "127.0.0.1:0",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			IdleTimeout:  5 * time.Second,
		},
		Frontend: baseconfig.FrontendConfig{
			AutoOpenBrowser:    false, // no browser launch under test
			EmbeddedAssetsPath: "web/dist",
		},
		Headless: true,
		Peers:    []baseconfig.Peer{},
	}
}

// runSmoke launches runWith on a fresh goroutine, returns the bound
// HTTP address, and a function the test calls to trigger graceful
// shutdown and wait for runWith to return.
func runSmoke(t *testing.T) (addr string, shutdown func() error) {
	t.Helper()
	cfg := smokeConfig()
	s := store.NewTestStore()

	ctx, cancel := context.WithCancel(context.Background())
	addrCh := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- runWith(ctx, cfg, s, fakeFrontendFS(), addrCh)
	}()

	select {
	case addr = <-addrCh:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("HTTP server did not bind within 2s")
	}

	shutdown = func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(3 * time.Second):
			return errBootBlock
		}
	}
	t.Cleanup(func() { _ = shutdown() })
	return addr, shutdown
}

// errBootBlock is returned when the smoke harness times out waiting
// for runWith to return after cancellation. Surfaces a clean failure
// rather than hanging the test indefinitely.
var errBootBlock = errString("smoke harness: runWith did not exit within 3s of cancel")

type errString string

func (e errString) Error() string { return string(e) }

// TestSmoke_ServesEmbeddedFrontendRoot — the embedded SPA placeholder
// is served at /.
func TestSmoke_ServesEmbeddedFrontendRoot(t *testing.T) {
	addr, _ := runSmoke(t)

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "spa-placeholder") {
		t.Errorf("body should contain the SPA placeholder marker; got: %s", body)
	}
}

// TestSmoke_SPAFallback — an unknown client-side path falls through
// to index.html so React Router can resolve it.
func TestSmoke_SPAFallback(t *testing.T) {
	addr, _ := runSmoke(t)

	resp, err := http.Get("http://" + addr + "/some/spa/path")
	if err != nil {
		t.Fatalf("GET SPA path: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "spa-placeholder") {
		t.Errorf("SPA fallback should serve index.html; got: %s", body)
	}
}

// TestSmoke_ServesNamedAsset — a real asset under dist/ is served
// verbatim (i.e., not silently SPA-fallback'd).
func TestSmoke_ServesNamedAsset(t *testing.T) {
	addr, _ := runSmoke(t)

	resp, err := http.Get("http://" + addr + "/assets/app.js")
	if err != nil {
		t.Fatalf("GET asset: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "console.log('app');" {
		t.Errorf("asset body: got %q, want %q", body, "console.log('app');")
	}
}

// TestSmoke_GracefulShutdown — cancelling the parent context (the
// in-process equivalent of SIGTERM) causes runWith to drain its
// shutdowns and return cleanly. This exercises the full bootstrap +
// HTTP server + store-close path.
func TestSmoke_GracefulShutdown(t *testing.T) {
	cfg := smokeConfig()
	s := store.NewTestStore()

	ctx, cancel := context.WithCancel(context.Background())
	addrCh := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- runWith(ctx, cfg, s, fakeFrontendFS(), addrCh) }()

	select {
	case <-addrCh:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("HTTP server did not bind within 2s")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("shutdown returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWith did not exit within 3s of cancel")
	}
}

// TestRunWith_RejectsNilCfg — defensive check at the entry to runWith.
func TestRunWith_RejectsNilCfg(t *testing.T) {
	err := runWith(context.Background(), nil, store.NewTestStore(), fakeFrontendFS(), nil)
	if err == nil {
		t.Error("expected error for nil cfg")
	}
}

// TestRunWith_RejectsNilStore — defensive check at the entry to runWith.
func TestRunWith_RejectsNilStore(t *testing.T) {
	err := runWith(context.Background(), smokeConfig(), nil, fakeFrontendFS(), nil)
	if err == nil {
		t.Error("expected error for nil store")
	}
}

// TestRunWith_FrontendMissingIndex — a malformed embedded FS surfaces
// as a runWith error rather than booting half-broken.
func TestRunWith_FrontendMissingIndex(t *testing.T) {
	bad := fstest.MapFS{
		"dist/something-else.txt": {Data: []byte("nope")},
	}
	err := runWith(context.Background(), smokeConfig(), store.NewTestStore(), bad, nil)
	if err == nil {
		t.Error("expected error when embedded FS is missing index.html")
	}
}

// TestBrowserURL_Localhost — wildcard bind addresses are rewritten to
// localhost so the browser-open helper opens a reachable URL.
func TestBrowserURL_Localhost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"0.0.0.0:8080", "http://localhost:8080"},
		{"[::]:8080", "http://localhost:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := browserURL(tc.in); got != tc.want {
				t.Errorf("browserURL(%q): got %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSmoke_AutoOpenBrowser_GateClosed — covers AC-10. With
// Headless=true the browser-open helper must NOT be called regardless
// of AutoOpenBrowser, and with AutoOpenBrowser=false it must NOT be
// called regardless of Headless.
func TestSmoke_AutoOpenBrowser_GateClosed(t *testing.T) {
	cases := []struct {
		name     string
		autoOpen bool
		headless bool
	}{
		{"headless overrides autoOpen", true, true},
		{"autoOpen disabled", false, false},
		{"both off", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := smokeConfig()
			cfg.Frontend.AutoOpenBrowser = tc.autoOpen
			cfg.Headless = tc.headless

			var called int
			prev := openBrowser
			openBrowser = func(string) error { called++; return nil }
			t.Cleanup(func() { openBrowser = prev })

			ctx, cancel := context.WithCancel(context.Background())
			addrCh := make(chan string, 1)
			done := make(chan error, 1)
			go func() { done <- runWith(ctx, cfg, store.NewTestStore(), fakeFrontendFS(), addrCh) }()

			select {
			case <-addrCh:
			case <-time.After(2 * time.Second):
				cancel()
				<-done
				t.Fatal("HTTP server did not bind")
			}

			cancel()
			<-done

			if called != 0 {
				t.Errorf("openBrowser was called %d time(s); expected 0", called)
			}
		})
	}
}

// TestSmoke_AutoOpenBrowser_GateOpen — covers AC-9. With
// AutoOpenBrowser=true and Headless=false the browser-open helper IS
// invoked exactly once with a localhost URL.
func TestSmoke_AutoOpenBrowser_GateOpen(t *testing.T) {
	cfg := smokeConfig()
	cfg.Frontend.AutoOpenBrowser = true
	cfg.Headless = false

	var (
		called int
		gotURL string
	)
	prev := openBrowser
	openBrowser = func(url string) error {
		called++
		gotURL = url
		return nil
	}
	t.Cleanup(func() { openBrowser = prev })

	ctx, cancel := context.WithCancel(context.Background())
	addrCh := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- runWith(ctx, cfg, store.NewTestStore(), fakeFrontendFS(), addrCh) }()

	select {
	case <-addrCh:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("HTTP server did not bind")
	}

	cancel()
	<-done

	if called != 1 {
		t.Errorf("openBrowser called %d time(s); expected 1", called)
	}
	if gotURL == "" || !strings.HasPrefix(gotURL, "http://") {
		t.Errorf("openBrowser URL: got %q, expected http:// prefix", gotURL)
	}
}

// TestResolveConfigPath — flag wins over default; default applies
// when flag is empty. CONFIG_FILE override is verified inside
// baseconfig.Load itself.
func TestResolveConfigPath(t *testing.T) {
	if got := resolveConfigPath(""); got != defaultConfigPath {
		t.Errorf("resolveConfigPath(empty): got %q, want %q", got, defaultConfigPath)
	}
	if got := resolveConfigPath("/etc/myapp.yaml"); got != "/etc/myapp.yaml" {
		t.Errorf("resolveConfigPath(flag): got %q, want %q", got, "/etc/myapp.yaml")
	}
}
