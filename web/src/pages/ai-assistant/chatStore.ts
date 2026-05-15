/**
 * Zustand store for the AI Assistant chat.
 *
 * All state mutations flow through the actions exposed here.  The store
 * is intentionally simple: append-only thread, one active permission
 * prompt at a time, one streaming status flag.
 *
 * Import the hook as `useChatStore` in component code.
 */
import { create } from 'zustand';

import { grantAlways, shouldPrompt } from './permissionState';
import type {
  PermissionPromptState,
  SessionPermissions,
  SseEvent,
  ThreadItem,
  ToolCall,
} from './types';

// ─── State shape ─────────────────────────────────────────────────────────────

export interface ChatState {
  /** Ordered thread items (messages, tool calls, planning blocks). */
  thread: ThreadItem[];
  /** True while the SSE stream is open. */
  streaming: boolean;
  /** Epoch ms when the current stream started; null when no stream has run yet. */
  streamStartedAt: number | null;
  /** Epoch ms when the last stream ended; null while streaming or before any stream. */
  streamEndedAt: number | null;
  /** Input tokens used in the current/last turn (from LLM usage report). */
  inputTokens: number;
  /** Output tokens generated in the current/last turn (from LLM usage report). */
  outputTokens: number;
  /** Running character count of streamed output — used for live ~token estimate. */
  outputChars: number;
  /** Non-null when a tool approval prompt is pending. */
  pendingPrompt: PermissionPromptState | null;
  /** Tools approved for the entire session. */
  sessionPermissions: SessionPermissions;
  /** Monotonically-increasing counter for generating stable IDs. */
  _idSeq: number;
}

// ─── Actions ─────────────────────────────────────────────────────────────────

export interface ChatActions {
  /**
   * Process a single SSE event from the /api/ai/chat stream.
   * This is the primary entry point for stream-driven state changes.
   */
  applyEvent: (event: SseEvent) => void;

  /** Called when the user submits a message. */
  pushUserMessage: (content: string) => void;

  /** Toggle the collapsed state of a planning block. */
  togglePlanning: (id: string) => void;

  /**
   * User clicked "Deny" on the permission prompt.
   * Marks the tool call as denied and aborts the stream.
   */
  denyPermission: (callId: string) => void;

  /**
   * User clicked "Once" — approve this invocation only.
   * Clears the prompt so the stream can continue.
   */
  approveOnce: (callId: string) => void;

  /**
   * User clicked "Always" — grant session-wide approval for this tool.
   * Clears the prompt so the stream can continue.
   */
  approveAlways: (callId: string) => void;

  /** Mark streaming as started. */
  startStream: () => void;

  /** Mark streaming as ended (clean finish or error). */
  endStream: () => void;

  /**
   * Abort an in-flight stream. Removes the trailing partial assistant
   * message (if any) so the thread doesn't carry an incomplete response
   * into the next turn.
   */
  abortStream: () => void;

  /** Append an error message to the thread. */
  appendError: (message: string) => void;

  /** Reset state for a new conversation. */
  reset: () => void;
}

// ─── Initial state ─────────────────────────────────────────────────────────────

