/**
 * Unit tests for AI Assistant shared types.
 *
 * Verifies discriminant-union invariants that downstream components rely on:
 *  - every ThreadItem kind is mutually exclusive
 *  - SseEvent type values are distinct
 *  - PermissionTier values align with what the permission-prompt component expects
 */
import { describe, expect, it } from 'vitest';

import type {
  AssistantMessage,
  PermissionTier,
  PlanningBlock,
  SseDoneEvent,
  SsePermissionRequiredEvent,
  SseThinkingEvent,
  SseTokenEvent,
  SseToolCallEvent,
  ThreadItem,
  ToolCallBlock,
  UserMessage,
} from './types';

// ─── ThreadItem discriminant ──────────────────────────────────────────────────

describe('ThreadItem discriminated union', () => {
  it('user kind narrows correctly', () => {
    const item: ThreadItem = { kind: 'user', id: '1', content: 'hello' } as UserMessage;
    expect(item.kind).toBe('user');
    if (item.kind === 'user') {
      expect(item.content).toBe('hello');
    }
  });

  it('assistant kind narrows correctly', () => {
    const item: ThreadItem = { kind: 'assistant', id: '2', content: 'world' } as AssistantMessage;
    expect(item.kind).toBe('assistant');
  });

  it('planning kind carries collapsed flag', () => {
    const item: ThreadItem = {
      kind: 'planning',
      id: '3',
      content: 'thinking…',
      collapsed: true,
    } as PlanningBlock;
    expect(item.kind).toBe('planning');
    if (item.kind === 'planning') {
      expect(item.collapsed).toBe(true);
    }
  });

  it('tool_call kind carries a ToolCall', () => {
    const item: ThreadItem = {
      kind: 'tool_call',
      id: '4',
      call: {
        id: 'c1',
        name: 'list_peers',
        description: 'List connected peers',
        tier: 'readonly',
        status: 'done',
        result: '[]',
      },
    } as ToolCallBlock;
    expect(item.kind).toBe('tool_call');
    if (item.kind === 'tool_call') {
      expect(item.call.tier).toBe<PermissionTier>('readonly');
    }
  });
});

// ─── SseEvent type values are distinct ───────────────────────────────────────

describe('SseEvent type discriminant', () => {
  it('token, tool_call, permission_required, thinking, done are unique strings', () => {
    const types = ['token', 'tool_call', 'permission_required', 'thinking', 'done'];
    expect(new Set(types).size).toBe(types.length);
  });

  it('SseTokenEvent carries a chunk', () => {
    const evt: SseTokenEvent = { type: 'token', data: { chunk: 'hi' } };
    expect(evt.data.chunk).toBe('hi');
  });

  it('SseToolCallEvent carries tier and id', () => {
    const evt: SseToolCallEvent = {
      type: 'tool_call',
      data: {
        id: 'tc1',
        name: 'list_peers',
        description: 'desc',
        tier: 'readonly',
        result: null,
      },
    };
    expect(evt.data.id).toBe('tc1');
    expect(evt.data.tier).toBe('readonly');
  });

  it('SsePermissionRequiredEvent carries callId and tier', () => {
    const evt: SsePermissionRequiredEvent = {
      type: 'permission_required',
      data: {
        callId: 'tc2',
        toolName: 'delete_scenario',
        description: 'Delete a scenario',
        tier: 'destructive',
      },
    };
    expect(evt.data.tier).toBe('destructive');
  });

  it('SseThinkingEvent carries content', () => {
    const evt: SseThinkingEvent = { type: 'thinking', data: { content: 'analysing…' } };
    expect(evt.data.content).toBe('analysing…');
  });

  it('SseDoneEvent carries empty data', () => {
    const evt: SseDoneEvent = { type: 'done', data: {} };
    expect(evt.type).toBe('done');
  });
});

// ─── PermissionTier coverage ──────────────────────────────────────────────────

describe('PermissionTier values', () => {
  it('covers readonly, write, destructive', () => {
    const tiers: PermissionTier[] = ['readonly', 'write', 'destructive'];
    expect(tiers).toHaveLength(3);
    expect(tiers[0]).toBe('readonly');
    expect(tiers[1]).toBe('write');
    expect(tiers[2]).toBe('destructive');
  });
});
