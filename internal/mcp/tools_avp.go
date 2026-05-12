package mcp

import (
	"context"
	"fmt"

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

// avpInfoJSON is the MCP response shape for a single AVP.
type avpInfoJSON struct {
	Name     string `json:"name"`
	Code     uint32 `json:"code"`
	VendorID uint32 `json:"vendorId,omitempty"`
	DataType string `json:"dataType"`
}

// handleListAVPs returns all known AVPs from the Diameter dictionary.
// When the parser is nil (e.g. in tests), returns an empty list.
func (srv *Server) handleListAVPs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.parser == nil {
		return mcp.NewToolResultJSON([]avpInfoJSON{})
	}

	// Iterate all applications in the parser and collect unique AVPs by name.
	seen := make(map[string]struct{})
	var avps []avpInfoJSON

	for _, app := range srv.parser.Apps() {
		for _, avp := range app.AVP {
			if _, already := seen[avp.Name]; already {
				continue
			}
			seen[avp.Name] = struct{}{}
			avps = append(avps, avpInfoJSON{
				Name:     avp.Name,
				Code:     avp.Code,
				VendorID: avp.VendorID,
				DataType: avp.Data.TypeName,
			})
		}
	}

	return mcp.NewToolResultJSON(avps)
}

// handleGetAVPInfo returns metadata for a named AVP.
func (srv *Server) handleGetAVPInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	if srv.dict == nil {
		return mcp.NewToolResultError("AVP dictionary not available"), nil
	}

	meta, lookupErr := srv.dict.Lookup(name)
	if lookupErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("AVP %q not found in dictionary", name)), nil
	}

	return mcp.NewToolResultJSON(avpInfoJSON{
		Name:     name,
		Code:     meta.Code,
		VendorID: meta.VendorID,
		DataType: meta.DataType,
	})
}
