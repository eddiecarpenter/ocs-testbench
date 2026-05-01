package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountPeers registers all /peers and /peers/{id} routes on r.
// CRUD handlers are implemented by Task 2; connection-control handlers
// are implemented by Task 7.
func mountPeers(r chi.Router, s store.Store) {
	_ = s // consumed by Task 2 handlers
	r.Get("/peers", notImplemented)
	r.Post("/peers", notImplemented)
	r.Get("/peers/{id}", notImplemented)
	r.Put("/peers/{id}", notImplemented)
	r.Delete("/peers/{id}", notImplemented)

	// Connection control — implemented by Task 7
	r.Post("/peers/{id}/connect", notImplemented)
	r.Post("/peers/{id}/disconnect", notImplemented)
	r.Get("/peers/{id}/status", notImplemented)
}
