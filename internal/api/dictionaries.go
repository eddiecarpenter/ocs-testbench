package api

import (
	"bytes"
	"net/http"

	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountDictionaries registers all /dictionaries routes on r.
//
// parser, when non-nil, is the live dict.Parser (typically dict.Default)
// into which newly-created dictionaries are loaded immediately so that
// the running process picks them up without a restart. Updates and
// deletes are store-only and still require a restart because the
// underlying go-diameter parser is additive-only.
func mountDictionaries(r chi.Router, s store.Store, parser *dict.Parser) {
	r.Get("/dictionaries", listDictionaries(s))
	r.Post("/dictionaries", createDictionary(s, parser))
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
//
// When parser is non-nil and the new dictionary is active, its XML is
// loaded into the live parser immediately after the store insert so
// the running process can resolve the new AVPs without a restart.
// If the XML loads successfully a log line at INFO is emitted; if it
// fails the store row is still committed and a WARN is logged — the
// caller should restart to retry loading.
func createDictionary(s store.Store, parser *dict.Parser) http.HandlerFunc {
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
		created, err := s.InsertCustomDictionary(
			r.Context(),
			req.Name,
			pgtype.Text{String: req.Description, Valid: req.Description != ""},
			req.XmlContent,
			req.IsActive,
		)
		if mapStoreError(w, err) != nil {
			return
		}

		// Hot-load into the live parser so the new AVPs are available
		// without a server restart. Only attempted when:
		//   - a live parser was injected (production path), and
		//   - the dictionary is marked active.
		// The parser is additive-only, so duplicate entries will return
		// an error here; we log-and-continue rather than failing the
		// HTTP request — the row is already committed.
		if parser != nil && req.IsActive {
			if loadErr := parser.Load(bytes.NewReader([]byte(req.XmlContent))); loadErr != nil {
				logging.Warn("dictionary: hot-load failed — restart to apply",
					"name", req.Name, "error", loadErr.Error())
			} else {
				logging.Info("dictionary: hot-loaded into live parser", "name", req.Name)
			}
		}

		respondJSON(w, http.StatusCreated, toDictionaryResponse(created))
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
