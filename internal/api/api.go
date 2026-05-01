package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// Router constructs and returns the chi.Router for the OCS Testbench
// REST API. The middleware stack is applied first (innermost →
// outermost: Recovery, RequestID, RequestLogger). All route groups are
// pre-wired so route registration is stable across tasks; handler
// bodies return 501 until each later task fills them in.
//
// The caller is responsible for mounting the returned router onto an
// outer chi.Router (typically in cmd/ocs-testbench/main.go) under the
// /api prefix.
func Router(s store.Store) chi.Router {
	r := chi.NewRouter()

	// Middleware stack — innermost to outermost:
	//   Recovery → RequestID → RequestLogger (logging from internal/logging).
	r.Use(Recovery)
	r.Use(RequestID)
	r.Use(logging.RequestLogger)

	// Custom 404 handler so unknown endpoints return a structured JSON
	// error body instead of chi's plain-text "404 page not found".
	// AC-2: unknown endpoint → 404 with {"error": {"code": ..., "message": ...}}.
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		respondError(w, http.StatusNotFound, CodeNotFound, "endpoint not found")
	})

	// CRUD routes for all five entities.
	// Handler bodies return 501 until later tasks fill them in.
	mountPeers(r, s)
	mountSubscribers(r, s)
	mountTemplates(r, s)
	mountScenarios(r, s)
	mountDictionaries(r, s)

	// Peer connection control (Task 7) — wired but handlers return
	// 501 until Task 7 injects the Manager dependency.
	// Execution control (Task 8) — wired but handlers return 501
	// until Feature #19 lands.
	// SSE streaming (Task 9) — same.

	return r
}

// notImplemented is a temporary handler placeholder. Routes wired in
// Task 1 that are implemented by later tasks return 501 until replaced.
func notImplemented(w http.ResponseWriter, _ *http.Request) {
	respondError(w, http.StatusNotImplemented, CodeInternalError, "not yet implemented")
}
