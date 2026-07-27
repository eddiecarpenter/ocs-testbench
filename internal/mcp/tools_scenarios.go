package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// registerScenarioTools registers the 2 scenario tools on the MCP server.
//
//	list_scenarios — list all scenarios (readonly)
//	scenario       — CRUD + duplicate: get · create · update · delete · duplicate
func registerScenarioTools(s *server.MCPServer, srv *Server) {
	// list_scenarios — readonly, no parameters
	s.AddTool(
		mcp.NewTool("list_scenarios",
			mcp.WithDescription("List all scenarios including system starter templates (origin=system)."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListScenarios,
	)

	// scenario — CRUD + duplicate
	s.AddTool(
		mcp.NewTool("scenario",
			mcp.WithDescription(`Scenario management.
op=get        id*                                          → full scenario with AVP steps
op=create     name*, peer_id*, subscriber_id*, body?       → new scenario (prefer duplicate over create)
op=update     id*, name*, peer_id*, subscriber_id*, body?  → update user scenario (not system starters)
op=delete     id*                                          → remove (fails if active executions)
op=duplicate  source_id*, new_name*                        → copy a scenario; use to start from a system starter`),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("op",
				mcp.Required(),
				mcp.Description("get | create | update | delete | duplicate"),
			),
			mcp.WithString("id",
				mcp.Description("Scenario UUID — required for get, update, delete."),
			),
			mcp.WithString("source_id",
				mcp.Description("Source scenario UUID — required for duplicate."),
			),
			mcp.WithString("new_name",
				mcp.Description("Name for the duplicated scenario — required for duplicate."),
			),
			mcp.WithString("name",
				mcp.Description("Scenario name — required for create and update."),
			),
			mcp.WithString("peer_id",
				mcp.Description("Peer UUID — required for create and update."),
			),
			mcp.WithString("subscriber_id",
				mcp.Description("Subscriber UUID — required for create and update."),
			),
			mcp.WithObject("body",
				mcp.Description(`Scenario body fields (all optional unless noted):
  serviceType     — service classification (determines unit of measure and Service-Information AVP):
                    VOICE        time-based voice call (IMS/IN-Information)
                    DATA         volume-based data session (PS-Information)
                    SMS          event-based SMS (SMS-Information)
                    USSD1        UNIT-based USSD — charges by number of messages sent (DCD-Information); almost always sessionMode=event
                    USSD2        TIME-based USSD — charges by session duration in seconds (DCD-Information); sessionMode=session or event
  serviceProfile  — OCS vendor profile: 3GPP | HUAWEI
  sessionMode     — event (single CCR-E) | session (CCR-I + CCR-U* + CCR-T)
  serviceModel    — root (single MSCC) | multi-mscc (multiple rating groups)
  serviceContextId — Service-Context-Id AVP value sent in CCR messages
  description     — human-readable description
  favourite       — boolean; pin to top of list
  avpTree         — array of AVP nodes to include in each CCR step.
                    RULES (violations are rejected at save time):
                    • Every LEAF node (no children) MUST have a non-empty "valueRef".
                      An empty valueRef causes ENCODING_TYPE_MISMATCH at runtime.
                    • valueRef semantics:
                        UPPER_SNAKE_CASE  → variable reference (must exist in variables[]
                                           or be a system variable — see below)
                        any other string  → literal value  (e.g. "0", "-3", "true")
                    • Group/container AVPs have a non-empty "children" array and do
                      NOT need a valueRef themselves.
                    • System variables always available (no declaration needed):
                        SUB_ID_TYPE, MSISDN, SESSION_ID, CHARGING_ID,
                        CC_REQUEST_NUMBER, ORIGIN_HOST, ORIGIN_REALM, DEST_REALM,
                        SERVICE_CONTEXT_ID, AUTH_APP_ID, TOTAL_USU, RESULT_CODE,
                        FUI_ACTION, RG_<n>_GRANTED_UNITS, RG_<n>_USED_UNITS,
                        RG_<n>_QUOTA_THRESHOLD (where <n> is the rating group index)
  services        — array of service/MSCC definitions (for multi-mscc model)
  variables       — array of variable declarations (name, source, description)
  steps           — array of scenario steps`),
			),
		),
		srv.handleScenario,
	)
}

// — scenario response shape —

// scenarioSummaryJSON is the MCP response shape for a scenario summary.
type scenarioSummaryJSON struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	ServiceType      string `json:"serviceType,omitempty"`
	ServiceProfile   string `json:"serviceProfile,omitempty"`
	SessionMode      string `json:"sessionMode"`
	ServiceModel     string `json:"serviceModel"`
	Origin           string `json:"origin"`
	Favourite        bool   `json:"favourite"`
	SubscriberID     string `json:"subscriberId,omitempty"`
	PeerID           string `json:"peerId,omitempty"`
	ServiceContextID string `json:"serviceContextId,omitempty"`
	StepCount        int    `json:"stepCount"`
}

// scenarioFullJSON extends the summary with the JSONB body fields.
type scenarioFullJSON struct {
	scenarioSummaryJSON
	AvpTree   json.RawMessage `json:"avpTree"`
	Services  json.RawMessage `json:"services"`
	Variables json.RawMessage `json:"variables"`
	Steps     json.RawMessage `json:"steps"`
}

// scenarioInternalBody mirrors the body JSONB shape stored in the database.
type scenarioInternalBody struct {
	Description      string          `json:"description,omitempty"`
	SessionMode      string          `json:"sessionMode"`
	ServiceModel     string          `json:"serviceModel"`
	ServiceType      string          `json:"serviceType,omitempty"`
	ServiceProfile   string          `json:"serviceProfile,omitempty"`
	Favourite        bool            `json:"favourite,omitempty"`
	ServiceContextID string          `json:"serviceContextId,omitempty"`
	AvpTree          json.RawMessage `json:"avpTree"`
	Services         json.RawMessage `json:"services"`
	Variables        json.RawMessage `json:"variables"`
	Steps            json.RawMessage `json:"steps"`
}

// toScenarioFullJSON converts a store.Scenario to the full response shape.
func toScenarioFullJSON(sc store.Scenario) scenarioFullJSON {
	var b scenarioInternalBody
	_ = json.Unmarshal(sc.Body, &b)

	steps := b.Steps
	if len(steps) == 0 {
		steps = json.RawMessage("null")
	}
	stepCount := countElements(b.Steps)

	return scenarioFullJSON{
		scenarioSummaryJSON: scenarioSummaryJSON{
			ID:               uuidStr(sc.ID),
			Name:             sc.Name,
			Description:      b.Description,
			ServiceType:      b.ServiceType,
			ServiceProfile:   b.ServiceProfile,
			SessionMode:      b.SessionMode,
			ServiceModel:     b.ServiceModel,
			Origin:           "user",
			Favourite:        b.Favourite,
			SubscriberID:     uuidStr(sc.SubscriberID),
			PeerID:           uuidStr(sc.PeerID),
			ServiceContextID: b.ServiceContextID,
			StepCount:        stepCount,
		},
		AvpTree:   nullableRaw(b.AvpTree),
		Services:  nullableRaw(b.Services),
		Variables: nullableRaw(b.Variables),
		Steps:     steps,
	}
}

// toScenarioSummaryJSON converts a store.Scenario to the summary response.
func toScenarioSummaryJSON(sc store.Scenario) scenarioSummaryJSON {
	return toScenarioFullJSON(sc).scenarioSummaryJSON
}

// countElements returns the number of elements in a JSON array.
func countElements(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return 0
	}
	return len(elems)
}

