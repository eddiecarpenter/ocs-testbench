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

// scenarioBody is the JSONB payload stored in the body column. It holds
// every ScenarioInput field that is not promoted to its own column
// (name, peerId, subscriberId are stored as columns).
type scenarioBody struct {
	Description  string          `json:"description,omitempty"`
	UnitType     string          `json:"unitType"`
	SessionMode  string          `json:"sessionMode"`
	ServiceModel string          `json:"serviceModel"`
	Favourite    bool            `json:"favourite,omitempty"`
	AvpTree      json.RawMessage `json:"avpTree"`
	Services     json.RawMessage `json:"services"`
	Variables    json.RawMessage `json:"variables"`
	Steps        json.RawMessage `json:"steps"`
}

// scenarioRequest is the decoded form of a ScenarioInput JSON body.
type scenarioRequest struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	UnitType     string          `json:"unitType"`
	SessionMode  string          `json:"sessionMode"`
	ServiceModel string          `json:"serviceModel"`
	Favourite    bool            `json:"favourite"`
	SubscriberID string          `json:"subscriberId"`
	PeerID       string          `json:"peerId"`
	AvpTree      json.RawMessage `json:"avpTree"`
	Services     json.RawMessage `json:"services"`
	Variables    json.RawMessage `json:"variables"`
	Steps        json.RawMessage `json:"steps"`
}

// scenarioSummaryResponse matches the OpenAPI ScenarioSummary shape.
type scenarioSummaryResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	UnitType     string `json:"unitType"`
	SessionMode  string `json:"sessionMode"`
	ServiceModel string `json:"serviceModel"`
	Origin       string `json:"origin"`
	Favourite    bool   `json:"favourite"`
	SubscriberID string `json:"subscriberId,omitempty"`
	PeerID       string `json:"peerId,omitempty"`
	StepCount    int    `json:"stepCount"`
	UpdatedAt    string `json:"updatedAt"`
}

// scenarioFullResponse extends scenarioSummaryResponse with the full
// body content — matches the OpenAPI Scenario shape.
type scenarioFullResponse struct {
	scenarioSummaryResponse
	AvpTree   json.RawMessage `json:"avpTree"`
	Services  json.RawMessage `json:"services"`
	Variables json.RawMessage `json:"variables"`
	Steps     json.RawMessage `json:"steps"`
}

// countJSONArray returns the number of elements in a JSON array, or 0
// if the value is null/invalid.
func countJSONArray(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return 0
	}
	return len(elems)
}

// nullableJSON returns null JSON (json.RawMessage("null")) when raw is
// empty, so callers always get valid JSON in the response.
func nullableJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

// toSummaryResponse converts a store.Scenario to a summary response,
// decoding the body to extract the list-level fields.
func toSummaryResponse(sc store.Scenario) scenarioSummaryResponse {
	var b scenarioBody
	_ = json.Unmarshal(sc.Body, &b)
	return scenarioSummaryResponse{
		ID:           uuidToString(sc.ID),
		Name:         sc.Name,
		Description:  b.Description,
		UnitType:     b.UnitType,
		SessionMode:  b.SessionMode,
		ServiceModel: b.ServiceModel,
		Origin:       "user",
		Favourite:    b.Favourite,
		SubscriberID: uuidToString(sc.SubscriberID),
		PeerID:       uuidToString(sc.PeerID),
		StepCount:    countJSONArray(b.Steps),
		UpdatedAt:    sc.UpdatedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// toFullResponse converts a store.Scenario to the full Scenario response.
func toFullResponse(sc store.Scenario) scenarioFullResponse {
	var b scenarioBody
	_ = json.Unmarshal(sc.Body, &b)
	return scenarioFullResponse{
		scenarioSummaryResponse: toSummaryResponse(sc),
		AvpTree:                 nullableJSON(b.AvpTree),
		Services:                nullableJSON(b.Services),
		Variables:               nullableJSON(b.Variables),
		Steps:                   nullableJSON(b.Steps),
	}
}

// buildBody serialises the non-column ScenarioInput fields into the
// JSONB body.
func buildBody(req scenarioRequest) ([]byte, error) {
	b := scenarioBody{
		Description:  req.Description,
		UnitType:     req.UnitType,
		SessionMode:  req.SessionMode,
		ServiceModel: req.ServiceModel,
		Favourite:    req.Favourite,
		AvpTree:      req.AvpTree,
		Services:     req.Services,
		Variables:    req.Variables,
		Steps:        req.Steps,
	}
	return json.Marshal(b)
}

// optionalUUID parses a string as a UUID, returning a null UUID if the
// string is empty (field is optional).
func optionalUUID(s string) (pgtype.UUID, bool) {
	if s == "" {
		return pgtype.UUID{}, true
	}
	id, err := uuidFromString(s)
	if err != nil {
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
		out := make([]scenarioSummaryResponse, len(scenarios))
		for i, sc := range scenarios {
			out[i] = toSummaryResponse(sc)
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
		peerID, ok := optionalUUID(req.PeerID)
		if !ok {
			respondInvalidRequest(w, "invalid peerId: must be a valid UUID")
			return
		}
		subscriberID, ok := optionalUUID(req.SubscriberID)
		if !ok {
			respondInvalidRequest(w, "invalid subscriberId: must be a valid UUID")
			return
		}
		body, err := buildBody(req)
		if err != nil {
			respondInternalError(w)
			return
		}
		sc, err := s.InsertScenario(r.Context(), req.Name, peerID, subscriberID, body)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusCreated, toFullResponse(sc))
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
		respondJSON(w, http.StatusOK, toFullResponse(sc))
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
		peerID, ok := optionalUUID(req.PeerID)
		if !ok {
			respondInvalidRequest(w, "invalid peerId: must be a valid UUID")
			return
		}
		subscriberID, ok := optionalUUID(req.SubscriberID)
		if !ok {
			respondInvalidRequest(w, "invalid subscriberId: must be a valid UUID")
			return
		}
		body, err := buildBody(req)
		if err != nil {
			respondInternalError(w)
			return
		}
		sc, err := s.UpdateScenario(r.Context(), store.UpdateScenarioParams{
			ID:           id,
			Name:         req.Name,
			PeerID:       peerID,
			SubscriberID: subscriberID,
			Body:         body,
		})
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toFullResponse(sc))
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
