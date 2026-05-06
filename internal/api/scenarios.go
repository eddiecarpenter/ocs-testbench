package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/engine"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// mountScenarios registers all /scenarios routes on r.
func mountScenarios(r chi.Router, s store.Store, dict template.Dictionary) {
	r.Get("/scenarios", listScenarios(s))
	r.Post("/scenarios", createScenario(s, dict))
	r.Get("/scenarios/{id}", getScenario(s, dict))
	r.Put("/scenarios/{id}", updateScenario(s, dict))
	r.Delete("/scenarios/{id}", deleteScenario(s))
}

// scenarioBody is the JSONB payload stored in the body column. It holds
// every ScenarioInput field that is not promoted to its own column
// (name, peerId, subscriberId are stored as columns).
type scenarioBody struct {
	Description    string          `json:"description,omitempty"`
	SessionMode    string          `json:"sessionMode"`
	ServiceModel   string          `json:"serviceModel"`
	ServiceType    string          `json:"serviceType,omitempty"`
	ServiceProfile string          `json:"serviceProfile,omitempty"`
	Favourite      bool            `json:"favourite,omitempty"`
	AvpTree        json.RawMessage `json:"avpTree"`
	Services       json.RawMessage `json:"services"`
	Variables      json.RawMessage `json:"variables"`
	Steps          json.RawMessage `json:"steps"`
}

// scenarioRequest is the decoded form of a ScenarioInput JSON body.
type scenarioRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	SessionMode    string          `json:"sessionMode"`
	ServiceModel   string          `json:"serviceModel"`
	ServiceType    string          `json:"serviceType"`
	ServiceProfile string          `json:"serviceProfile"`
	Favourite      bool            `json:"favourite"`
	SubscriberID   string          `json:"subscriberId"`
	PeerID         string          `json:"peerId"`
	AvpTree        json.RawMessage `json:"avpTree"`
	Services       json.RawMessage `json:"services"`
	Variables      json.RawMessage `json:"variables"`
	Steps          json.RawMessage `json:"steps"`
}

// scenarioSummaryResponse matches the OpenAPI ScenarioSummary shape.
type scenarioSummaryResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	ServiceType    string `json:"serviceType,omitempty"`
	ServiceProfile string `json:"serviceProfile,omitempty"`
	SessionMode    string `json:"sessionMode"`
	ServiceModel   string `json:"serviceModel"`
	Origin         string `json:"origin"`
	Favourite      bool   `json:"favourite"`
	SubscriberID   string `json:"subscriberId,omitempty"`
	PeerID         string `json:"peerId,omitempty"`
	StepCount      int    `json:"stepCount"`
	UpdatedAt      string `json:"updatedAt"`
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
		ID:             uuidToString(sc.ID),
		Name:           sc.Name,
		Description:    b.Description,
		ServiceType:    b.ServiceType,
		ServiceProfile: b.ServiceProfile,
		SessionMode:    b.SessionMode,
		ServiceModel:   b.ServiceModel,
		Origin:         "user",
		Favourite:      b.Favourite,
		SubscriberID:   uuidToString(sc.SubscriberID),
		PeerID:         uuidToString(sc.PeerID),
		StepCount:      countJSONArray(b.Steps),
		UpdatedAt:      sc.UpdatedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// toFullResponse converts a store.Scenario to the full Scenario response,
// enriching each avpTree node with code/vendorId from the dictionary.
func toFullResponse(sc store.Scenario, dict template.Dictionary) scenarioFullResponse {
	var b scenarioBody
	_ = json.Unmarshal(sc.Body, &b)
	return scenarioFullResponse{
		scenarioSummaryResponse: toSummaryResponse(sc),
		AvpTree:                 enrichAvpTree(nullableJSON(b.AvpTree), dict),
		Services:                nullableJSON(b.Services),
		Variables:               nullableJSON(b.Variables),
		Steps:                   nullableJSON(b.Steps),
	}
}

// avpRichNode is the wire shape for an enriched avpTree node returned to the
// frontend. It mirrors AvpNode in the OpenAPI schema and carries code +
// vendorId resolved from the Diameter dictionary.
type avpRichNode struct {
	Name     string        `json:"name"`
	Code     uint32        `json:"code"`
	VendorID uint32        `json:"vendorId,omitempty"`
	ValueRef string        `json:"valueRef,omitempty"`
	Locked   bool          `json:"locked,omitempty"`
	Children []avpRichNode `json:"children,omitempty"`
}

// enrichAvpTree walks raw avpTree JSON and fills in missing code / vendorId
// for each node using the Diameter dictionary. Nodes already carrying a
// non-zero code are left unchanged. Unknown AVP names are left with code 0.
func enrichAvpTree(raw json.RawMessage, dict template.Dictionary) json.RawMessage {
	if dict == nil || len(raw) == 0 || string(raw) == "null" {
		return raw
	}
	var nodes []avpRichNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return raw
	}
	enrichNodes(nodes, dict)
	enriched, err := json.Marshal(nodes)
	if err != nil {
		return raw
	}
	return enriched
}

