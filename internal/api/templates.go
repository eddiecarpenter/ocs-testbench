package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountTemplates registers all /templates routes on r.
// Handlers are implemented by Task 4.
func mountTemplates(r chi.Router, s store.Store) {
	_ = s // consumed by Task 4 handlers
	r.Get("/templates", notImplemented)
	r.Post("/templates", notImplemented)
	r.Get("/templates/{id}", notImplemented)
	r.Put("/templates/{id}", notImplemented)
	r.Delete("/templates/{id}", notImplemented)
}
