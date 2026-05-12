/**
 * Unit tests for the AI Assistant permission-state logic.
 *
 * Covers:
 *  - shouldPrompt: readonly always silent, destructive always prompts,
 *    write respects session grants, destructive ignores session grants.
 *  - grantAlways: immutable update, idempotent re-grant.
 *  - hasAlwaysGrant: positive and negative cases.
 */
import { describe, expect, it } from 'vitest';

import {
  grantAlways,
  hasAlwaysGrant,
  shouldPrompt,
} from './permissionState';
import type { SessionPermissions } from './types';

const EMPTY: SessionPermissions = {};

describe('shouldPrompt', () => {
  it('readonly tier is always silent', () => {
    expect(shouldPrompt('readonly', 'list_peers', EMPTY)).toBe('silent');
    expect(shouldPrompt('readonly', 'list_peers', { list_peers: true })).toBe('silent');
  });

  it('destructive tier always prompts regardless of session grants', () => {
    expect(shouldPrompt('destructive', 'delete_all', EMPTY)).toBe('prompt');
    expect(shouldPrompt('destructive', 'delete_all', { delete_all: true })).toBe(
      'prompt',
    );
  });

  it('write tier prompts when no session grant exists', () => {
    expect(shouldPrompt('write', 'duplicate_scenario', EMPTY)).toBe('prompt');
  });

  it('write tier is silent when an Always grant is present', () => {
    expect(shouldPrompt('write', 'duplicate_scenario', { duplicate_scenario: true })).toBe('silent');
  });

  it('write tier prompt is per tool name — other tools are unaffected', () => {
    const perms: SessionPermissions = { other_tool: true };
    expect(shouldPrompt('write', 'duplicate_scenario', perms)).toBe('prompt');
  });
});

describe('grantAlways', () => {
  it('adds the tool name to the permissions map', () => {
    const result = grantAlways(EMPTY, 'duplicate_scenario');
    expect(result).toEqual({ duplicate_scenario: true });
  });

  it('does not mutate the original map', () => {
    const original: SessionPermissions = {};
    grantAlways(original, 'tool_x');
    expect(original).toEqual({});
  });

  it('preserves existing grants when adding a new one', () => {
    const existing: SessionPermissions = { existing_tool: true };
    const result = grantAlways(existing, 'new_tool');
    expect(result).toEqual({ existing_tool: true, new_tool: true });
  });

  it('is idempotent — re-granting produces the same map shape', () => {
    const once = grantAlways(EMPTY, 'tool_a');
    const twice = grantAlways(once, 'tool_a');
    expect(twice).toEqual({ tool_a: true });
  });
});

describe('hasAlwaysGrant', () => {
  it('returns true for a granted tool', () => {
    const perms: SessionPermissions = { my_tool: true };
    expect(hasAlwaysGrant(perms, 'my_tool')).toBe(true);
  });

  it('returns false when the tool is absent', () => {
    expect(hasAlwaysGrant(EMPTY, 'my_tool')).toBe(false);
  });

  it('returns false for a different tool name', () => {
    const perms: SessionPermissions = { other_tool: true };
    expect(hasAlwaysGrant(perms, 'my_tool')).toBe(false);
  });
});
