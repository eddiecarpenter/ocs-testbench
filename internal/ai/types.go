// Package ai implements the AI assistant runtime for the OCS Testbench.
//
// It provides an OpenAI-compatible LLM client, an agentic loop that calls
// MCP tools on behalf of the LLM, an in-memory session manager for
// conversation history, and a permission guardrail for write-tier tool
// calls.
//
// The package does NOT import internal/diameter, internal/store,
// internal/engine, or internal/api. All testbench operations are
// accessed through the MCP server's HTTP interface (/mcp), which enforces
// the domain boundary established in docs/ARCHITECTURE.md.
package ai

// Message represents a single message in a conversation thread.
// The Role field follows the OpenAI convention:
//   - "system"    — system prompt injected at the head of every request
//   - "user"      — message from the human operator
//   - "assistant" — response from the LLM
//   - "tool"      — result of a tool call returned to the LLM
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is a single tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // always "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall carries the name and JSON-encoded arguments of a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded argument map
}

// ToolDefinition is the OpenAI function-calling schema shape used in
// CompletionRequest.Tools.
type ToolDefinition struct {
	Type     string      `json:"type"` // always "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef describes a function (tool) to the LLM.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// CompletionRequest is the OpenAI-format chat completion request.
// Stream is always false — the agentic loop uses non-streaming calls so
// the full response (including tool_calls) is available atomically.
type CompletionRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
}

// CompletionResponse is the parsed top-level OpenAI chat completion
// response envelope.
type CompletionResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice is one candidate response inside a CompletionResponse.
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// EmitFunc is the callback used by Agent.Run to stream SSE events to the
// HTTP response writer. The eventType matches the SSE event types already
// defined in internal/api/ai_chat.go (token, tool_call, permission_required,
// done, error, thinking). data is JSON-serialisable.
type EmitFunc func(eventType string, data any) error

// PermissionFunc is the callback used by Agent.Run to request operator
// approval before executing write-tier or destructive-tier tool calls.
//
// callID is the LLM-assigned tool call identifier, toolName is the MCP
// tool name, and tier is one of "write" or "destructive".
//
// The function blocks until the operator's decision arrives (via the
// permission API endpoint) or the request context is cancelled.
// Returns true to allow execution, false to deny.
type PermissionFunc func(callID, toolName, tier string) bool

// PermissionDecision is the decision payload sent by the permission API
// endpoint to the agent goroutine over Session.PermChan.
type PermissionDecision struct {
	CallID   string `json:"callId"`
	Decision string `json:"decision"` // "allow_once" | "allow_always" | "deny"
}

// toolTier maps MCP tool annotation booleans to the permission tier label
// used in SSE events and PermissionFunc arguments.
func toolTier(readOnly, destructive bool) string {
	if readOnly {
		return "readonly"
	}
	if destructive {
		return "destructive"
	}
	return "write"
}
