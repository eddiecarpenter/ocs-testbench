package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerAVPTools registers the 2 AVP dictionary tools.
func registerAVPTools(s *server.MCPServer, srv *Server) {
	// list_avps — read-only
	s.AddTool(
		mcp.NewTool("list_avps",
			mcp.WithDescription("List all known AVPs from the loaded Diameter dictionary (name, code, vendor-id)."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListAVPs,
	)

	// get_avp_info — read-only
	s.AddTool(
		mcp.NewTool("get_avp_info",
			mcp.WithDescription("Get detailed information about a specific AVP: name, code, vendor ID, and Diameter data type."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("AVP name to look up (e.g. 'Origin-Host', 'Service-Identifier')."),
			),
		),
		srv.handleGetAVPInfo,
	)
}

// handleListAVPs returns all known AVPs from the dictionary.
func (srv *Server) handleListAVPs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetAVPInfo returns metadata for a named AVP.
func (srv *Server) handleGetAVPInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}
