package ai_test

import (
	"context"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newMCPServerWithTools creates a real mcp-go HTTP handler with the given
// tools and a common call handler. Used by agent tests that need a real
// MCP server to exercise the production connectivity path.
func newMCPServerWithTools(
	tools []mcp.Tool,
	handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
) http.Handler {
	mcpServer := server.NewMCPServer("test-server", "1.0.0")
	for _, t := range tools {
		// Capture the tool in the closure.
		tool := t
		mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handler(ctx, req)
		})
	}
	return server.NewStreamableHTTPServer(mcpServer)
}
