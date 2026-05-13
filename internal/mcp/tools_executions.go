package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
)

// registerExecutionTools registers all 11 execution control tools.
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

	// get_step_detail — read-only
	s.AddTool(
		mcp.NewTool("get_step_detail",
			mcp.WithDescription("Get the CCR/CCA AVP detail for a single step of an execution session. "+
				"Returns structured AVP lines for both the request and the answer, "+
				"plus result code, duration, and state."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the execution."),
			),
			mcp.WithNumber("step",
				mcp.Required(),
				mcp.Description("1-based step number to retrieve (e.g. 1 = first step, 2 = second step)."),
			),
		),
		srv.handleGetStepDetail,
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

	// wait_execution — read-only, blocking
	s.AddTool(
		mcp.NewTool("wait_execution",
			mcp.WithDescription("Block until an execution session reaches a terminal state (success, error, terminated) "+
				"or the timeout expires. Returns the final execution detail. "+
				"Use after start_execution in continuous mode to avoid polling."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID of the execution to wait for."),
			),
			mcp.WithNumber("timeout_seconds",
				mcp.Description("Maximum seconds to wait before returning (default 120, max 600)."),
			),
		),
		srv.handleWaitExecution,
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

// — execution response shapes —

// executionSummaryMCPJSON is the MCP response shape for an execution summary.
type executionSummaryMCPJSON struct {
	SessionID           string `json:"sessionId"`
	ScenarioID          string `json:"scenarioId"`
	ScenarioName        string `json:"scenarioName"`
	Mode                string `json:"mode"`
	State               string `json:"state"`
	StartedAt           string `json:"startedAt"`
	Repeats             int    `json:"repeats,omitempty"`
	CompletedIterations int    `json:"completedIterations,omitempty"`
}

// toExecutionSummaryMCP converts an api.ExecutionSummary to the MCP response shape.
func toExecutionSummaryMCP(s api.ExecutionSummary) executionSummaryMCPJSON {
	return executionSummaryMCPJSON{
		SessionID:           s.SessionID,
		ScenarioID:          s.ScenarioID,
		ScenarioName:        s.ScenarioName,
		Mode:                s.Mode,
		State:               s.State,
		StartedAt:           s.StartedAt,
		Repeats:             s.Repeats,
		CompletedIterations: s.CompletedIterations,
	}
}

// — handlers —

// handleStartExecution starts a new execution session.
func (srv *Server) handleStartExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	scenarioID, err := req.RequireString("scenario_id")
	if err != nil {
		return mcp.NewToolResultError("scenario_id is required"), nil
	}
	mode, err := req.RequireString("mode")
	if err != nil {
		return mcp.NewToolResultError("mode is required"), nil
	}
	if mode != "interactive" && mode != "continuous" {
		return mcp.NewToolResultError("mode must be 'interactive' or 'continuous'"), nil
	}
	repeats := int(req.GetFloat("repeats", 1))

	info, startErr := srv.execEngine.Start(ctx, scenarioID, mode, repeats)
	if startErr != nil {
		if errors.Is(startErr, api.ErrScenarioNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("scenario %q not found", scenarioID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("start execution failed: %v", startErr)), nil
	}

	detail, detailErr := srv.execEngine.Detail(ctx, info.SessionID)
	if detailErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get execution detail failed: %v", detailErr)), nil
	}
	return mcp.NewToolResultJSON(detail)
}

// handleListExecutions lists all execution sessions.
func (srv *Server) handleListExecutions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultJSON(map[string]any{"executions": []executionSummaryMCPJSON{}})
	}
	sessions := srv.execEngine.List(ctx)
	out := make([]executionSummaryMCPJSON, len(sessions))
	for i, s := range sessions {
		out[i] = toExecutionSummaryMCP(s)
	}
	return mcp.NewToolResultJSON(map[string]any{"executions": out})
}

// handleGetExecution gets the current state of an execution.
func (srv *Server) handleGetExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	detail, detailErr := srv.execEngine.Detail(ctx, sessionID)
	if detailErr != nil {
		if errors.Is(detailErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get execution failed: %v", detailErr)), nil
	}
	return mcp.NewToolResultJSON(detail)
}

// handleStopExecution stops an execution session.
func (srv *Server) handleStopExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	if stopErr := srv.execEngine.Stop(ctx, sessionID); stopErr != nil {
		if errors.Is(stopErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("stop execution failed: %v", stopErr)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "stopping", "sessionId": sessionID})
}

