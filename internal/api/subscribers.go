package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountSubscribers registers all /subscribers routes on r.
// Handlers are implemented by Task 3.
func mountSubscribers(r chi.Router, s store.Store) {
	_ = s // consumed by Task 3 handlers
	r.Get("/subscribers", notImplemented)
	r.Post("/subscribers", notImplemented)
	r.Get("/subscribers/{id}", notImplemented)
	r.Put("/subscribers/{id}", notImplemented)
	r.Delete("/subscribers/{id}", notImplemented)
}
