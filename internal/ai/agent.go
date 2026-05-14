package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
// simultaneously. Agent holds no mutable per-run state.
type Agent struct {
	llm    LLMClient
	mcpURL string // full URL for the MCP endpoint, e.g. "http://host/mcp"
	prompt string
	// testMCP is non-nil only in tests; overrides the real mcp-go client.
	testMCP mcpCaller
}

// NewAgent creates a production Agent. Returns nil when cfg.Endpoint is
// empty — callers use nil as the "feature disabled" signal and return 503.
func NewAgent(cfg baseconfig.AIConfig, mcpBaseURL string) *Agent {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil
	}
	llm, err := NewHTTPLLMClient(cfg)
	if err != nil {
		return nil
	}
	return &Agent{
		llm:    llm,
		mcpURL: mcpBaseURL + "/mcp",
		prompt: SystemPrompt,
	}
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

	// Agentic loop — runs until the LLM produces a final response with no
	// tool_calls, or the context is cancelled, or a fatal error occurs.
	for {
		if ctx.Err() != nil {
			return updatedHistory, ctx.Err()
		}

		resp, err := a.llm.Complete(ctx, CompletionRequest{
			Messages: messages,
			Tools:    toolDefs,
		})
		if err != nil {
			_ = emit("error", map[string]string{"message": "LLM error: " + err.Error()})
			return updatedHistory, nil
		}
		if len(resp.Choices) == 0 {
			_ = emit("error", map[string]string{"message": "LLM returned no choices"})
			return updatedHistory, nil
		}

		choice := resp.Choices[0].Message
		messages = append(messages, choice)

		// No tool calls → stream the final text response.
		if len(choice.ToolCalls) == 0 {
			updatedHistory = append(updatedHistory, choice)
			chunks := splitIntoChunks(choice.Content, 50)
			for _, chunk := range chunks {
				if err := emit("token", map[string]string{"chunk": chunk}); err != nil {
					return updatedHistory, nil
				}
			}
			_ = emit("done", map[string]any{})
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
				if !perm(tc.ID, tc.Function.Name, tier) {
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
			var resultAny any
			_ = json.Unmarshal([]byte(resultContent), &resultAny)
			_ = emit("tool_call", map[string]any{
				"id":          tc.ID,
				"name":        tc.Function.Name,
				"description": toolDescription(toolsResult.Tools, tc.Function.Name),
				"tier":        tier,
				"result":      resultAny,
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
		params := map[string]any{
			"type": "object",
		}
		// Preserve the MCP input schema properties if present.
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

// splitIntoChunks splits content into chunks of at most chunkSize runes.
// This simulates token streaming for the frontend.
func splitIntoChunks(content string, chunkSize int) []string {
	runes := []rune(content)
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	if len(chunks) == 0 {
		chunks = []string{""}
	}
	return chunks
}
