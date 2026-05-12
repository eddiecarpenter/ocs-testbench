package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerExecutionTools registers all 9 execution control tools.
func registerExecutionTools(s *server.MCPServer, srv *Server) {
	// start_execution — write, non-destructive
	s.AddTool(
		mcp.NewTool("start_execution",
			mcp.WithDescription("Start a new execution session for the given scenario. "+
				"Returns the session ID and initial execution state."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("scenario_id",
				mcp.Required(),
				mcp.Description("UUID of the scenario to execute."),
			),
			mcp.WithString("mode",
				mcp.Required(),
				mcp.Description("Execution mode: 'interactive' (pause after each step) or 'continuous' (run through automatically)."),
			),
			mcp.WithNumber("repeats",
				mcp.Description("Number of full passes in continuous mode (0 = unlimited). Defaults to 1."),
			),
		),
		srv.handleStartExecution,
	)

	// list_executions — read-only
	s.AddTool(
		mcp.NewTool("list_executions",
			mcp.WithDescription("List all execution sessions (active, completed, terminated, error)."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListExecutions,
	)

	// get_execution — read-only
	s.AddTool(
		mcp.NewTool("get_execution",
			mcp.WithDescription("Get the current state and summary of an execution session."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the execution to retrieve."),
			),
		),
		srv.handleGetExecution,
	)

	// stop_execution — destructive
	s.AddTool(
		mcp.NewTool("stop_execution",
			mcp.WithDescription("Request the execution session to stop after the current step completes."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the execution to stop."),
			),
		),
		srv.handleStopExecution,
	)

	// resume_execution — write, non-destructive
	s.AddTool(
		mcp.NewTool("resume_execution",
			mcp.WithDescription("Resume a paused continuous execution session from its current step, running to completion."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the execution to resume."),
			),
		),
		srv.handleResumeExecution,
	)

	// step_execution — write, non-destructive
	s.AddTool(
		mcp.NewTool("step_execution",
			mcp.WithDescription("Execute exactly one step in a paused interactive execution session."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the interactive execution."),
			),
			mcp.WithObject("overrides",
				mcp.Description("Optional map of variable names to values that override the session context for this step only."),
			),
		),
		srv.handleStepExecution,
	)

	// get_execution_detail — read-only
	s.AddTool(
		mcp.NewTool("get_execution_detail",
			mcp.WithDescription("Get the full execution detail including per-step CCR/CCA AVP content and execution context snapshot."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the execution."),
			),
		),
		srv.handleGetExecutionDetail,
	)

	// apply_execution_context_override — write, non-destructive
	s.AddTool(
		mcp.NewTool("apply_execution_context_override",
			mcp.WithDescription("Write variables into the execution's live context. "+
				"Changes are permanent for the remainder of the run. Session must be paused."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the paused execution."),
			),
			mcp.WithObject("variables",
				mcp.Required(),
				mcp.Description("Map of variable names to values to write into the live context."),
			),
		),
		srv.handleApplyContextOverride,
	)

	// apply_execution_payload_override — write, non-destructive
	s.AddTool(
		mcp.NewTool("apply_execution_payload_override",
			mcp.WithDescription("Stage one-shot variables for the next step only. "+
				"Overrides revert after the next step or run-to-end send. Session must be paused."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the paused execution."),
			),
			mcp.WithObject("variables",
				mcp.Required(),
				mcp.Description("Map of variable names to values for one-shot override of the next step."),
			),
		),
		srv.handleApplyPayloadOverride,
	)
}

// handleStartExecution starts a new execution session.
func (srv *Server) handleStartExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleListExecutions lists all execution sessions.
func (srv *Server) handleListExecutions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetExecution gets execution state.
func (srv *Server) handleGetExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleStopExecution stops an execution session.
func (srv *Server) handleStopExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleResumeExecution resumes a paused continuous session.
func (srv *Server) handleResumeExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleStepExecution executes one step in an interactive session.
func (srv *Server) handleStepExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleGetExecutionDetail returns full execution detail with AVP trees.
func (srv *Server) handleGetExecutionDetail(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleApplyContextOverride writes variables into the live execution context.
func (srv *Server) handleApplyContextOverride(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

// handleApplyPayloadOverride stages one-shot variables for the next step.
func (srv *Server) handleApplyPayloadOverride(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}
