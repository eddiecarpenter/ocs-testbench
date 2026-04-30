package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// withSink replaces the package-level sink for the duration of the
// test. Returns a cleanup that restores the prior sink.
func withSink(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := sink
	sink = buf
	t.Cleanup(func() {
		sink = prev
		// Reset slog default to a benign handler to avoid leaking the
		// test buffer into other tests' output.
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		levelVar.Set(slog.LevelInfo)
	})
	return buf
}

// TestBootstrap_DefaultsToInfoText — Bootstrap installs a text
// handler at INFO level; debug calls are suppressed.
func TestBootstrap_DefaultsToInfoText(t *testing.T) {
	buf := withSink(t)
	Bootstrap()

	Debug("debug-line", "k", "v")
	Info("info-line", "k", "v")

	out := buf.String()
	if strings.Contains(out, "debug-line") {
		t.Errorf("debug should be suppressed at INFO; got: %s", out)
	}
	if !strings.Contains(out, "info-line") {
		t.Errorf("info-line should be present; got: %s", out)
	}
	// Text format produces a key=value-shaped line, not JSON.
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("Bootstrap should produce text output, not JSON; got: %s", out)
	}
}

// TestConfigure_JSON_SuppressesDebugAtInfo — format=json, level=info:
// output is JSON, debug is suppressed.
func TestConfigure_JSON_SuppressesDebugAtInfo(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "json", Level: "info"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	Debug("debug-line")
	Info("info-line", "request_id", "r-1")

	out := buf.String()
	if strings.Contains(out, "debug-line") {
		t.Errorf("debug should be suppressed at INFO; got: %s", out)
	}
	// Each emitted record must be a parsable JSON object.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("JSON parse failed for %q: %v", line, err)
		}
	}
}

// TestConfigure_Text_AtDebug_IncludesDebug — format=text, level=debug:
// debug records appear and output is not JSON.
func TestConfigure_Text_AtDebug_IncludesDebug(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "text", Level: "debug"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	Debug("debug-line")
	Info("info-line")

	out := buf.String()
	if !strings.Contains(out, "debug-line") {
		t.Errorf("debug-line should be present at DEBUG; got: %s", out)
	}
	if !strings.Contains(out, "info-line") {
		t.Errorf("info-line should be present; got: %s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("Configure(text) should not produce JSON; got: %s", out)
	}
}

// TestConfigure_RejectsInvalidFormat — invalid format returns an
// error and leaves the running handler intact.
func TestConfigure_RejectsInvalidFormat(t *testing.T) {
	withSink(t)
	Bootstrap() // text/info baseline
	if err := Configure(baseconfig.LogConfig{Format: "xml", Level: "info"}); err == nil {
		t.Error("expected error for invalid format")
	}
}

// TestConfigure_RejectsInvalidLevel — invalid level returns an
// error and leaves the running handler intact.
func TestConfigure_RejectsInvalidLevel(t *testing.T) {
	withSink(t)
	Bootstrap()
	if err := Configure(baseconfig.LogConfig{Format: "text", Level: "trace"}); err == nil {
		t.Error("expected error for invalid level")
	}
}

// TestSetLevel_UpdatesAtRuntime — flipping from info to debug at
// runtime begins emitting debug records.
func TestSetLevel_UpdatesAtRuntime(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "text", Level: "info"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	Debug("first-debug")
	if strings.Contains(buf.String(), "first-debug") {
		t.Fatal("debug should be suppressed at INFO")
	}
	if err := SetLevel("debug"); err != nil {
		t.Fatalf("SetLevel(debug): %v", err)
	}
	Debug("second-debug")
	if !strings.Contains(buf.String(), "second-debug") {
		t.Errorf("debug should appear after SetLevel(debug); got: %s", buf.String())
	}
}

// TestSetLevel_RejectsInvalid — invalid level returns an error and
// leaves the running level untouched.
func TestSetLevel_RejectsInvalid(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "text", Level: "info"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := SetLevel("bogus"); err == nil {
		t.Fatal("expected error for invalid level")
	}
	Debug("should-not-appear")
	if strings.Contains(buf.String(), "should-not-appear") {
		t.Error("level should remain at INFO after invalid SetLevel")
	}
}

// TestWithFrom_RoundTrip — attaching attributes via With produces a
// logger that emits them; From returns that logger.
func TestWithFrom_RoundTrip(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "json", Level: "info"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	ctx := With(context.Background(), slog.String("request_id", "rid-123"))
	From(ctx).Info("scoped")

	out := buf.String()
	if !strings.Contains(out, `"request_id":"rid-123"`) {
		t.Errorf("scoped attribute missing from output: %s", out)
	}
}

// TestFrom_NilSafe — nil context returns the default logger; calling
// it must not panic.
func TestFrom_NilSafe(t *testing.T) {
	withSink(t)
	Bootstrap()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("From(nil) panicked: %v", r)
		}
	}()
	if l := From(nil); l == nil {
		t.Error("From(nil) returned nil logger")
	}
}

