package ai

import (
	"time"
)

// NewAgentWithMock exposes newAgentWithMock to the ai_test package for
// unit testing. It injects both the LLM client and MCP caller as mocks,
// bypassing the production HTTP setup.
func NewAgentWithMock(llmClient LLMClient, caller mcpCaller) *Agent {
	return newAgentWithMock(llmClient, caller)
}

// MCPCaller is the mcpCaller interface exported for use in tests.
type MCPCaller = mcpCaller

// EvictExpiredAt calls the unexported evictExpired on sm with the given
// "now" time. Tests use this to simulate TTL-based eviction with a fake clock.
func EvictExpiredAt(sm *SessionManager, now time.Time) {
	sm.evictExpired(now)
}
