package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountDictionaries registers all /dictionaries routes on r.
func mountDictionaries(r chi.Router, s store.Store) {
	r.Get("/dictionaries", listDictionaries(s))
	r.Post("/dictionaries", createDictionary(s))
	r.Get("/dictionaries/{id}", getDictionary(s))
	r.Put("/dictionaries/{id}", updateDictionary(s))
	r.Delete("/dictionaries/{id}", deleteDictionary(s))
}

// dictionaryRequest is the JSON request body for CustomDictionary
// create and update operations.
type dictionaryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	XmlContent  string `json:"xmlContent"`
	IsActive    bool   `json:"isActive"`
}

// dictionaryResponse is the JSON response shape for a CustomDictionary.
type dictionaryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	XmlContent  string `json:"xmlContent"`
	IsActive    bool   `json:"isActive"`
}

// toDictionaryResponse converts a store.CustomDictionary to a
// dictionaryResponse.
func toDictionaryResponse(d store.CustomDictionary) dictionaryResponse {
	return dictionaryResponse{
		ID:          uuidToString(d.ID),
		Name:        d.Name,
		Description: d.Description.String,
		XmlContent:  d.XmlContent,
		IsActive:    d.IsActive,
	}
}

// listDictionaries handles GET /dictionaries.
func listDictionaries(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dicts, err := s.ListCustomDictionaries(r.Context())
		if err != nil {
			respondInternalError(w)
			return
		}
		out := make([]dictionaryResponse, len(dicts))
		for i, d := range dicts {
			out[i] = toDictionaryResponse(d)
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// createDictionary handles POST /dictionaries.
func createDictionary(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req dictionaryRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		if req.XmlContent == "" {
			respondInvalidRequest(w, "xmlContent is required")
			return
		}
		dict, err := s.InsertCustomDictionary(
			r.Context(),
			req.Name,
			pgtype.Text{String: req.Description, Valid: req.Description != ""},
			req.XmlContent,
			req.IsActive,
		)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toDictionaryResponse(dict))
	}
}

// getDictionary handles GET /dictionaries/{id}.
func getDictionary(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		dict, err := s.GetCustomDictionary(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toDictionaryResponse(dict))
	}
}

// updateDictionary handles PUT /dictionaries/{id}.
func updateDictionary(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		var req dictionaryRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		if req.XmlContent == "" {
			respondInvalidRequest(w, "xmlContent is required")
			return
		}
		dict, err := s.UpdateCustomDictionary(r.Context(), store.UpdateCustomDictionaryParams{
			ID:          id,
			Name:        req.Name,
			Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
			XmlContent:  req.XmlContent,
			IsActive:    req.IsActive,
		})
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toDictionaryResponse(dict))
	}
}

// deleteDictionary handles DELETE /dictionaries/{id}.
func deleteDictionary(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		err := s.DeleteCustomDictionary(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
