/**
 * Tests for the optimistic-prepend cache helper used by the
 * Start-Run dialog's continuous-mode submit path (AC-17).
 */
import { describe, expect, it } from 'vitest';

import type {
  ExecutionPage,
  ExecutionSummary,
} from '../../api/resources/executions';

import { prependExecutions } from './optimisticPrepend';

function exec(overrides: Partial<ExecutionSummary>): ExecutionSummary {
  return {
    id: 'e',
    scenarioId: 'scn',
    scenarioName: 'Scenario',
    mode: 'continuous',
    state: 'running',
    startedAt: '2026-04-29T10:00:00Z',
    ...overrides,
  };
}

function page(items: ExecutionSummary[]): ExecutionPage {
  return { items, page: { total: items.length, limit: 50, offset: 0 } };
}

describe('prependExecutions', () => {
  it('returns undefined when prev is undefined', () => {
    expect(prependExecutions(undefined, [exec({})])).toBeUndefined();
  });

  it('returns prev unchanged when created is empty', () => {
    const prev = page([exec({ id: '1' })]);
    expect(prependExecutions(prev, [])).toBe(prev);
  });

  it('prepends created rows ahead of existing items', () => {
    const prev = page([exec({ id: '1' })]);
    const out = prependExecutions(prev, [exec({ id: 'new-1' })]);
    expect(out?.items.map((e) => e.id)).toEqual(['new-1', '1']);
  });

  it('increments the total by the number of created rows', () => {
    const prev = page([exec({ id: '1' }), exec({ id: '2' })]);
    const out = prependExecutions(prev, [
      exec({ id: 'a' }),
      exec({ id: 'b' }),
    ]);
    expect(out?.page.total).toBe(4);
    expect(out?.page.limit).toBe(prev.page.limit);
    expect(out?.page.offset).toBe(prev.page.offset);
  });

  it('does not mutate prev', () => {
    const prev = page([exec({ id: '1' })]);
    prependExecutions(prev, [exec({ id: 'new-1' })]);
    expect(prev.items.map((e) => e.id)).toEqual(['1']);
    expect(prev.page.total).toBe(1);
  });
});
