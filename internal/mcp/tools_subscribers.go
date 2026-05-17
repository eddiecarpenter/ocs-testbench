package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// registerSubscriberTools registers the 2 subscriber tools on the MCP server.
//
//	list_subscribers — list all subscribers (readonly)
//	subscriber       — CRUD: get · create · update · delete
func registerSubscriberTools(s *server.MCPServer, srv *Server) {
	// list_subscribers — readonly, no parameters
	s.AddTool(
		mcp.NewTool("list_subscribers",
			mcp.WithDescription("List all configured test subscribers."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListSubscribers,
	)

	// subscriber — CRUD operations
	s.AddTool(
		mcp.NewTool("subscriber",
			mcp.WithDescription(`Test subscriber CRUD.
op=get     id*                              → single subscriber
op=create  msisdn*, iccid*, name?, imei?, tac?  → new subscriber
op=update  id*, msisdn*, iccid*, name?, imei?, tac?  → update subscriber
op=delete  id*                              → remove subscriber`),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("op",
				mcp.Required(),
				mcp.Description("get | create | update | delete"),
			),
			mcp.WithString("id",
				mcp.Description("Subscriber UUID — required for get, update, delete."),
			),
			mcp.WithString("msisdn",
				mcp.Description("MSISDN (phone number) — required for create and update."),
			),
			mcp.WithString("iccid",
				mcp.Description("ICCID (SIM card identifier) — required for create and update."),
			),
			mcp.WithString("name",
				mcp.Description("Display name. Optional."),
			),
			mcp.WithString("imei",
				mcp.Description("IMEI device identifier. Optional."),
			),
			mcp.WithString("tac",
				mcp.Description("TAC (Type Allocation Code). Optional."),
			),
		),
		srv.handleSubscriber,
	)
}

// — subscriber response shape —

// subscriberJSON is the MCP response shape for a Subscriber resource.
type subscriberJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Msisdn string `json:"msisdn"`
	Iccid  string `json:"iccid"`
	Imei   string `json:"imei,omitempty"`
	Tac    string `json:"tac,omitempty"`
}

// toSubscriberJSON converts a store.Subscriber to a subscriberJSON response.
func toSubscriberJSON(s store.Subscriber) subscriberJSON {
	return subscriberJSON{
		ID:     uuidStr(s.ID),
		Name:   s.Name,
		Msisdn: s.Msisdn,
		Iccid:  s.Iccid,
		Imei:   s.Imei.String,
		Tac:    s.Tac.String,
	}
}

// optionalPGText converts a string to a pgtype.Text, treating empty string as null.
func optionalPGText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// — handlers —

// handleSubscriber dispatches CRUD operations for the consolidated "subscriber" tool.
func (srv *Server) handleSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	op, err := req.RequireString("op")
	if err != nil {
		return mcp.NewToolResultError("op is required (get | create | update | delete)"), nil
	}
	switch op {
	case "get":
		return srv.handleGetSubscriber(ctx, req)
	case "create":
		return srv.handleCreateSubscriber(ctx, req)
	case "update":
		return srv.handleUpdateSubscriber(ctx, req)
	case "delete":
		return srv.handleDeleteSubscriber(ctx, req)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown op %q — use get | create | update | delete", op)), nil
	}
}

// handleListSubscribers returns all subscribers.
func (srv *Server) handleListSubscribers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subs, err := srv.store.ListSubscribers(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list subscribers failed: %v", err)), nil
	}
	out := make([]subscriberJSON, len(subs))
	for i, s := range subs {
		out[i] = toSubscriberJSON(s)
	}
	return mcp.NewToolResultJSON(map[string]any{"subscribers": out})
}

// handleGetSubscriber returns a single subscriber by UUID.
func (srv *Server) handleGetSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid subscriber id %q: %v", idStr, err)), nil
	}
	sub, err := srv.store.GetSubscriber(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("subscriber %q not found", idStr)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get subscriber failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toSubscriberJSON(sub))
}

// handleCreateSubscriber creates a new subscriber.
func (srv *Server) handleCreateSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	msisdn, err := req.RequireString("msisdn")
	if err != nil {
		return mcp.NewToolResultError("msisdn is required"), nil
	}
	iccid, err := req.RequireString("iccid")
	if err != nil {
		return mcp.NewToolResultError("iccid is required"), nil
	}
	name := req.GetString("name", "")
	imei := req.GetString("imei", "")
	tac := req.GetString("tac", "")

	sub, err := srv.store.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name:   name,
		Msisdn: msisdn,
		Iccid:  iccid,
		Imei:   optionalPGText(imei),
		Tac:    optionalPGText(tac),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create subscriber failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toSubscriberJSON(sub))
}

// handleUpdateSubscriber updates an existing subscriber.
func (srv *Server) handleUpdateSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid subscriber id %q: %v", idStr, err)), nil
	}
	msisdn, err := req.RequireString("msisdn")
	if err != nil {
		return mcp.NewToolResultError("msisdn is required"), nil
	}
	iccid, err := req.RequireString("iccid")
	if err != nil {
		return mcp.NewToolResultError("iccid is required"), nil
	}
	name := req.GetString("name", "")
	imei := req.GetString("imei", "")
	tac := req.GetString("tac", "")

	sub, err := srv.store.UpdateSubscriber(ctx, store.UpdateSubscriberParams{
		ID:     id,
		Name:   name,
		Msisdn: msisdn,
		Iccid:  iccid,
		Imei:   optionalPGText(imei),
		Tac:    optionalPGText(tac),
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("subscriber %q not found", idStr)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("update subscriber failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(toSubscriberJSON(sub))
}

// handleDeleteSubscriber deletes a subscriber by UUID.
func (srv *Server) handleDeleteSubscriber(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid subscriber id %q: %v", idStr, err)), nil
	}
	if err := srv.store.DeleteSubscriber(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("subscriber %q not found", idStr)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("delete subscriber failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "deleted"})
}
