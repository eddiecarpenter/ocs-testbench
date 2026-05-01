package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountTemplates registers all /templates routes on r.
func mountTemplates(r chi.Router, s store.Store) {
	r.Get("/templates", listTemplates(s))
	r.Post("/templates", createTemplate(s))
	r.Get("/templates/{id}", getTemplate(s))
	r.Put("/templates/{id}", updateTemplate(s))
	r.Delete("/templates/{id}", deleteTemplate(s))
}

// templateRequest is the JSON request body for AVPTemplate create and
// update operations.
type templateRequest struct {
	Name string          `json:"name"`
	Body json.RawMessage `json:"body"`
}

// templateResponse is the JSON response shape for an AVPTemplate.
type templateResponse struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Body json.RawMessage `json:"body"`
}

// toTemplateResponse converts a store.AVPTemplate to a
// templateResponse.
func toTemplateResponse(t store.AVPTemplate) templateResponse {
	return templateResponse{
		ID:   uuidToString(t.ID),
		Name: t.Name,
		Body: t.Body,
	}
}

// listTemplates handles GET /templates.
func listTemplates(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templates, err := s.ListAVPTemplates(r.Context())
		if err != nil {
			respondInternalError(w)
			return
		}
		out := make([]templateResponse, len(templates))
		for i, t := range templates {
			out[i] = toTemplateResponse(t)
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// createTemplate handles POST /templates.
func createTemplate(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req templateRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		tpl, err := s.InsertAVPTemplate(r.Context(), req.Name, req.Body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toTemplateResponse(tpl))
	}
}

// getTemplate handles GET /templates/{id}.
func getTemplate(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		tpl, err := s.GetAVPTemplate(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toTemplateResponse(tpl))
	}
}

// updateTemplate handles PUT /templates/{id}.
func updateTemplate(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		var req templateRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		tpl, err := s.UpdateAVPTemplate(r.Context(), id, req.Name, req.Body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toTemplateResponse(tpl))
	}
}

// deleteTemplate handles DELETE /templates/{id}.
func deleteTemplate(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		err := s.DeleteAVPTemplate(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
