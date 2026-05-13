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

// registerScenarioTools registers all 6 scenario management tools.
func registerScenarioTools(s *server.MCPServer, srv *Server) {
	// list_scenarios — read-only
	s.AddTool(
		mcp.NewTool("list_scenarios",
			mcp.WithDescription("List all scenarios, including system starter scenarios. "+
				"System starters (origin=system) serve as templates — use duplicate_scenario to create your own editable copy."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListScenarios,
	)

	// get_scenario — read-only
	s.AddTool(
		mcp.NewTool("get_scenario",
			mcp.WithDescription("Get a single scenario by ID, including its full AVP step definitions."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the scenario to retrieve."),
			),
		),
		srv.handleGetScenario,
	)

	// create_scenario — write, non-destructive
	s.AddTool(
		mcp.NewTool("create_scenario",
			mcp.WithDescription("Create a new scenario. For most use-cases, prefer duplicate_scenario to start from a system starter."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name for the new scenario."),
			),
			mcp.WithString("peer_id",
				mcp.Required(),
				mcp.Description("UUID of the peer to associate with this scenario."),
			),
			mcp.WithString("subscriber_id",
				mcp.Required(),
				mcp.Description("UUID of the subscriber to associate with this scenario."),
			),
			mcp.WithObject("body",
				mcp.Description("Scenario body (steps, variables, sessionMode, serviceModel, etc.). See OpenAPI spec for shape. Optional fields default to empty."),
			),
		),
		srv.handleCreateScenario,
	)

	// update_scenario — write, non-destructive
	s.AddTool(
		mcp.NewTool("update_scenario",
			mcp.WithDescription("Update an existing user scenario. Cannot update system starter scenarios."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the scenario to update."),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Updated scenario name."),
			),
			mcp.WithString("peer_id",
				mcp.Required(),
				mcp.Description("UUID of the peer to associate."),
			),
			mcp.WithString("subscriber_id",
				mcp.Required(),
				mcp.Description("UUID of the subscriber to associate."),
			),
			mcp.WithObject("body",
				mcp.Description("Updated scenario body."),
			),
		),
		srv.handleUpdateScenario,
	)

	// delete_scenario — destructive
	s.AddTool(
		mcp.NewTool("delete_scenario",
			mcp.WithDescription("Delete a scenario by ID. Returns a tool error when the scenario has active executions."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the scenario to delete."),
			),
		),
		srv.handleDeleteScenario,
	)

	// duplicate_scenario — write, non-destructive
	s.AddTool(
		mcp.NewTool("duplicate_scenario",
			mcp.WithDescription("Create a copy of an existing scenario as a new user-editable scenario. "+
				"Recommended starting point: duplicate a system starter scenario (origin=system) to create your own "+
				"editable copy. System starters cannot be modified directly."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("source_id",
				mcp.Required(),
				mcp.Description("UUID of the scenario to duplicate (source)."),
			),
			mcp.WithString("new_name",
				mcp.Required(),
				mcp.Description("Name for the new duplicate scenario."),
			),
		),
		srv.handleDuplicateScenario,
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

// bodyFromMap converts a map[string]any (from MCP tool args) into the JSONB
// bytes to persist in the store.
func bodyFromMap(m map[string]any) ([]byte, error) {
	if m == nil {
		// Empty body: marshal an empty scenario body with sane defaults.
		return json.Marshal(scenarioInternalBody{
			SessionMode:  "session",
			ServiceModel: "gy",
		})
	}
	return json.Marshal(m)
}

// — handlers —

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