func enrichNodes(nodes []avpRichNode, dict template.Dictionary) {
	for i := range nodes {
		if nodes[i].Code == 0 {
			if meta, err := dict.Lookup(nodes[i].Name); err == nil {
				nodes[i].Code = meta.Code
				if nodes[i].VendorID == 0 && meta.VendorID != 0 {
					nodes[i].VendorID = meta.VendorID
				}
			}
		}
		if len(nodes[i].Children) > 0 {
			enrichNodes(nodes[i].Children, dict)
		}
	}
}

// buildBody serialises the non-column ScenarioInput fields into the
// JSONB body.
func buildBody(req scenarioRequest) ([]byte, error) {
	b := scenarioBody{
		Description:    req.Description,
		SessionMode:    req.SessionMode,
		ServiceModel:   req.ServiceModel,
		ServiceType:    req.ServiceType,
		ServiceProfile: req.ServiceProfile,
		Favourite:      req.Favourite,
		AvpTree:        req.AvpTree,
		Services:       req.Services,
		Variables:      req.Variables,
		Steps:          req.Steps,
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

// validateScenarioExpressions parses the steps and variables from the raw JSON
// request and checks that every expression field (repeatUntil, assertions,
// guards, resultHandler.when, derivedValues.expression) only references
// declared variables or well-known system variables.
// Returns a human-readable error string, or "" if everything is valid.
func validateScenarioExpressions(req scenarioRequest) string {
	// Minimal struct to extract variable names from the JSON array.
	type varDecl struct {
		Name string `json:"name"`
	}
	var varDecls []varDecl
	if len(req.Variables) > 0 {
		_ = json.Unmarshal(req.Variables, &varDecls)
	}
	declared := make([]string, 0, len(varDecls))
	for _, v := range varDecls {
		if v.Name != "" {
			declared = append(declared, v.Name)
		}
	}

	var steps []engine.ScenarioStep
	if len(req.Steps) > 0 {
		_ = json.Unmarshal(req.Steps, &steps)
	}

	var problems []string
	check := func(ctx, expr string) {
		if err := engine.ValidateExpr(declared, expr); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", ctx, err))
		}
	}

	for i, step := range steps {
		label := step.Label
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		check(label+" repeatUntil", step.RepeatUntil)
		for j, a := range step.Assertions {
			check(fmt.Sprintf("%s assertion[%d]", label, j), a)
		}
		for j, g := range step.Guards {
			check(fmt.Sprintf("%s guard[%d]", label, j), g)
		}
		for j, h := range step.ResultHandlers {
			check(fmt.Sprintf("%s resultHandler[%d].when", label, j), h.When)
		}
		for j, dv := range step.DerivedValues {
			check(fmt.Sprintf("%s derivedValues[%d]", label, j), dv.Expression)
		}
	}

	if len(problems) == 0 {
		return ""
	}
	msg := "expression validation failed:\n"
	for _, p := range problems {
		msg += "  • " + p + "\n"
	}
	return msg
}

// createScenario handles POST /scenarios.
func createScenario(s store.Store, dict template.Dictionary) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req scenarioRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			respondInvalidRequest(w, "name is required")
			return
		}
		if msg := validateScenarioExpressions(req); msg != "" {
			respondInvalidRequest(w, msg)
			return
		}
		if req.PeerID == "" {
			respondInvalidRequest(w, "peerId is required")
			return
		}
		if req.SubscriberID == "" {
			respondInvalidRequest(w, "subscriberId is required")
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
		respondJSON(w, http.StatusCreated, toFullResponse(sc, dict))
	}
}

// getScenario handles GET /scenarios/{id}.
func getScenario(s store.Store, dict template.Dictionary) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDParam(w, r, "id")
		if !ok {
			return
		}
		sc, err := s.GetScenario(r.Context(), id)
		if mapStoreError(w, err) != nil {
			return
		}
		respondJSON(w, http.StatusOK, toFullResponse(sc, dict))
	}
}

// updateScenario handles PUT /scenarios/{id}.
func updateScenario(s store.Store, dict template.Dictionary) http.HandlerFunc {
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
		if msg := validateScenarioExpressions(req); msg != "" {
			respondInvalidRequest(w, msg)
			return
		}
		if req.PeerID == "" {
			respondInvalidRequest(w, "peerId is required")
			return
		}
		if req.SubscriberID == "" {
			respondInvalidRequest(w, "subscriberId is required")
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
		respondJSON(w, http.StatusOK, toFullResponse(sc, dict))
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