// nullableRaw returns json.RawMessage("null") when raw is empty.
func nullableRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

// validScenarioServiceTypes is the closed set of accepted serviceType values.
// Keep in sync with the OpenAPI ServiceType enum and the API-layer validServiceTypes map.
// validScenarioServiceTypes is the closed set of accepted serviceType values.
// Keep in sync with the OpenAPI ServiceType enum and the API-layer validServiceTypes map.
// USSD1 = unit-based (message count), typically event mode.
// USSD2 = time-based (seconds), session or event — sessionMode is the orthogonal field.
var validScenarioServiceTypes = map[string]bool{
	"VOICE": true,
	"DATA":  true,
	"SMS":   true,
	"USSD1": true,
	"USSD2": true,
}

// validateScenarioServiceType returns an error string when the supplied value
// is not a recognised ServiceType. An empty string is accepted (field optional).
func validateScenarioServiceType(s string) string {
	if s == "" {
		return ""
	}
	if !validScenarioServiceTypes[s] {
		return fmt.Sprintf("serviceType %q is not valid — must be one of: VOICE, DATA, SMS, USSD1, USSD2", s)
	}
	return ""
}

// bodyFromMap converts a map[string]any (from MCP tool args) into the JSONB
// bytes to persist in the store.
//
// Normalisation applied before marshalling:
//   - Array fields (avpTree, services, variables, steps) that are nil or JSON
//     null are replaced with an empty slice so the frontend never sees null
//     where it expects an array.
//   - variable entries that have a non-map source or missing required fields
//     are rejected (returning an error string via validateBodyVariables).
func bodyFromMap(m map[string]any) ([]byte, error) {
	if m == nil {
		// Empty body: marshal an empty scenario body with sane defaults.
		return json.Marshal(scenarioInternalBody{
			SessionMode:  "session",
			ServiceModel: "gy",
		})
	}
	// Normalise array fields: nil → empty slice, so the frontend never gets null.
	for _, field := range []string{"avpTree", "services", "variables", "steps"} {
		if v, exists := m[field]; !exists || v == nil {
			m[field] = []any{}
		}
	}
	// Validate variables structure if present.
	if vars, ok := m["variables"].([]any); ok {
		if msg := validateBodyVariables(vars); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
	}
	// Validate avpTree: every leaf node must have a non-empty valueRef.
	if tree, ok := m["avpTree"].([]any); ok && len(tree) > 0 {
		if msg := walkAvpNodes(tree, "avpTree"); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
	}
	return json.Marshal(m)
}