// TestFrom_NoAttachedLogger — a context without a logger attached
// still returns a non-nil default.
func TestFrom_NoAttachedLogger(t *testing.T) {
	withSink(t)
	Bootstrap()
	if l := From(context.Background()); l == nil {
		t.Error("From(empty ctx) returned nil logger")
	}
}

// TestWith_NilContext — With(nil, ...) does not panic and yields a
// usable context.
func TestWith_NilContext(t *testing.T) {
	withSink(t)
	Bootstrap()
	ctx := With(nil, slog.String("k", "v"))
	if ctx == nil {
		t.Fatal("With(nil, ...) returned nil context")
	}
	if From(ctx) == nil {
		t.Error("From returned nil from With(nil) result")
	}
}

// TestRequestLogger_LogsAttributes — middleware wraps a sample
// handler and emits a log line carrying method, path, status, and a
// non-zero duration.
func TestRequestLogger_LogsAttributes(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "json", Level: "info"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(RequestLogger(handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/foo")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	out := buf.String()
	for _, want := range []string{`"method":"GET"`, `"path":"/foo"`, `"status":418`, `"duration":`} {
		if !strings.Contains(out, want) {
			t.Errorf("RequestLogger output missing %q in: %s", want, out)
		}
	}
}

// TestRequestLogger_ImplicitOK — handler that doesn't call
// WriteHeader is logged as 200.
func TestRequestLogger_ImplicitOK(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "json", Level: "info"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(RequestLogger(handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/bar")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(buf.String(), `"status":200`) {
		t.Errorf("RequestLogger should log implicit 200; got: %s", buf.String())
	}
}

// TestFatal_Subprocess — Fatal exits with code 1. Re-runs the test
// binary with an env-var marker so the actual os.Exit happens in a
// child process, leaving the test runner unaffected.
func TestFatal_Subprocess(t *testing.T) {
	if os.Getenv("LOG_TEST_FATAL") == "1" {
		// In the child: install a discard handler so we don't pollute
		// CI logs, then call Fatal.
		sink = io.Discard
		Bootstrap()
		Fatal("fatal-line")
		return // unreachable
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestFatal_Subprocess$")
	cmd.Env = append(os.Environ(), "LOG_TEST_FATAL=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit; got nil")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError; got %T: %v", err, err)
	}
	if got := exitErr.ExitCode(); got != 1 {
		t.Errorf("Fatal exit code: got %d, want 1", got)
	}
}

// TestFatal_DoesNotExitWhenHookOverridden — the in-process exit hook
// allows tests to call Fatal and assert side-effects without
// terminating the runner. Confirms the seam works.
func TestFatal_DoesNotExitWhenHookOverridden(t *testing.T) {
	buf := withSink(t)
	Bootstrap()

	prevExit := exitFn
	var (
		mu       sync.Mutex
		exitedAt int = -1
	)
	exitFn = func(code int) {
		mu.Lock()
		defer mu.Unlock()
		exitedAt = code
	}
	t.Cleanup(func() { exitFn = prevExit })

	Fatal("hook-test", "k", "v")

	mu.Lock()
	defer mu.Unlock()
	if exitedAt != 1 {
		t.Errorf("exit hook called with %d; want 1", exitedAt)
	}
	if !strings.Contains(buf.String(), "hook-test") {
		t.Errorf("Fatal should log before exiting; got: %s", buf.String())
	}
}

// TestModuleLevelHelpers — Debug/Info/Warn/Error each route to the
// right slog level.
func TestModuleLevelHelpers(t *testing.T) {
	buf := withSink(t)
	if err := Configure(baseconfig.LogConfig{Format: "json", Level: "debug"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	Debug("d-msg")
	Info("i-msg")
	Warn("w-msg")
	Error("e-msg")

	out := buf.String()
	for _, want := range []string{`"level":"DEBUG"`, `"level":"INFO"`, `"level":"WARN"`, `"level":"ERROR"`,
		`"msg":"d-msg"`, `"msg":"i-msg"`, `"msg":"w-msg"`, `"msg":"e-msg"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %s", want, out)
		}
	}
}
