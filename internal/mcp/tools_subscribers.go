package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerSubscriberTools registers all 5 subscriber management tools.
func registerSubscriberTools(s *server.MCPServer, srv *Server) {
	// list_subscribers — read-only
	s.AddTool(
		mcp.NewTool("list_subscribers",
			mcp.WithDescription("List all configured test subscribers."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListSubscribers,
	)

	// get_subscriber — read-only
	s.AddTool(
		mcp.NewTool("get_subscriber",
			mcp.WithDescription("Get a single test subscriber by ID."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the subscriber to retrieve."),
			),
		),
		srv.handleGetSubscriber,
	)

	// create_subscriber — write, non-destructive
	s.AddTool(
		mcp.NewTool("create_subscriber",
			mcp.WithDescription("Create a new test subscriber."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Display name for the subscriber."),
			),
			mcp.WithObject("config",
				mcp.Required(),
				mcp.Description("Subscriber configuration object (MSISDN, IMSI, balance, etc.)."),
			),
		),
		srv.handleCreateSubscriber,
	)

	// update_subscriber — write, non-destructive
	s.AddTool(
		mcp.NewTool("update_subscriber",
			mcp.WithDescription("Update an existing test subscriber."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the subscriber to update."),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Updated display name."),
			),
			mcp.WithObject("config",
				mcp.Required(),
				mcp.Description("Updated subscriber configuration object."),
			),
		),
		srv.handleUpdateSubscriber,
	)

	// delete_subscriber — destructive
	s.AddTool(
		mcp.NewTool("delete_subscriber",
			mcp.WithDescription("Delete a test subscriber by ID."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the subscriber to delete."),
			),
		),
		srv.handleDeleteSubscriber,
	)
}

// handleListSubscribers returns all subscribers.
func (srv *Server) handleListSubscribers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetSubscriber returns a single subscriber by UUID.
func (srv *Server) handleGetSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleCreateSubscriber creates a new subscriber.
func (srv *Server) handleCreateSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleUpdateSubscriber updates an existing subscriber.
func (srv *Server) handleUpdateSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleDeleteSubscriber deletes a subscriber by UUID.
func (srv *Server) handleDeleteSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}
