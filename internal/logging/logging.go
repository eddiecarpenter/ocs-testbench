// Package logging is a thin facade over Go's log/slog standard library.
//
// The package exposes a Bootstrap/Configure lifecycle so the binary
// has a working logger from the very first line of main() (Bootstrap),
// and can be re-targeted with the operator-supplied format and level
// once the configuration file has been parsed (Configure).
//
// Module-level helpers (Debug, Info, Warn, Error, Fatal) delegate to
// slog.Default() so calling code does not need to thread a *slog.Logger
// through every signature. Request-scoped attributes are attached and
// retrieved via With and From; a chi-compatible HTTP middleware is
// provided as RequestLogger but the package itself does not import
// chi — RequestLogger is built against net/http and composes with any
// router accepting an http.Handler middleware.
//
// Runtime level changes are atomic: SetLevel updates a package-level
// slog.LevelVar that is shared across every handler this package
// installs.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// levelVar is the shared LevelVar referenced by every handler this
// package installs. SetLevel mutates it atomically; the running
// handler picks up the new level on its next log call.
var levelVar = new(slog.LevelVar)

// sink is the io.Writer every installed handler writes to. The
// package-level variable is overridable from tests so the produced
// output can be captured and asserted against.
var sink io.Writer = os.Stderr

// exitFn is the os.Exit hook used by Fatal. Overridable from tests
// so a Fatal call doesn't terminate the test runner.
var exitFn = os.Exit

// Bootstrap installs a default text handler at INFO level so log
// output is observable before the configuration file is loaded. Call
// once at the very start of main(). Calling Bootstrap after Configure
// resets the handler to the early-startup default.
func Bootstrap() {
	levelVar.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: levelVar})))
}

// Configure replaces the default handler with one matching the
// configured format and level. Format must be "json" or "text"; level
// must be "debug", "info", "warn", or "error". On invalid input the
// handler is left untouched and an error is returned.
func Configure(cfg baseconfig.LogConfig) error {
	lvl, err := parseLevel(cfg.Level)
	if err != nil {
		return err
	}

	opts := &slog.HandlerOptions{Level: levelVar}
	var h slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "json":
		h = slog.NewJSONHandler(sink, opts)
	case "text":
		h = slog.NewTextHandler(sink, opts)
	default:
		return fmt.Errorf("logging: unknown format %q (want json|text)", cfg.Format)
	}

	levelVar.Set(lvl)
	slog.SetDefault(slog.New(h))
	return nil
}

// SetLevel updates the active handler's level atomically. Accepts the
// same level names as Configure. Returns an error and leaves the
// running level untouched when the input is invalid.
func SetLevel(level string) error {
	lvl, err := parseLevel(level)
	if err != nil {
		return err
	}
	levelVar.Set(lvl)
	return nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: unknown level %q (want debug|info|warn|error)", s)
	}
}

// Debug logs at slog.LevelDebug via slog.Default().
func Debug(msg string, args ...any) { slog.Default().Debug(msg, args...) }

// Info logs at slog.LevelInfo via slog.Default().
func Info(msg string, args ...any) { slog.Default().Info(msg, args...) }

// Warn logs at slog.LevelWarn via slog.Default().
func Warn(msg string, args ...any) { slog.Default().Warn(msg, args...) }

// Error logs at slog.LevelError via slog.Default().
func Error(msg string, args ...any) { slog.Default().Error(msg, args...) }

// Fatal logs at slog.LevelError via slog.Default() and then calls
// os.Exit(1). Tests override the underlying exit hook so the test
// runner is not terminated.
func Fatal(msg string, args ...any) {
	slog.Default().Error(msg, args...)
	exitFn(1)
}

// loggerKey is the context key used by With/From. The empty struct
// keeps the key unique to this package.
type loggerKey struct{}

// With attaches a logger derived from the existing one in ctx (or
// slog.Default() when ctx has none) carrying the given attributes.
// Safe with a nil context — a fresh background context is used in
// that case. Subsequent calls overwrite any previously-attached
// logger.
func With(ctx context.Context, attrs ...slog.Attr) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	parent := From(ctx)
	args := make([]any, 0, len(attrs))
	for _, a := range attrs {
		args = append(args, a)
	}
	return context.WithValue(ctx, loggerKey{}, parent.With(args...))
}

// From retrieves the logger attached to ctx, falling back to
// slog.Default() when ctx is nil or has no attached logger.
func From(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if v := ctx.Value(loggerKey{}); v != nil {
		if l, ok := v.(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// RequestLogger returns a middleware that logs method, path, status,
// and duration for each handled request. Built against net/http
// directly so it composes with any router accepting http.Handler
// middleware (chi, gorilla/mux, stdlib).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Default().Info("http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// statusRecorder captures the status code written by the wrapped
// handler. http.ResponseWriter does not expose the status post-write,
// so wrapping is the only way to surface it for the middleware log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

// WriteHeader records the status before delegating to the wrapped
// writer.
func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.wrote = true
	sr.ResponseWriter.WriteHeader(code)
}

// Write delegates to the wrapped writer; if WriteHeader was not
// called explicitly, the recorded status remains the constructor
// default (http.StatusOK), which matches stdlib behaviour.
func (sr *statusRecorder) Write(b []byte) (int, error) {
	sr.wrote = true
	return sr.ResponseWriter.Write(b)
}

// Flush implements http.Flusher by delegating to the underlying
// ResponseWriter when it supports flushing. This makes statusRecorder
// transparent to callers that type-assert to http.Flusher (e.g. SSE
// streaming handlers).
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
