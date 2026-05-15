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

// streamOptions asks the OpenAI-compatible endpoint to include a usage object
// in the final SSE chunk so we can report real token counts.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// openAIUsage is the usage shape returned by OpenAI-compatible endpoints.
// Field names differ from Anthropic's so we map them on decode.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// CompletionRequest is the OpenAI-format chat completion request.
type CompletionRequest struct {
	Model         string           `json:"model"`
	Messages      []Message        `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Stream        bool             `json:"stream"`
	StreamOptions *streamOptions   `json:"stream_options,omitempty"`
	// EnableThinking controls the model's extended chain-of-thought reasoning
	// mode (e.g. Qwen3 <think> blocks). When nil the parameter is omitted and
	// the model uses its default. Set to a *false pointer to disable thinking
	// for faster, lower-latency responses.
	EnableThinking *bool `json:"enable_thinking,omitempty"`
}

// Usage holds the token consumption figures returned by the LLM.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// CompletionResponse is the parsed top-level OpenAI chat completion
// response envelope.
type CompletionResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is one candidate response inside a CompletionResponse.
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// streamChunk is a single Server-Sent Event chunk from a streaming
// chat completion response (OpenAI format).
type streamChunk struct {
	Choices []streamChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
}

// streamChoice is one choice delta inside a streamChunk.
type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

// streamDelta carries the incremental content for a streaming choice.
type streamDelta struct {
	Content   string          `json:"content"`
	ToolCalls []toolCallDelta `json:"tool_calls"`
}

// toolCallDelta is an incremental fragment of a tool call in a streaming
// response. Multiple deltas with the same Index are assembled into one
// complete ToolCall.
type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
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
// tool name, description is the tool's human-readable description, and
// tier is one of "write" or "destructive".
//
// The function blocks until the operator's decision arrives (via the
// permission API endpoint) or the request context is cancelled.
// Returns true to allow execution, false to deny.
type PermissionFunc func(callID, toolName, description, tier string) bool

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
