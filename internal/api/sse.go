package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountSSE registers the Server-Sent Events streaming endpoints.
// Peer SSE (GET /events/peers) is implemented here.
// Execution SSE (GET /events/executions/{id}) is blocked on Feature
// #19 (internal/engine/) and will be implemented in Task 9 when that
// feature lands.
func mountSSE(r chi.Router, s store.Store, mgr PeerManager) {
	_ = s // used by Task 9
	r.Get("/events/peers", notImplemented) // Task 9 (peer SSE, immediate)
	r.Get("/events/executions/{id}", notImplemented) // Task 9 (blocked on #19)
}
