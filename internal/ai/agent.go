package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// mcpCaller is the minimal interface for MCP operations required by the
// Agent. It is satisfied by *mcpclient.Client in production and by a
// lightweight mock in unit tests.
type mcpCaller interface {
	Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error)
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// Agent is the agentic loop runtime. It drives a conversation with an
// LLM, executing MCP tool calls on behalf of the model until the model
// produces a final text response with no further tool calls.
//
// Agent is safe for concurrent use — multiple goroutines may call Run
// simultaneously. The mu/llm/cfg fields are guarded by mu; all other
// fields are immutable after construction.
type Agent struct {
	mu  sync.RWMutex
	llm LLMClient
	cfg baseconfig.AIConfig

	mcpURL string // full URL for the MCP endpoint, e.g. "http://host/mcp"
	prompt string
	// saveFn is called after every successful Reconfigure so settings survive
	// restarts. nil in tests and when no persistence path is configured.
	saveFn func(baseconfig.AIConfig) error
	// testMCP is non-nil only in tests; overrides the real mcp-go client.
	testMCP mcpCaller
}

// NewAgent creates a production Agent. Returns nil when cfg.Endpoint is
// empty — callers use nil as the "feature disabled" signal and return 503.
func NewAgent(cfg baseconfig.AIConfig, mcpBaseURL string) *Agent {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil
	}
	llm, err := NewLLMClient(cfg)
	if err != nil {
		return nil
	}
	return &Agent{
		llm:    llm,
		cfg:    cfg,
		mcpURL: mcpBaseURL + "/mcp",
		prompt: SystemPrompt,
	}
}

// MCPTool is a simplified view of a single MCP tool with its permission tier.
type MCPTool struct {
	Name        string
	Description string
	Tier        string
}

// ListMCPTools fetches the current MCP tool list and returns a slice of
// MCPTool values including the permission tier derived from tool annotations.
// Returns an empty slice (not an error) when the MCP server has no tools.
func (a *Agent) ListMCPTools(ctx context.Context) ([]MCPTool, error) {
	caller, err := a.getMCPCaller(ctx)
	if err != nil {
		return nil, fmt.Errorf("ai: list MCP tools: %w", err)
	}
	result, err := caller.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		if strings.Contains(err.Error(), "tools not supported") {
			return nil, nil
		}
		return nil, fmt.Errorf("ai: list MCP tools: %w", err)
	}
	out := make([]MCPTool, 0, len(result.Tools))
	for _, t := range result.Tools {
		out = append(out, MCPTool{
			Name:        t.Name,
			Description: t.Description,
			Tier:        toolTierFromList(result.Tools, t.Name),
		})
	}
	return out, nil
}

// Reconfigure atomically swaps the LLM client with one built from cfg.
// In-flight Run calls continue with the old client; subsequent calls use
// the new one. Returns an error when cfg.Endpoint is empty or the new
// client cannot be constructed.
func (a *Agent) Reconfigure(cfg baseconfig.AIConfig) error {
	newLLM, err := NewLLMClient(cfg)
	if err != nil {
		return fmt.Errorf("ai: reconfigure: %w", err)
	}
	a.mu.Lock()
	a.llm = newLLM
	a.cfg = cfg
	saveFn := a.saveFn
	a.mu.Unlock()
	if saveFn != nil {
		if err := saveFn(cfg); err != nil {
			// Log but don't fail the reconfigure — in-memory update succeeded.
			fmt.Printf("ai: persist config: %v\n", err)
		}
	}
	return nil
}

// SetSaveFn registers the function called after every successful Reconfigure
// to persist the config across restarts. Call once after construction.
func (a *Agent) SetSaveFn(fn func(baseconfig.AIConfig) error) {
	a.mu.Lock()
	a.saveFn = fn
	a.mu.Unlock()
}

// GetConfig returns a copy of the current AIConfig with the API key masked.
// When the API key is non-empty, APIKey is replaced with "••••" so the
// caller can signal to the UI that a key is set without exposing it.
func (a *Agent) GetConfig() baseconfig.AIConfig {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	if cfg.APIKey != "" {
		cfg.APIKey = "••••"
	}
	return cfg
}

// GetRawAPIKey returns the unmasked API key under a read lock.
// This is intentionally a separate method from GetConfig so callers
// that only need to check whether a key is set use GetConfig (safe for
// logging/serialisation), while callers that must forward the key to an
// upstream service (e.g. the /config/ai/models proxy) call this method
// explicitly — making the usage auditable.
func (a *Agent) GetRawAPIKey() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.APIKey
}

// NewAgentWithClient creates a testable Agent with a custom LLMClient.
// The mcpBaseURL is used to construct the real mcp-go client for MCP calls;
// pass an httptest.Server URL for end-to-end tests that need a real MCP server.
func NewAgentWithClient(llmClient LLMClient, mcpBaseURL string) *Agent {
	return &Agent{
		llm:    llmClient,
		mcpURL: mcpBaseURL + "/mcp",
		prompt: SystemPrompt,
	}
}

