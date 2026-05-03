package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// PeerManager is the interface the API layer uses to control peer
// connections. Defined here (at the point of consumption) so the API
// package is decoupled from the concrete manager type. The production
// wiring supplies *manager.Manager; tests supply a fake that satisfies
// the interface without spinning up a live Diameter connection.
type PeerManager interface {
	// Connect initiates the connection lifecycle for the named peer.
	// Returns manager.ErrUnknownPeer if the peer is not registered
	// and manager.ErrNotStarted if the Manager has not been started.
	Connect(name string) error

	// Disconnect gracefully closes the named peer's connection.
	// Returns manager.ErrUnknownPeer if the peer is not registered.
	Disconnect(name string) error

	// State returns the current connection state for the named peer.
	// Returns manager.ErrUnknownPeer if the peer is not registered.
	State(name string) (diameter.ConnectionState, error)

	// Subscribe returns a manager-level StateEvent channel that fans
	// out events from every registered peer. Used by the SSE
	// streaming handler in Task 9.
	Subscribe() <-chan diameter.StateEvent
}

// Router constructs and returns the chi.Router for the OCS Testbench
// REST API. The middleware stack is applied first (innermost →
// outermost: Recovery, RequestID, RequestLogger). All route groups are
// pre-wired so route registration is stable across tasks; handler
// bodies return 501 until each later task fills them in.
//
// mgr may be nil — if nil, the peer connection-control endpoints
// return 503 Service Unavailable until the Manager is wired in. In
// practice, production always supplies a started Manager.
//
// The caller is responsible for mounting the returned router onto an
// outer chi.Router (typically in cmd/ocs-testbench/main.go) under the
// /api prefix.
func Router(s store.Store, mgr PeerManager) chi.Router {
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

	// All versioned routes live under /v1.
	r.Route("/v1", func(v1 chi.Router) {
		// CRUD routes for all five entities.
		mountPeers(v1, s, mgr)
		mountSubscribers(v1, s)
		mountTemplates(v1, s)
		mountScenarios(v1, s)
		mountDictionaries(v1, s)
		mountDashboard(v1, s, mgr)

		// Execution control (Task 8) — blocked on Feature #19 landing.
		// SSE streaming (Task 9) — wired with peer SSE; execution SSE
		// blocked on Feature #19.
		mountSSE(v1, s, mgr)
	})

	return r
}

// notImplemented is a temporary handler placeholder. Routes wired in
// Task 1 that are implemented by later tasks return 501 until replaced.
func notImplemented(w http.ResponseWriter, _ *http.Request) {
	respondError(w, http.StatusNotImplemented, CodeInternalError, "not yet implemented")
}

// managerUnavailable returns a 503 Service Unavailable response when
// the PeerManager is not available.
func managerUnavailable(w http.ResponseWriter) {
	respondError(w, http.StatusServiceUnavailable, CodeInternalError, "peer manager not available")
}
