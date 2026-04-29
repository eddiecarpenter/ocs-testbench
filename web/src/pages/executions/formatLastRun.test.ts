/**
 * Tests for the sidebar's "Last run" sub-line formatter.
 *
 * Pure logic — covers the relative-time bucket boundaries and the
 * fall-through "Never run" case for scenarios that have never been
 * executed.
 */
import { describe, expect, it } from 'vitest';

import { formatLastRun } from './formatLastRun';

const NOW = Date.parse('2026-04-29T10:00:00Z');

describe('formatLastRun', () => {
  it('returns "Never run" when no startedAt is provided', () => {
    expect(formatLastRun(undefined, NOW)).toBe('Never run');
  });

  it('returns "Never run" when startedAt is an unparseable string', () => {
    expect(formatLastRun('not-a-date', NOW)).toBe('Never run');
  });

  it('reports "just now" when the run started under a minute ago', () => {
    expect(
      formatLastRun(new Date(NOW - 30_000).toISOString(), NOW),
    ).toBe('Last run: just now');
  });

  it('reports minutes when under an hour ago', () => {
    expect(
      formatLastRun(new Date(NOW - 5 * 60_000).toISOString(), NOW),
    ).toBe('Last run: 5m ago');
  });

  it('reports hours when under a day ago', () => {
    expect(
      formatLastRun(new Date(NOW - 3 * 60 * 60_000).toISOString(), NOW),
    ).toBe('Last run: 3h ago');
  });

  it('reports days when older than a day', () => {
    expect(
      formatLastRun(
        new Date(NOW - 2 * 24 * 60 * 60_000).toISOString(),
        NOW,
      ),
    ).toBe('Last run: 2d ago');
  });
});