// newAgentWithMock creates a fully-mocked Agent for unit tests.
// It is unexported to keep the public API minimal; tests in the ai_test
// package access it via the exported test helper (see export_test.go).
func newAgentWithMock(llmClient LLMClient, caller mcpCaller) *Agent {
	return &Agent{
		llm:     llmClient,
		prompt:  SystemPrompt,
		testMCP: caller,
	}
}

// llmClient returns the current LLM client under a read lock so callers
// get a stable reference for the duration of a single send.
func (a *Agent) llmClient() LLMClient {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.llm
}

// getMCPCaller returns the mcpCaller for a Run. In tests, testMCP is
// returned directly. In production, a new *mcpclient.Client is created,
// connected, and initialized for the run. The caller is not cached across
// runs so there is no stale-connection state.
func (a *Agent) getMCPCaller(ctx context.Context) (mcpCaller, error) {
	if a.testMCP != nil {
		return a.testMCP, nil
	}
	mc, err := mcpclient.NewStreamableHttpClient(a.mcpURL)
	if err != nil {
		return nil, fmt.Errorf("ai: create MCP client: %w", err)
	}
	_, err = mc.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "ocs-testbench-agent",
				Version: "1.0",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ai: initialize MCP client: %w", err)
	}
	return mc, nil
}

// Run executes the agentic loop for one conversation turn. It:
//  1. Fetches the MCP tool list and converts them to OpenAI tool definitions.
//  2. Sends the system prompt + history + user message to the LLM.
//  3. For each tool_call in the response:
//     - readonly tier: executes immediately via MCP.
//     - write/destructive tier: calls perm to request operator approval;
//     denies if perm returns false.
//  4. Appends tool results and loops back to step 2.
//  5. When the LLM responds with no tool_calls, emits the content as
//     "token" SSE events followed by a "done" event.
//
// Run returns the updated history (including the new assistant turn and
// all tool-result messages) so the caller can store it for the next turn.
//
// If the context is cancelled mid-loop Run returns the context error.
// LLM errors and MCP errors are surfaced as an "error" SSE event and
// terminate the loop — Run returns nil in this case so the HTTP handler
// does not double-error.
func (a *Agent) Run(
	ctx context.Context,
	history []Message,
	emit EmitFunc,
	perm PermissionFunc,
) ([]Message, error) {
	caller, err := a.getMCPCaller(ctx)
	if err != nil {
		_ = emit("error", map[string]string{"message": err.Error()})
		return history, nil
	}

	// Fetch the MCP tool list and convert to OpenAI tool definitions.
	// "tools not supported" is returned by servers with no tools registered —
	// treat it as an empty list so the agent can still answer without tools.
	toolsResult, err := caller.ListTools(ctx, mcp.ListToolsRequest{})
	var toolDefs []ToolDefinition
	if err != nil {
		if !strings.Contains(err.Error(), "tools not supported") {
			_ = emit("error", map[string]string{"message": "failed to list tools: " + err.Error()})
			return history, nil
		}
		toolsResult = &mcp.ListToolsResult{}
	}
	toolDefs = mcpToolsToOpenAI(toolsResult.Tools)

	// Build the messages array: system prompt first, then history.
	messages := make([]Message, 0, 1+len(history))
	messages = append(messages, Message{Role: "system", Content: a.prompt})
	messages = append(messages, history...)

	updatedHistory := append([]Message(nil), history...)
	var totalUsage Usage

	// Agentic loop — runs until the LLM produces a final response with no
	// tool_calls, or the context is cancelled, or a fatal error occurs.
	for {
		if ctx.Err() != nil {
			return updatedHistory, ctx.Err()
		}

		req := CompletionRequest{
			Messages: messages,
			Tools:    toolDefs,
		}
		// When thinking is disabled, explicitly tell the model to skip its
		// chain-of-thought reasoning phase (Qwen3 and compatible models).
		if !a.cfg.Thinking {
			f := false
			req.EnableThinking = &f
		}
		resp, err := a.llmClient().CompleteStream(ctx, req,
			func(chunk string) {
				_ = emit("token", map[string]string{"chunk": chunk})
			},
			func(u Usage) {
				_ = emit("usage_update", map[string]any{
					"inputTokens":  u.InputTokens,
					"outputTokens": u.OutputTokens,
				})
			},
		)
		if err != nil {
			_ = emit("error", map[string]string{"message": "LLM error: " + err.Error()})
			return updatedHistory, nil
		}
		if len(resp.Choices) == 0 {
			_ = emit("error", map[string]string{"message": "LLM returned no choices"})
			return updatedHistory, nil
		}
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens

		choice := resp.Choices[0].Message
		messages = append(messages, choice)

		// No tool calls → streaming is complete; signal done with token usage.
		if len(choice.ToolCalls) == 0 {
			updatedHistory = append(updatedHistory, choice)
			_ = emit("done", map[string]any{
				"inputTokens":  totalUsage.InputTokens,
				"outputTokens": totalUsage.OutputTokens,
			})
			return updatedHistory, nil
		}

		// Process each tool call.
		updatedHistory = append(updatedHistory, choice)
		for _, tc := range choice.ToolCalls {
			if ctx.Err() != nil {
				return updatedHistory, ctx.Err()
			}

			// Determine tier from tool annotations.
			tier := toolTierFromList(toolsResult.Tools, tc.Function.Name)

			// Permission check for non-readonly tools.
			if tier != "readonly" {
				if !perm(tc.ID, tc.Function.Name, toolDescription(toolsResult.Tools, tc.Function.Name), tier) {
					// Denied — inject an error result so the LLM can recover.
					errMsg := Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    fmt.Sprintf(`{"error": "tool call %q denied by operator"}`, tc.Function.Name),
					}
					messages = append(messages, errMsg)
					updatedHistory = append(updatedHistory, errMsg)
					continue
				}
			}

			// Execute the tool via MCP.
			var args map[string]any
			if tc.Function.Arguments != "" {
				if unmarshalErr := json.Unmarshal([]byte(tc.Function.Arguments), &args); unmarshalErr != nil {
					errMsg := Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    fmt.Sprintf(`{"error": "could not parse tool arguments: %s"}`, unmarshalErr.Error()),
					}
					messages = append(messages, errMsg)
					updatedHistory = append(updatedHistory, errMsg)
					continue
				}
			}
			toolResult, toolErr := caller.CallTool(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      tc.Function.Name,
					Arguments: args,
				},
			})

			var resultContent string
			if toolErr != nil {
				resultContent = fmt.Sprintf(`{"error": %q}`, toolErr.Error())
			} else {
				resultContent = mcpResultToString(toolResult)
			}

			// Emit tool_call SSE event to the frontend.
			_ = emit("tool_call", map[string]any{
				"id":          tc.ID,
				"name":        tc.Function.Name,
				"description": toolDescription(toolsResult.Tools, tc.Function.Name),
				"tier":        tier,
				"result":      resultContent,
			})

			// Append tool result message for the next LLM call.
			resultMsg := Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    resultContent,
			}
			messages = append(messages, resultMsg)
			updatedHistory = append(updatedHistory, resultMsg)
		}
	}
}

