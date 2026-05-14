package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newEmptyMCPServer spins up a real mcp-go HTTP server with no tools.
// Used by simple chat handler tests that need a working MCP transport
// but do not exercise any actual tool calls.
func newEmptyMCPServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	mcpServer := server.NewMCPServer("test-empty-server", "1.0.0")
	srv := httptest.NewServer(http.StripPrefix("/mcp", server.NewStreamableHTTPServer(mcpServer)))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// newEndToEndMCPServer spins up a real mcp-go HTTP server with a minimal
// set of tools used by the end-to-end chat handler tests. It returns the
// httptest.Server and the base URL (without the /mcp suffix — the ai.Agent
// constructor appends /mcp automatically).
func newEndToEndMCPServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	ro := true
	de := false
	listPeersTool := mcp.Tool{
		Name:        "list_peers",
		Description: "List all registered Diameter peers and their connection state",
		Annotations: mcp.ToolAnnotation{ReadOnlyHint: &ro, DestructiveHint: &de},
	}

	mcpServer := server.NewMCPServer("test-e2e-server", "1.0.0")
	mcpServer.AddTool(listPeersTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: `[{"name":"ocs-01","state":"connected"}]`},
			},
		}, nil
	})

	srv := httptest.NewServer(http.StripPrefix("/mcp", server.NewStreamableHTTPServer(mcpServer)))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// newEndToEndMCPServerWithError spins up a real mcp-go HTTP server whose
// tool handler always returns an error. Used to verify that the agent
// surfaces MCP errors as error-result tool_call SSE events and continues.
func newEndToEndMCPServerWithError(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	ro := true
	de := false
	listPeersTool := mcp.Tool{
		Name:        "list_peers",
		Description: "List all registered Diameter peers and their connection state",
		Annotations: mcp.ToolAnnotation{ReadOnlyHint: &ro, DestructiveHint: &de},
	}

	mcpServer := server.NewMCPServer("test-e2e-error-server", "1.0.0")
	mcpServer.AddTool(listPeersTool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("MCP tool execution failed")
	})

	srv := httptest.NewServer(http.StripPrefix("/mcp", server.NewStreamableHTTPServer(mcpServer)))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}