// handleResumeExecution resumes a paused continuous session.
func (srv *Server) handleResumeExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	if resumeErr := srv.execEngine.RunToEnd(ctx, sessionID); resumeErr != nil {
		if errors.Is(resumeErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		if errors.Is(resumeErr, api.ErrInvalidState) {
			return mcp.NewToolResultError("session is not a paused continuous session"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("resume execution failed: %v", resumeErr)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "resuming", "sessionId": sessionID})
}

// handleStepExecution executes one step in an interactive session.
func (srv *Server) handleStepExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}

	var overrides map[string]any
	args := req.GetArguments()
	if o, ok := args["overrides"].(map[string]any); ok {
		overrides = o
	}

	result, stepErr := srv.execEngine.Step(ctx, sessionID, overrides)
	if stepErr != nil {
		if errors.Is(stepErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		if errors.Is(stepErr, api.ErrInvalidState) {
			return mcp.NewToolResultError("session is not in a state that permits stepping"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("step failed: %v", stepErr)), nil
	}
	return mcp.NewToolResultJSON(result)
}

// handleGetExecutionDetail returns full execution detail with AVP trees.
func (srv *Server) handleGetExecutionDetail(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	detail, detailErr := srv.execEngine.Detail(ctx, sessionID)
	if detailErr != nil {
		if errors.Is(detailErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get execution detail failed: %v", detailErr)), nil
	}
	return mcp.NewToolResultJSON(detail)
}

// handleApplyContextOverride writes variables into the live execution context.
func (srv *Server) handleApplyContextOverride(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	args := req.GetArguments()
	variables, ok := args["variables"].(map[string]any)
	if !ok {
		return mcp.NewToolResultError("variables is required and must be an object"), nil
	}
	if applyErr := srv.execEngine.ApplyContextOverride(ctx, sessionID, variables); applyErr != nil {
		if errors.Is(applyErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		if errors.Is(applyErr, api.ErrInvalidState) {
			return mcp.NewToolResultError("session is not paused — context override requires a paused session"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("apply context override failed: %v", applyErr)), nil
	}
	return mcp.NewToolResultJSON(map[string]any{
		"status":    "applied",
		"sessionId": sessionID,
		"variables": variables,
	})
}

// handleApplyPayloadOverride stages one-shot variables for the next step.
func (srv *Server) handleApplyPayloadOverride(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	args := req.GetArguments()
	variables, ok := args["variables"].(map[string]any)
	if !ok {
		return mcp.NewToolResultError("variables is required and must be an object"), nil
	}
	if applyErr := srv.execEngine.ApplyPayloadOverride(ctx, sessionID, variables); applyErr != nil {
		if errors.Is(applyErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		if errors.Is(applyErr, api.ErrInvalidState) {
			return mcp.NewToolResultError("session is not paused — payload override requires a paused session"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("apply payload override failed: %v", applyErr)), nil
	}
	return mcp.NewToolResultJSON(map[string]any{
		"status":    "staged",
		"sessionId": sessionID,
		"variables": variables,
	})
}

// handleWaitExecution blocks until the execution session reaches a terminal
// state (success, error, terminated) or the timeout expires, then returns
// the final execution detail. This avoids the need to poll get_execution.
func (srv *Server) handleWaitExecution(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}

	timeoutSecs := req.GetFloat("timeout_seconds", 120)
	if timeoutSecs <= 0 || timeoutSecs > 600 {
		timeoutSecs = 120
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	events, subErr := srv.execEngine.Subscribe(waitCtx, sessionID)
	if subErr != nil {
		if errors.Is(subErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("subscribe failed: %v", subErr)), nil
	}

	// Drain events until the channel closes (terminal state) or timeout fires.
	for range events {
	}

	// Context deadline exceeded — session did not reach a terminal state in time.
	if waitCtx.Err() != nil {
		return mcp.NewToolResultError(fmt.Sprintf(
			"session %q did not complete within %.0f seconds", sessionID, timeoutSecs,
		)), nil
	}

	// Session is terminal — return final detail.
	detail, detailErr := srv.execEngine.Detail(ctx, sessionID)
	if detailErr != nil {
		if errors.Is(detailErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get execution detail failed: %v", detailErr)), nil
	}
	return mcp.NewToolResultJSON(detail)
}

// handleGetStepDetail returns the CCR/CCA AVP detail for a single step.
func (srv *Server) handleGetStepDetail(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.execEngine == nil {
		return mcp.NewToolResultError("execution engine not available"), nil
	}
	sessionID, err := req.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("session_id is required"), nil
	}
	stepNum := int(req.GetFloat("step", 0))
	if stepNum < 1 {
		return mcp.NewToolResultError("step must be a 1-based step number (e.g. 1, 2, 3)"), nil
	}

	detail, detailErr := srv.execEngine.Detail(ctx, sessionID)
	if detailErr != nil {
		if errors.Is(detailErr, api.ErrSessionNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("session %q not found", sessionID)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get execution detail failed: %v", detailErr)), nil
	}

	if stepNum > len(detail.Steps) {
		return mcp.NewToolResultError(fmt.Sprintf(
			"step %d out of range — session has %d step(s)", stepNum, len(detail.Steps),
		)), nil
	}

	step := detail.Steps[stepNum-1]
	return mcp.NewToolResultJSON(map[string]any{
		"n":           step.N,
		"label":       step.Label,
		"requestType": step.RequestType,
		"state":       step.State,
		"durationMs":  step.DurationMs,
		"resultCode":  step.Response["resultCode"],
		"request":     parseAVPText(step.RequestText),
		"response":    parseAVPText(step.ResponseText),
	})
}

// parseAVPText converts the human-readable Diameter AVP text into a slice
// of structured {code, name, value} objects for easy consumption.
func parseAVPText(raw string) []map[string]string {
	var avps []map[string]string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Diameter Message:") {
			continue
		}
		// Format: "  264: Origin-Host. . . . . . ocsclient:1812"
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		code := strings.TrimSpace(line[:colonIdx])
		rest := strings.TrimSpace(line[colonIdx+1:])
		// Split name from value on the dot-leader sequence
		dotIdx := strings.Index(rest, ". ")
		if dotIdx < 0 {
			// Grouped AVP line with no value
			avps = append(avps, map[string]string{"code": code, "name": strings.TrimRight(rest, ". "), "value": "<Grouped>"})
			continue
		}
		name := strings.TrimRight(rest[:dotIdx], ". ")
		value := strings.TrimSpace(rest[dotIdx:])
		value = strings.TrimLeft(value, ". ")
		avps = append(avps, map[string]string{"code": code, "name": name, "value": value})
	}
	return avps
}
