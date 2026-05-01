package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountDictionaries registers all /dictionaries routes on r.
// Handlers are implemented by Task 6.
func mountDictionaries(r chi.Router, s store.Store) {
	_ = s // consumed by Task 6 handlers
	r.Get("/dictionaries", notImplemented)
	r.Post("/dictionaries", notImplemented)
	r.Get("/dictionaries/{id}", notImplemented)
	r.Put("/dictionaries/{id}", notImplemented)
	r.Delete("/dictionaries/{id}", notImplemented)
}
