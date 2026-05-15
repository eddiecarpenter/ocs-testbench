/**
 * AI Assistant chat types — shared across the thread, permission prompt,
 * compose bar, and SSE client (Feature #177).
 *
 * The SSE event schema is forward-compatible with the real agentic loop
 * landing in Feature 3: the mock backend emits the same types so the
 * frontend requires no changes on the swap.
 */

// ─── Permission tiers ────────────────────────────────────────────────────────

/** The three-tier permission model for AI tool calls. */
export type PermissionTier = 'readonly' | 'write' | 'destructive';

// ─── Tool call ────────────────────────────────────────────────────────────────

export type ToolCallStatus = 'pending' | 'running' | 'done' | 'denied';

export interface ToolCall {
  /** Unique ID for this call within the current stream. */
  id: string;
  /** Name of the tool being invoked. */
  name: string;
  /** Human-readable description of the operation. */
  description: string;
  /** Permission tier controlling the guardrail behaviour. */
  tier: PermissionTier;
  /** Current execution status. */
  status: ToolCallStatus;
  /** Serialised result once the call completes; null while pending/running. */
  result: string | null;
}

// ─── Thread messages ─────────────────────────────────────────────────────────

export type MessageRole = 'user' | 'assistant';

/** A single assistant text chunk or fully assembled message. */
export interface AssistantMessage {
  kind: 'assistant';
  id: string;
  /** Accumulated text content. Tokens are appended as they stream in. */
  content: string;
  /**
   * True when this message was part of an aborted exchange.
   * It is shown in the thread but excluded from the next request payload.
   */
  aborted?: true;
}

/** A message the user typed and submitted. */
export interface UserMessage {
  kind: 'user';
  id: string;
  content: string;
  /**
   * True when this message was part of an aborted exchange.
   * It is shown in the thread but excluded from the next request payload.
   */
  aborted?: true;
}

/**
 * Planning / reasoning block emitted before the assistant's first response.
 * Displayed as a collapsible purple block.
 */
export interface PlanningBlock {
  kind: 'planning';
  id: string;
  content: string;
  /** Whether the block is collapsed in the UI. Defaults to true. */
  collapsed: boolean;
}

/** An inline tool call block within the assistant's response flow. */
export interface ToolCallBlock {
  kind: 'tool_call';
  id: string;
  call: ToolCall;
}

export type ThreadItem =
  | UserMessage
  | AssistantMessage
  | PlanningBlock
  | ToolCallBlock;

// ─── Permission prompt state ─────────────────────────────────────────────────

/** Pending permission prompt — surfaced when a write/destructive tool fires. */
export interface PermissionPromptState {
  callId: string;
  toolName: string;
  description: string;
  tier: PermissionTier;
}

// ─── Session permission memory (Always grants) ───────────────────────────────

/**
 * Tools approved with "Always" for the current browser session.
 * Key is the tool name.
 */
export type SessionPermissions = Record<string, true>;

// ─── SSE events from /api/ai/chat ────────────────────────────────────────────

/** Streamed text chunk from the assistant. */
export interface SseTokenEvent {
  type: 'token';
  data: { chunk: string };
}

/** Tool invocation event — may require a permission prompt. */
export interface SseToolCallEvent {
  type: 'tool_call';
  data: {
    id: string;
    name: string;
    description: string;
    tier: PermissionTier;
    result: string | null;
  };
}

/** Permission required — stream pauses until the user responds. */
export interface SsePermissionRequiredEvent {
  type: 'permission_required';
  data: {
    callId: string;
    toolName: string;
    description: string;
    tier: PermissionTier;
  };
}

/** Planning / reasoning block content. */
export interface SseThinkingEvent {
  type: 'thinking';
  data: { content: string };
}

/** Mid-stream usage update — fired as soon as token counts are known. */
export interface SseUsageUpdateEvent {
  type: 'usage_update';
  data: { inputTokens?: number; outputTokens?: number };
}

/** Terminal event — stream complete. */
export interface SseDoneEvent {
  type: 'done';
  data: { inputTokens?: number; outputTokens?: number };
}

export type SseEvent =
  | SseTokenEvent
  | SseToolCallEvent
  | SsePermissionRequiredEvent
  | SseThinkingEvent
  | SseUsageUpdateEvent
  | SseDoneEvent;

/** Request body for POST /api/ai/chat */
export interface ChatRequest {
  messages: Array<{ role: MessageRole; content: string }>;
}
