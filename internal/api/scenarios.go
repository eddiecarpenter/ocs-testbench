package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountScenarios registers all /scenarios routes on r.
// Handlers are implemented by Task 5.
func mountScenarios(r chi.Router, s store.Store) {
	_ = s // consumed by Task 5 handlers
	r.Get("/scenarios", notImplemented)
	r.Post("/scenarios", notImplemented)
	r.Get("/scenarios/{id}", notImplemented)
	r.Put("/scenarios/{id}", notImplemented)
	r.Delete("/scenarios/{id}", notImplemented)
}