// validateBodyVariables checks that every variable entry in the body has a
// valid structure. Returns a human-readable error string or "" if all are valid.
func validateBodyVariables(vars []any) string {
	for i, v := range vars {
		entry, ok := v.(map[string]any)
		if !ok {
			return fmt.Sprintf("variables[%d]: must be an object", i)
		}
		name, _ := entry["name"].(string)
		if name == "" {
			return fmt.Sprintf("variables[%d]: name is required and must be a non-empty string", i)
		}
		src, ok := entry["source"].(map[string]any)
		if !ok || src == nil {
			return fmt.Sprintf("variables[%d] (%q): source is required and must be an object", i, name)
		}
		kind, _ := src["kind"].(string)
		switch kind {
		case "generator":
			if _, ok := src["strategy"].(string); !ok {
				return fmt.Sprintf("variables[%d] (%q): source.strategy is required for kind=generator", i, name)
			}
			if _, ok := src["refresh"].(string); !ok {
				return fmt.Sprintf("variables[%d] (%q): source.refresh is required for kind=generator (once | per-send)", i, name)
			}
		case "bound":
			if _, ok := src["from"].(string); !ok {
				return fmt.Sprintf("variables[%d] (%q): source.from is required for kind=bound (subscriber | peer | config | step)", i, name)
			}
			if _, ok := src["field"].(string); !ok {
				return fmt.Sprintf("variables[%d] (%q): source.field is required for kind=bound", i, name)
			}
		case "extracted":
			if _, ok := src["path"].(string); !ok {
				return fmt.Sprintf("variables[%d] (%q): source.path is required for kind=extracted", i, name)
			}
		default:
			return fmt.Sprintf("variables[%d] (%q): source.kind %q is invalid — must be generator | bound | extracted", i, name, kind)
		}
	}
	return ""
}

