package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
			mcp.WithObject("body",
				mcp.Required(),
				mcp.Description("Scenario body (steps, variables, peerName, subscriberName). See OpenAPI spec for shape."),
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
			mcp.WithObject("body",
				mcp.Required(),
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

// handleListScenarios returns all scenarios.
func (srv *Server) handleListScenarios(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetScenario returns a single scenario by UUID.
func (srv *Server) handleGetScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleCreateScenario creates a new scenario.
func (srv *Server) handleCreateScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleUpdateScenario updates an existing scenario.
func (srv *Server) handleUpdateScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleDeleteScenario deletes a scenario by UUID.
func (srv *Server) handleDeleteScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleDuplicateScenario duplicates a scenario.
func (srv *Server) handleDuplicateScenario(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}
