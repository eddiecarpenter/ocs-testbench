package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountScenarios registers all /scenarios routes on r.
func mountScenarios(r chi.Router, s store.Store) {
	r.Get("/scenarios", listScenarios(s))
	r.Post("/scenarios", createScenario(s))
	r.Get("/scenarios/{id}", getScenario(s))
	r.Put("/scenarios/{id}", updateScenario(s))
	r.Delete("/scenarios/{id}", deleteScenario(s))
}

// scenarioRequest is the JSON request body for Scenario create and
// update operations.
type scenarioRequest struct {
	Name       string          `json:"name"`
	TemplateID string          `json:"templateId"`
	PeerID     string          `json:"peerId"`
	Body       json.RawMessage `json:"body"`
}

// scenarioResponse is the JSON response shape for a Scenario.
type scenarioResponse struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	TemplateID string          `json:"templateId"`
	PeerID     string          `json:"peerId"`
	Body       json.RawMessage `json:"body"`
}

// toScenarioResponse converts a store.Scenario to a scenarioResponse.
func toScenarioResponse(sc store.Scenario) scenarioResponse {
	return scenarioResponse{
		ID:         uuidToString(sc.ID),
		Name:       sc.Name,
		TemplateID: uuidToString(sc.TemplateID),
		PeerID:     uuidToString(sc.PeerID),
		Body:       sc.Body,
	}
}

// parseRequiredUUID parses a UUID from a string field in a request
// body. Writes a 400 response if the string is empty or not a valid
// UUID, and returns ok=false.
func parseRequiredUUID(w http.ResponseWriter, fieldName, value string) (pgtype.UUID, bool) {
	if value == "" {
		respondInvalidRequest(w, fieldName+" is required")
		return pgtype.UUID{}, false
	}
	id, err := uuidFromString(value)
	if err != nil {
		respondInvalidRequest(w, "invalid "+fieldName+": must be a valid UUID")
		return pgtype.UUID{}, false
	}
	return id, true
}

// listScenarios handles GET /scenarios.
func listScenarios(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarios, err := s.ListScenarios(r.Context())
		if err != nil {
			respondInternalError(w)
			return
		}
		out := make([]scenarioResponse, len(scenarios))
		for i, sc := range scenarios {
			out[i] = toScenarioResponse(sc)
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// createScenario handles POST /scenarios.
func createScenario(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req scenarioRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		tplID, ok := parseRequiredUUID(w, "templateId", req.TemplateID)
		if !ok {
			return
		}
		peerID, ok := parseRequiredUUID(w, "peerId", req.PeerID)
		if !ok {
			return
		}
		sc, err := s.InsertScenario(r.Context(), req.Name, tplID, peerID, req.Body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toScenarioResponse(sc))
	}
}

// getScenario handles GET /scenarios/{id}.
func getScenario(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		sc, err := s.GetScenario(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toScenarioResponse(sc))
	}
}

// updateScenario handles PUT /scenarios/{id}.
func updateScenario(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		var req scenarioRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		tplID, ok := parseRequiredUUID(w, "templateId", req.TemplateID)
		if !ok {
			return
		}
		peerID, ok := parseRequiredUUID(w, "peerId", req.PeerID)
		if !ok {
			return
		}
		sc, err := s.UpdateScenario(r.Context(), store.UpdateScenarioParams{
			ID:         id,
			Name:       req.Name,
			TemplateID: tplID,
			PeerID:     peerID,
			Body:       req.Body,
		})
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toScenarioResponse(sc))
	}
}

// deleteScenario handles DELETE /scenarios/{id}.
func deleteScenario(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		err := s.DeleteScenario(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