// walkAvpNodes recursively checks that every leaf AVP node in the tree has a
// non-empty valueRef. A leaf is a node with no children. Returns the first
// problem found, or "" when the tree is valid.
//
// valueRef semantics (mirrors smConvertAvpNode in session_manager.go):
//   - UPPER_SNAKE_CASE  → treated as a variable reference (must be declared or a
//     well-known system variable such as MSISDN, SESSION_ID, CC_REQUEST_NUMBER…)
//   - any other non-empty string (e.g. "0", "-3", "true") → passed as a literal
//
// An empty valueRef always causes ENCODING_TYPE_MISMATCH at execution time.
func walkAvpNodes(nodes []any, path string) string {
	for i, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := node["name"].(string)
		label := fmt.Sprintf("%s[%d]", path, i)
		if name != "" {
			label = fmt.Sprintf("%s[%d] (%q)", path, i, name)
		}
		// A node is grouped iff it carries a "children" key (an array, even if
		// empty) — mirroring the frontend's Array.isArray(node.children) and the
		// OpenAPI schema. Empty grouped AVPs are legal in Diameter and encode
		// fine, so they need no valueRef. Only a true leaf (no children key at
		// all) must carry a value.
		childrenRaw, hasChildren := node["children"]
		if hasChildren {
			children, _ := childrenRaw.([]any)
			if msg := walkAvpNodes(children, label+".children"); msg != "" {
				return msg
			}
		} else {
			valueRef, _ := node["valueRef"].(string)
			if valueRef == "" {
				return fmt.Sprintf(
					`%s: leaf AVP node has no valueRef — every leaf must reference a variable or carry a literal value (e.g. "0")`,
					label,
				)
			}
		}
	}
	return ""
}

// — handlers —

// handleScenario dispatches CRUD + duplicate operations for the consolidated "scenario" tool.
func (srv *Server) handleScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	op, err := req.RequireString("op")
	if err != nil {
		return mcp.NewToolResultError("op is required (get | create | update | delete | duplicate)"), nil
	}
	switch op {
	case "get":
		return srv.handleGetScenario(ctx, req)
	case "create":
		return srv.handleCreateScenario(ctx, req)
	case "update":
		return srv.handleUpdateScenario(ctx, req)
	case "delete":
		return srv.handleDeleteScenario(ctx, req)
	case "duplicate":
		return srv.handleDuplicateScenario(ctx, req)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown op %q — use get | create | update | delete | duplicate", op)), nil
	}
}

// handleListScenarios returns all scenarios.
func (srv *Server) handleListScenarios(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scenarios, err := srv.store.ListScenarios(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list scenarios failed: %v", err)), nil
	}
	out := make([]scenarioSummaryJSON, len(scenarios))
	for i, sc := range scenarios {
		out[i] = toScenarioSummaryJSON(sc)
	}
	return mcp.NewToolResultJSON(map[string]any{"scenarios": out})
}

// handleGetScenario returns a single scenario by UUID.
func (srv *Server) handleGetScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid scenario id %q: %v", idStr, err)), nil
	}
	sc, err := srv.store.GetScenario(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario %q not found", idStr)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get scenario failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toScenarioFullJSON(sc))
}

