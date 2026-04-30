//go:build dev

// Dev-only frontend handler — reverse-proxies non-API requests to a
// running Vite dev server so HMR works through the Go process's port.
// Compiled in only with `-tags dev`; production builds never link this
// file (the prod path stays in frontend.go, serving the embedded FS).
//
// Layout:
//   - DevFrontendHandler — the http.Handler that proxies to Vite. Plug
//     into chi's NotFound the same way prod plugs in FrontendHandler.
//   - ViteSupervisor    — optional subprocess manager. Tries to detect
//     an already-running Vite first; spawns `npm run dev` if none is
//     reachable. Stop() kills the spawned process group on shutdown.
//
// The supervisor and the handler are independent — the handler only
// needs the address of a reachable Vite. Callers that prefer to manage
// Vite externally (two-terminal flow) can skip the supervisor and use
// just the handler.

package appl

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DevFrontendHandler reverse-proxies all incoming requests to a Vite
// dev server at viteAddr. WebSocket upgrade for HMR is transparent —
// httputil.ReverseProxy honours `Connection: Upgrade` since Go 1.12.
//
// `viteAddr` should be an absolute URL, e.g. `http://localhost:5173`.
// The Go router's NotFound mount means API routes registered earlier
// take precedence; everything else falls through to this handler and
// gets proxied to Vite, which serves the SPA + HMR.
func DevFrontendHandler(viteAddr string) (http.Handler, error) {
	target, err := url.Parse(viteAddr)
	if err != nil {
		return nil, fmt.Errorf("appl: parse vite addr %q: %w", viteAddr, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Quieter error reporting — Vite occasionally drops the HMR socket
	// during restarts; the default `log` output is noisy and panics in
	// tests. Surface as a structured warning instead.
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		slog.Warn("dev frontend proxy error", "err", err)
		w.WriteHeader(http.StatusBadGateway)
	}
	return proxy, nil
}

// ViteSupervisor manages a child Vite dev server.
//
// Behaviour:
//   - On Start: probes viteAddr; if Vite is already running, attaches
//     to it without spawning a child. This lets the developer keep
//     `npm run dev` open in another terminal and have the Go process
//     reuse it.
//   - If no Vite responds: spawns `npm run dev -- --port <port>` from
//     workDir, with stdout/stderr piped through a `[vite]` prefix so
//     the operator can tell which logs come from where.
//   - Polls the address until reachable (timeout default 30s).
//   - Stop forwards SIGTERM to the entire process group so npm's
//     descendants (Vite, esbuild, postcss watchers) die too.
type ViteSupervisor struct {
	addr    string
	workDir string

	mu      sync.Mutex
	cmd     *exec.Cmd
	spawned bool
}

// NewViteSupervisor returns a supervisor; nothing is spawned yet.
// Call Start to attach or spawn.
func NewViteSupervisor(workDir, viteAddr string) *ViteSupervisor {
	return &ViteSupervisor{addr: viteAddr, workDir: workDir}
}

// Start attaches to a running Vite or spawns a new one. Returns nil
// when Vite is reachable on viteAddr; returns an error if spawn fails
// or the address never becomes reachable within the timeout.
func (s *ViteSupervisor) Start(timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if reachable(s.addr, 250*time.Millisecond) {
		slog.Info("dev: vite already running, reusing", "addr", s.addr)
		return nil
	}

	port := portFromAddr(s.addr)
	slog.Info("dev: starting vite subprocess", "dir", s.workDir, "port", port)

	cmd := exec.Command("npm", "run", "dev", "--", "--port", port, "--strictPort")
	cmd.Dir = s.workDir
	cmd.Stdout = newPrefixedWriter("vite", os.Stdout)
	cmd.Stderr = newPrefixedWriter("vite", os.Stderr)
	// New process group — Stop can SIGTERM the whole tree.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("dev: start vite: %w", err)
	}
	s.cmd = cmd
	s.spawned = true

	if err := waitForReachable(s.addr, timeout); err != nil {
		_ = s.stopLocked()
		return fmt.Errorf("dev: vite never became reachable on %s within %s: %w", s.addr, timeout, err)
	}
	slog.Info("dev: vite ready", "addr", s.addr)
	return nil
}

// Stop signals the spawned Vite to terminate (process-group SIGTERM)
// and waits for it to exit. No-op if we attached to an externally-
// managed Vite or never spawned one.
func (s *ViteSupervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *ViteSupervisor) stopLocked() error {
	if !s.spawned || s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(s.cmd.Process.Pid)
	if err == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = s.cmd.Process.Signal(syscall.SIGTERM)
	}
	// Give it a moment; if it ignores SIGTERM, force-kill.
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case waitErr := <-done:
		s.cmd = nil
		s.spawned = false
		// Exit-status errors from a SIGTERM kill are expected — only
		// surface unrelated failures.
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) {
			return nil
		}
		return waitErr
	case <-time.After(5 * time.Second):
		if pgid > 0 {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = s.cmd.Process.Kill()
		}
		<-done
		s.cmd = nil
		s.spawned = false
		return errors.New("dev: vite did not stop within 5s, force-killed")
	}
}

// reachable returns true if a TCP connection to the address's host:port
// succeeds within the given timeout.
func reachable(rawURL string, timeout time.Duration) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitForReachable(rawURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if reachable(rawURL, 250*time.Millisecond) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("timeout")
}

func portFromAddr(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "5173"
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "5173"
	}
	return port
}

// prefixedWriter forwards writes to `out`, prepending each line with
// `[prefix] ` so logs from the Vite child are distinguishable from the
// Go process's own slog output. Lines are buffered until the trailing
// newline arrives so multi-byte writes don't get split.
type prefixedWriter struct {
	prefix string
	out    io.Writer

	mu  sync.Mutex
	buf strings.Builder
}

func newPrefixedWriter(prefix string, out io.Writer) *prefixedWriter {
	return &prefixedWriter{prefix: prefix, out: out}
}

func (w *prefixedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.buf.WriteString(string(p))
	for {
		s := w.buf.String()
		nl := strings.IndexByte(s, '\n')
		if nl < 0 {
			break
		}
		line := s[:nl]
		fmt.Fprintf(w.out, "[%s] %s\n", w.prefix, line)
		w.buf.Reset()
		_, _ = w.buf.WriteString(s[nl+1:])
	}
	return len(p), nil
}
