/**
 * Unit tests for the AI Assistant chat store.
 *
 * Tests the pure-logic state transitions: applying SSE events, pushing
 * user messages, permission prompt lifecycle, planning block toggle,
 * and the session-permission grant flow.
 *
 * Runs in node — no DOM, no React.  We reset the Zustand store between
 * each test via the `reset` action.
 */
import { afterEach, describe, expect, it } from 'vitest';

import { useChatStore } from './chatStore';
import type { SseDoneEvent, SsePermissionRequiredEvent, SseThinkingEvent, SseTokenEvent, SseToolCallEvent } from './types';

// Reset store state after each test.
afterEach(() => {
  useChatStore.getState().reset();
});

// ─── pushUserMessage ──────────────────────────────────────────────────────────

describe('pushUserMessage', () => {
  it('appends a user message to the thread', () => {
    useChatStore.getState().pushUserMessage('hello');
    const { thread } = useChatStore.getState();
    expect(thread).toHaveLength(1);
    expect(thread[0].kind).toBe('user');
    if (thread[0].kind === 'user') {
      expect(thread[0].content).toBe('hello');
    }
  });

  it('assigns unique IDs to sequential messages', () => {
    useChatStore.getState().pushUserMessage('one');
    useChatStore.getState().pushUserMessage('two');
    const { thread } = useChatStore.getState();
    expect(thread[0].id).not.toBe(thread[1].id);
  });
});

// ─── applyEvent — thinking ────────────────────────────────────────────────────

describe('applyEvent — thinking', () => {
  it('creates a collapsed planning block', () => {
    const evt: SseThinkingEvent = { type: 'thinking', data: { content: 'working…' } };
    useChatStore.getState().applyEvent(evt);
    const { thread } = useChatStore.getState();
    expect(thread).toHaveLength(1);
    expect(thread[0].kind).toBe('planning');
    if (thread[0].kind === 'planning') {
      expect(thread[0].content).toBe('working…');
      expect(thread[0].collapsed).toBe(true);
    }
  });

  it('appends to an existing planning block', () => {
    useChatStore.getState().applyEvent({ type: 'thinking', data: { content: 'part1' } } as SseThinkingEvent);
    useChatStore.getState().applyEvent({ type: 'thinking', data: { content: ' part2' } } as SseThinkingEvent);
    const { thread } = useChatStore.getState();
    expect(thread).toHaveLength(1);
    if (thread[0].kind === 'planning') {
      expect(thread[0].content).toBe('part1 part2');
    }
  });
});

// ─── applyEvent — token ───────────────────────────────────────────────────────

describe('applyEvent — token', () => {
  it('creates a new assistant message on first token', () => {
    const evt: SseTokenEvent = { type: 'token', data: { chunk: 'Hello' } };
    useChatStore.getState().applyEvent(evt);
    const { thread } = useChatStore.getState();
    expect(thread).toHaveLength(1);
    expect(thread[0].kind).toBe('assistant');
    if (thread[0].kind === 'assistant') {
      expect(thread[0].content).toBe('Hello');
    }
  });

  it('appends subsequent tokens to the last assistant message', () => {
    useChatStore.getState().applyEvent({ type: 'token', data: { chunk: 'Hello' } } as SseTokenEvent);
    useChatStore.getState().applyEvent({ type: 'token', data: { chunk: ' world' } } as SseTokenEvent);
    const { thread } = useChatStore.getState();
    expect(thread).toHaveLength(1);
    if (thread[0].kind === 'assistant') {
      expect(thread[0].content).toBe('Hello world');
    }
  });

  it('starts a new assistant message after a user message', () => {
    useChatStore.getState().applyEvent({ type: 'token', data: { chunk: 'First' } } as SseTokenEvent);
    useChatStore.getState().pushUserMessage('follow-up');
    useChatStore.getState().applyEvent({ type: 'token', data: { chunk: 'Second' } } as SseTokenEvent);
    const { thread } = useChatStore.getState();
    // Items: assistant, user, assistant
    expect(thread).toHaveLength(3);
    expect(thread[0].kind).toBe('assistant');
    expect(thread[1].kind).toBe('user');
    expect(thread[2].kind).toBe('assistant');
  });
});

// ─── applyEvent — tool_call ───────────────────────────────────────────────────

describe('applyEvent — tool_call', () => {
  it('appends a tool_call block', () => {
    const evt: SseToolCallEvent = {
      type: 'tool_call',
      data: {
        id: 'tc1',
        name: 'list_peers',
        description: 'list peers',
        tier: 'readonly',
        result: '[]',
      },
    };
    useChatStore.getState().applyEvent(evt);
    const { thread } = useChatStore.getState();
    expect(thread).toHaveLength(1);
    expect(thread[0].kind).toBe('tool_call');
    if (thread[0].kind === 'tool_call') {
      expect(thread[0].call.name).toBe('list_peers');
      expect(thread[0].call.tier).toBe('readonly');
      expect(thread[0].call.status).toBe('done'); // result present
    }
  });

  it('sets status to pending when result is null', () => {
    const evt: SseToolCallEvent = {
      type: 'tool_call',
      data: {
        id: 'tc2',
        name: 'start_execution',
        description: 'start',
        tier: 'write',
        result: null,
      },
    };
    useChatStore.getState().applyEvent(evt);
    const { thread } = useChatStore.getState();
    if (thread[0].kind === 'tool_call') {
      expect(thread[0].call.status).toBe('pending');
    }
  });
});