// handleCreateScenario creates a new scenario.
func (srv *Server) handleCreateScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	peerIDStr, err := req.RequireString("peer_id")
	if err != nil {
		return mcp.NewToolResultError("peer_id is required"), nil
	}
	peerID, err := parseUUID(peerIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid peer_id %q: %v", peerIDStr, err)), nil
	}
	subIDStr, err := req.RequireString("subscriber_id")
	if err != nil {
		return mcp.NewToolResultError("subscriber_id is required"), nil
	}
	subID, err := parseUUID(subIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid subscriber_id %q: %v", subIDStr, err)), nil
	}

	args := req.GetArguments()
	var bodyMap map[string]any
	if b, ok := args["body"].(map[string]any); ok {
		bodyMap = b
	}
	if bodyMap != nil {
		if st, _ := bodyMap["serviceType"].(string); st != "" {
			if msg := validateScenarioServiceType(st); msg != "" {
				return mcp.NewToolResultError(msg), nil
			}
		}
	}
	body, err := bodyFromMap(bodyMap)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid body: %v", err)), nil
	}

	sc, err := srv.store.InsertScenario(ctx, name, peerID, subID, body)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario name %q is already taken", name)), nil
		}
		if errors.Is(err, store.ErrForeignKey) {
			return mcp.NewToolResultError("peer_id or subscriber_id does not exist"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("create scenario failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toScenarioFullJSON(sc))
}

// handleUpdateScenario updates an existing scenario.
func (srv *Server) handleUpdateScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid scenario id %q: %v", idStr, err)), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	peerIDStr, err := req.RequireString("peer_id")
	if err != nil {
		return mcp.NewToolResultError("peer_id is required"), nil
	}
	peerID, err := parseUUID(peerIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid peer_id %q: %v", peerIDStr, err)), nil
	}
	subIDStr, err := req.RequireString("subscriber_id")
	if err != nil {
		return mcp.NewToolResultError("subscriber_id is required"), nil
	}
	subID, err := parseUUID(subIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid subscriber_id %q: %v", subIDStr, err)), nil
	}

	args := req.GetArguments()
	var bodyMap map[string]any
	if b, ok := args["body"].(map[string]any); ok {
		bodyMap = b
	}
	if bodyMap != nil {
		if st, _ := bodyMap["serviceType"].(string); st != "" {
			if msg := validateScenarioServiceType(st); msg != "" {
				return mcp.NewToolResultError(msg), nil
			}
		}
	}
	body, err := bodyFromMap(bodyMap)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid body: %v", err)), nil
	}

	sc, err := srv.store.UpdateScenario(ctx, store.UpdateScenarioParams{
		ID:           id,
		Name:         name,
		PeerID:       peerID,
		SubscriberID: subID,
		Body:         body,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario %q not found", idStr)), nil
		}
		if errors.Is(err, store.ErrDuplicateName) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario name %q is already taken", name)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("update scenario failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toScenarioFullJSON(sc))
}

// handleDeleteScenario deletes a scenario by UUID.
func (srv *Server) handleDeleteScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid scenario id %q: %v", idStr, err)), nil
	}
	if err := srv.store.DeleteScenario(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario %q not found", idStr)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("delete scenario failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "deleted"})
}

// handleDuplicateScenario creates a copy of an existing scenario.
func (srv *Server) handleDuplicateScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sourceIDStr, err := req.RequireString("source_id")
	if err != nil {
		return mcp.NewToolResultError("source_id is required"), nil
	}
	sourceID, err := parseUUID(sourceIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid source_id %q: %v", sourceIDStr, err)), nil
	}
	newName, err := req.RequireString("new_name")
	if err != nil {
		return mcp.NewToolResultError("new_name is required"), nil
	}

	// Fetch the source scenario.
	source, err := srv.store.GetScenario(ctx, sourceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("source scenario %q not found", sourceIDStr)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get source scenario failed: %v", err)), nil
	}

	// Insert the copy with the new name, preserving peer/subscriber/body.
	dup, err := srv.store.InsertScenario(ctx, newName, source.PeerID, source.SubscriberID, source.Body)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario name %q is already taken", newName)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("duplicate scenario failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toScenarioFullJSON(dup))
}