const initialState: ChatState = {
  thread: [],
  streaming: false,
  streamStartedAt: null,
  streamEndedAt: null,
  inputTokens: 0,
  outputTokens: 0,
  outputChars: 0,
  pendingPrompt: null,
  sessionPermissions: {},
  _idSeq: 0,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function nextId(state: ChatState): [string, ChatState] {
  const id = `item-${state._idSeq + 1}`;
  return [id, { ...state, _idSeq: state._idSeq + 1 }];
}

/**
 * Return the last item in the thread only if it is an assistant message,
 * otherwise null.  Tokens must only append to the immediately-preceding
 * assistant turn — not to an older one buried earlier in the thread.
 */
function lastAssistantMessage(thread: ThreadItem[]) {
  const last = thread[thread.length - 1];
  if (last && last.kind === 'assistant') return last;
  return null;
}

/** Update a tool call block identified by its call.id. */
function updateToolCallInThread(
  thread: ThreadItem[],
  callId: string,
  update: Partial<ToolCall>,
): ThreadItem[] {
  return thread.map((item) => {
    if (item.kind !== 'tool_call') return item;
    if (item.call.id !== callId) return item;
    return { ...item, call: { ...item.call, ...update } };
  });
}

// ─── Store ─────────────────────────────────────────────────────────────────────

export const useChatStore = create<ChatState & ChatActions>((set, get) => ({
  ...initialState,

  applyEvent(event: SseEvent) {
    switch (event.type) {
      case 'thinking': {
        const state = get();
        // Replace or extend the current planning block.
        const existing = state.thread.find((i) => i.kind === 'planning');
        if (existing && existing.kind === 'planning') {
          set({
            thread: state.thread.map((i) =>
              i.kind === 'planning'
                ? { ...i, content: i.content + event.data.content }
                : i,
            ),
          });
        } else {
          const [id, next] = nextId(state);
          set({
            thread: [
              ...state.thread,
              {
                kind: 'planning',
                id,
                content: event.data.content,
                collapsed: true,
              },
            ],
            _idSeq: next._idSeq,
          });
        }
        break;
      }

      case 'token': {
        const state = get();
        const chars = event.data.chunk.length;
        const last = lastAssistantMessage(state.thread);
        if (last && last.kind === 'assistant') {
          set({
            thread: state.thread.map((i) =>
              i.id === last.id && i.kind === 'assistant'
                ? { ...i, content: i.content + event.data.chunk }
                : i,
            ),
            outputChars: state.outputChars + chars,
          });
        } else {
          const [id, next] = nextId(state);
          set({
            thread: [
              ...state.thread,
              { kind: 'assistant', id, content: event.data.chunk },
            ],
            _idSeq: next._idSeq,
            outputChars: state.outputChars + chars,
          });
        }
        break;
      }

      case 'tool_call': {
        const state = get();
        const { id, name, description, tier, result } = event.data;
        // Check whether to surface a permission prompt.
        const decision = shouldPrompt(tier, name, state.sessionPermissions);
        const callStatus = result !== null ? 'done' : 'pending';

        const [itemId, next] = nextId(state);
        set({
          thread: [
            ...state.thread,
            {
              kind: 'tool_call',
              id: itemId,
              call: {
                id,
                name,
                description,
                tier,
                status: callStatus,
                result: result !== null
                  ? (typeof result === 'string' ? result : JSON.stringify(result, null, 2))
                  : null,
              },
            },
          ],
          _idSeq: next._idSeq,
        });

        // Readonly: silent — no prompt needed.
        // Write/destructive handled by permission_required event.
        void decision; // used for reference; prompt surfaced via permission_required
        break;
      }

      case 'permission_required': {
        const state = get();
        const { callId, toolName, description, tier } = event.data;
        // Destructive always prompts; write prompts unless Always-granted.
        const decision = shouldPrompt(tier, toolName, state.sessionPermissions);
        if (decision === 'silent') {
          // Already granted for this session — no prompt.
          break;
        }
        set({
          pendingPrompt: { callId, toolName, description, tier },
          streaming: false, // pause stream indicator
        });
        break;
      }

      case 'usage_update': {
        // Fired mid-stream as soon as counts are known (Anthropic: on
        // message_start; OpenAI: on the final usage chunk).
        const u = event.data;
        set((s) => ({
          inputTokens:  u.inputTokens  ?? s.inputTokens,
          outputTokens: u.outputTokens ?? s.outputTokens,
        }));
        break;
      }

      case 'done': {
        set((s) => ({
          streaming: false,
          streamEndedAt: Date.now(),
          // Prefer values already set by usage_update; fall back to done payload.
          inputTokens:  event.data.inputTokens  ?? s.inputTokens,
          outputTokens: event.data.outputTokens ?? s.outputTokens,
        }));
        break;
      }
    }
  },

  pushUserMessage(content: string) {
    const state = get();
    const [id, next] = nextId(state);
    set({
      thread: [...state.thread, { kind: 'user', id, content }],
      _idSeq: next._idSeq,
    });
  },

  togglePlanning(id: string) {
    set((state) => ({
      thread: state.thread.map((item) =>
        item.kind === 'planning' && item.id === id
          ? { ...item, collapsed: !item.collapsed }
          : item,
      ),
    }));
  },

  denyPermission(callId: string) {
    set((state) => ({
      thread: updateToolCallInThread(state.thread, callId, { status: 'denied' }),
      pendingPrompt: null,
      streaming: false,
    }));
  },

  approveOnce(_callId: string) { // eslint-disable-line @typescript-eslint/no-unused-vars
    // Clear the prompt; the caller resumes the stream.
    set({ pendingPrompt: null, streaming: true });
  },

  approveAlways(callId: string) {
    const state = get();
    const prompt = state.pendingPrompt;
    if (!prompt) return;
    set({
      sessionPermissions: grantAlways(state.sessionPermissions, prompt.toolName),
      pendingPrompt: null,
      streaming: true,
    });
    void callId; // identified by prompt.toolName
  },

  startStream() {
    set({ streaming: true, streamStartedAt: Date.now(), streamEndedAt: null, inputTokens: 0, outputTokens: 0, outputChars: 0 });
  },

  endStream() {
    set({ streaming: false, streamEndedAt: Date.now() });
  },

  abortStream() {
    const state = get();
    // Mark the trailing partial assistant message (if any) and the user message
    // that triggered the aborted stream as aborted=true.  They remain visible
    // in the thread but are excluded from the next request payload.
    const thread = state.thread.map((item, i, arr) => {
      const isLast = i === arr.length - 1;
      const isSecondLast = i === arr.length - 2;
      if (isLast && item.kind === 'assistant') {
        return { ...item, aborted: true as const };
      }
      if (isLast && item.kind === 'user') {
        return { ...item, aborted: true as const };
      }
      if (isSecondLast && item.kind === 'user' && arr.at(-1)?.kind === 'assistant') {
        return { ...item, aborted: true as const };
      }
      return item;
    });
    set({ streaming: false, streamStartedAt: null, streamEndedAt: null, thread });
  },

  appendError(message: string) {
    const state = get();
    const [id, next] = nextId(state);
    set({
      thread: [
        ...state.thread,
        { kind: 'assistant', id, content: `⚠ ${message}` },
      ],
      _idSeq: next._idSeq,
      streaming: false,
      streamEndedAt: Date.now(),
    });
  },

  reset() {
    set(initialState);
  },
}));