// ─── applyEvent — permission_required ────────────────────────────────────────

describe('applyEvent — permission_required', () => {
  it('sets pendingPrompt for a write-tier tool', () => {
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: {
        callId: 'tc2',
        toolName: 'duplicate_scenario',
        description: 'dup',
        tier: 'write',
      },
    };
    useChatStore.getState().applyEvent(evt);
    const { pendingPrompt } = useChatStore.getState();
    expect(pendingPrompt).not.toBeNull();
    expect(pendingPrompt?.toolName).toBe('duplicate_scenario');
    expect(pendingPrompt?.tier).toBe('write');
  });

  it('sets pendingPrompt for a destructive-tier tool even with an Always grant', () => {
    // Grant Always first
    useChatStore.setState((s) => ({
      sessionPermissions: { delete_all: true },
      streaming: false,
      pendingPrompt: null,
      thread: s.thread,
      _idSeq: s._idSeq,
    }));
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: {
        callId: 'tc3',
        toolName: 'delete_all',
        description: 'delete',
        tier: 'destructive',
      },
    };
    useChatStore.getState().applyEvent(evt);
    expect(useChatStore.getState().pendingPrompt).not.toBeNull();
  });

  it('skips the prompt for a write tool with an Always grant', () => {
    useChatStore.setState((s) => ({
      sessionPermissions: { allowed_tool: true },
      streaming: false,
      pendingPrompt: null,
      thread: s.thread,
      _idSeq: s._idSeq,
    }));
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: {
        callId: 'tc4',
        toolName: 'allowed_tool',
        description: 'already allowed',
        tier: 'write',
      },
    };
    useChatStore.getState().applyEvent(evt);
    expect(useChatStore.getState().pendingPrompt).toBeNull();
  });
});

// ─── applyEvent — done ────────────────────────────────────────────────────────

describe('applyEvent — done', () => {
  it('sets streaming to false', () => {
    useChatStore.getState().startStream();
    expect(useChatStore.getState().streaming).toBe(true);
    const evt: SseDoneEvent = { type: 'done', data: {} };
    useChatStore.getState().applyEvent(evt);
    expect(useChatStore.getState().streaming).toBe(false);
  });
});

// ─── Permission actions ───────────────────────────────────────────────────────

describe('denyPermission', () => {
  it('clears the pending prompt', () => {
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: { callId: 'tc5', toolName: 'delete_all', description: 'd', tier: 'destructive' },
    };
    useChatStore.getState().applyEvent(evt);
    useChatStore.getState().denyPermission('tc5');
    expect(useChatStore.getState().pendingPrompt).toBeNull();
    expect(useChatStore.getState().streaming).toBe(false);
  });
});

describe('approveOnce', () => {
  it('clears the pending prompt and resumes streaming', () => {
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: { callId: 'tc6', toolName: 'dup', description: 'd', tier: 'write' },
    };
    useChatStore.getState().applyEvent(evt);
    useChatStore.getState().approveOnce('tc6');
    expect(useChatStore.getState().pendingPrompt).toBeNull();
    expect(useChatStore.getState().streaming).toBe(true);
  });

  it('does not add to sessionPermissions', () => {
    useChatStore.getState().approveOnce('any');
    expect(Object.keys(useChatStore.getState().sessionPermissions)).toHaveLength(0);
  });
});

describe('approveAlways', () => {
  it('adds the tool to sessionPermissions and resumes streaming', () => {
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: { callId: 'tc7', toolName: 'dup_scenario', description: 'd', tier: 'write' },
    };
    useChatStore.getState().applyEvent(evt);
    useChatStore.getState().approveAlways('tc7');
    const state = useChatStore.getState();
    expect(state.pendingPrompt).toBeNull();
    expect(state.streaming).toBe(true);
    expect(state.sessionPermissions['dup_scenario']).toBe(true);
  });
});

// ─── togglePlanning ───────────────────────────────────────────────────────────

describe('togglePlanning', () => {
  it('expands a collapsed block', () => {
    useChatStore.getState().applyEvent({ type: 'thinking', data: { content: 'x' } } as SseThinkingEvent);
    const { thread } = useChatStore.getState();
    const planId = thread[0].id;
    expect(thread[0].kind === 'planning' && thread[0].collapsed).toBe(true);
    useChatStore.getState().togglePlanning(planId);
    const updated = useChatStore.getState().thread[0];
    expect(updated.kind === 'planning' && updated.collapsed).toBe(false);
  });

  it('collapses an expanded block', () => {
    useChatStore.getState().applyEvent({ type: 'thinking', data: { content: 'x' } } as SseThinkingEvent);
    const { thread } = useChatStore.getState();
    const planId = thread[0].id;
    useChatStore.getState().togglePlanning(planId); // expand
    useChatStore.getState().togglePlanning(planId); // collapse
    const updated = useChatStore.getState().thread[0];
    expect(updated.kind === 'planning' && updated.collapsed).toBe(true);
  });
});

// ─── reset ────────────────────────────────────────────────────────────────────

describe('reset', () => {
  it('clears all state to initial values', () => {
    useChatStore.getState().pushUserMessage('hi');
    useChatStore.getState().startStream();
    useChatStore.getState().reset();
    const state = useChatStore.getState();
    expect(state.thread).toHaveLength(0);
    expect(state.streaming).toBe(false);
    expect(state.pendingPrompt).toBeNull();
    expect(Object.keys(state.sessionPermissions)).toHaveLength(0);
  });
});
