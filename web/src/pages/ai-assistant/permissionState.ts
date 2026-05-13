/**
 * Permission-state reducer for the AI Assistant chat.
 *
 * Manages the session-level "Always" grant set and the per-invocation
 * permission decision.  Pure functions — no React state; tested in
 * permissionState.test.ts.
 */

import type { PermissionTier, SessionPermissions } from './types';

// ─── Types ─────────────────────────────────────────────────────────────────────

export type PermissionDecision = 'allow' | 'deny';

/**
 * Determines whether a tool invocation should be auto-approved, denied, or
 * needs to show the prompt.
 *
 * Rules per the design spec:
 * - `readonly` tools: always auto-approved; never prompts.
 * - `write` tools whose name is in `sessionPermissions`: auto-approved.
 * - `write` tools NOT in `sessionPermissions`: prompt required.
 * - `destructive` tools: ALWAYS prompt, regardless of sessionPermissions.
 */
export function shouldPrompt(
  tier: PermissionTier,
  toolName: string,
  sessionPermissions: SessionPermissions,
): 'silent' | 'prompt' {
  if (tier === 'readonly') return 'silent';
  if (tier === 'destructive') return 'prompt';
  // write tier
  if (sessionPermissions[toolName]) return 'silent';
  return 'prompt';
}

/**
 * Returns an updated sessionPermissions map after an "Always" grant.
 * Immutable — returns a new object.
 */
export function grantAlways(
  sessionPermissions: SessionPermissions,
  toolName: string,
): SessionPermissions {
  return { ...sessionPermissions, [toolName]: true };
}

/**
 * Returns true when the given tool has an "Always" grant in the session.
 */
export function hasAlwaysGrant(
  sessionPermissions: SessionPermissions,
  toolName: string,
): boolean {
  return sessionPermissions[toolName] === true;
}
