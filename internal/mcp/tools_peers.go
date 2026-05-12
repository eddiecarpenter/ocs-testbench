package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerPeerTools registers all 7 peer management tools on the MCP server.
func registerPeerTools(s *server.MCPServer, srv *Server) {
	// list_peers — read-only
	s.AddTool(
		mcp.NewTool("list_peers",
			mcp.WithDescription("List all configured Diameter peers."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListPeers,
	)

	// get_peer — read-only
	s.AddTool(
		mcp.NewTool("get_peer",
			mcp.WithDescription("Get a single Diameter peer by ID."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the peer to retrieve."),
			),
		),
		srv.handleGetPeer,
	)

	// create_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("create_peer",
			mcp.WithDescription("Create a new Diameter peer configuration."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Unique name for the peer."),
			),
			mcp.WithObject("config",
				mcp.Required(),
				mcp.Description("Peer configuration object (host, port, originHost, originRealm, etc.)."),
			),
		),
		srv.handleCreatePeer,
	)

	// update_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("update_peer",
			mcp.WithDescription("Update an existing Diameter peer configuration."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the peer to update."),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("New unique name for the peer."),
			),
			mcp.WithObject("config",
				mcp.Required(),
				mcp.Description("Updated peer configuration object."),
			),
		),
		srv.handleUpdatePeer,
	)

	// delete_peer — destructive
	s.AddTool(
		mcp.NewTool("delete_peer",
			mcp.WithDescription("Delete a Diameter peer configuration. Fails if the peer is referenced by any scenario."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the peer to delete."),
			),
		),
		srv.handleDeletePeer,
	)

	// connect_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("connect_peer",
			mcp.WithDescription("Initiate the Diameter connection lifecycle for the named peer."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the peer to connect."),
			),
		),
		srv.handleConnectPeer,
	)

	// disconnect_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("disconnect_peer",
			mcp.WithDescription("Gracefully close the Diameter connection for the named peer."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the peer to disconnect."),
			),
		),
		srv.handleDisconnectPeer,
	)
}

// handleListPeers returns all configured peers with their live connection state.
func (srv *Server) handleListPeers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetPeer returns a single peer by UUID.
func (srv *Server) handleGetPeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleCreatePeer creates a new peer configuration.
func (srv *Server) handleCreatePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleUpdatePeer updates an existing peer configuration.
func (srv *Server) handleUpdatePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleDeletePeer deletes a peer configuration.
func (srv *Server) handleDeletePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleConnectPeer initiates connection for a named peer.
func (srv *Server) handleConnectPeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleDisconnectPeer gracefully closes a named peer connection.
func (srv *Server) handleDisconnectPeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}