// mcpToolsToOpenAI converts a slice of MCP tools to the OpenAI
// function-calling tool definition format.
func mcpToolsToOpenAI(tools []mcp.Tool) []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(tools))
	for _, t := range tools {
		// Always include "properties" — even as an empty object — because
		// strict OpenAI-compatible validators (LM Studio, etc.) require the
		// field to be present and reject requests where it is absent.
		params := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		// Overwrite with the actual schema properties when present.
		if len(t.InputSchema.Properties) > 0 {
			props := make(map[string]any, len(t.InputSchema.Properties))
			for k, v := range t.InputSchema.Properties {
				props[k] = v
			}
			params["properties"] = props
		}
		if len(t.InputSchema.Required) > 0 {
			params["required"] = t.InputSchema.Required
		}
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return defs
}

// toolTierFromList returns the permission tier for a named tool by
// inspecting its annotation flags in the tool list.
func toolTierFromList(tools []mcp.Tool, name string) string {
	for _, t := range tools {
		if t.Name == name {
			readOnly := t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint
			destructive := t.Annotations.DestructiveHint != nil && *t.Annotations.DestructiveHint
			return toolTier(readOnly, destructive)
		}
	}
	// Unknown tool — treat as write (safest non-readonly tier).
	return "write"
}

// toolDescription returns the description for a named tool.
func toolDescription(tools []mcp.Tool, name string) string {
	for _, t := range tools {
		if t.Name == name {
			return t.Description
		}
	}
	return ""
}

// mcpResultToString converts a CallToolResult to a JSON string suitable
// for use as a tool-role message content. IsError results are wrapped in
// an {"error": ...} envelope so the LLM can detect and handle them.
func mcpResultToString(result *mcp.CallToolResult) string {
	if result == nil {
		return `{"error": "nil result"}`
	}

	// Collect text content from the result.
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	text := strings.Join(parts, "\n")

	if result.IsError {
		b, _ := json.Marshal(map[string]string{"error": text})
		return string(b)
	}

	// Try to return the content as-is if it's valid JSON, otherwise
	// wrap it as a plain string.
	var raw any
	if json.Unmarshal([]byte(text), &raw) == nil {
		return text
	}
	b, _ := json.Marshal(map[string]string{"result": text})
	return string(b)
}

