package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerSystemTools registers the 3 system tools.
func registerSystemTools(s *server.MCPServer, srv *Server) {
	// get_health — read-only
	s.AddTool(
		mcp.NewTool("get_health",
			mcp.WithDescription("Get the health status of the testbench: store reachability and per-peer connection states."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleGetHealth,
	)

	// get_config — read-only
	s.AddTool(
		mcp.NewTool("get_config",
			mcp.WithDescription("Get the current runtime-editable configuration fields "+
				"(server address, metrics address, logging format, headless mode)."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleGetConfig,
	)

	// update_config — write, non-destructive
	s.AddTool(
		mcp.NewTool("update_config",
			mcp.WithDescription("Update runtime-editable configuration fields in-memory. "+
				"WARNING: Changes apply only to the running process and are not persisted across restarts. "+
				"Excluded fields: database_url, peers (not runtime-editable)."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("logging_format",
				mcp.Description("Logging format: 'text' or 'json'. Optional."),
			),
			mcp.WithBoolean("headless",
				mcp.Description("Headless mode toggle. Optional."),
			),
		),
		srv.handleUpdateConfig,
	)
}

// handleGetHealth returns store reachability and per-peer connection states.
func (srv *Server) handleGetHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetConfig returns runtime-editable configuration fields.
func (srv *Server) handleGetConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleUpdateConfig updates in-memory configuration fields.
func (srv *Server) handleUpdateConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}
