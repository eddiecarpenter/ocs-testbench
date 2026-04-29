/**
 * Tests for the run-table pure helpers.
 *
 * Covers AC-9 (sort order), AC-10 (continuous progress), AC-11
 * (interactive progress) and the duration formatter.
 */
import { describe, expect, it } from 'vitest';

import type { ExecutionSummary } from '../../api/resources/executions';

import {
  formatDuration,
  formatProgress,
  groupByBatch,
  isTerminal,
  modeLabel,
  sortByStartedDesc,
} from './runTableHelpers';

function exec(overrides: Partial<ExecutionSummary>): ExecutionSummary {
  return {
    id: 'e',
    scenarioId: 'scn-a',
    scenarioName: 'Scenario A',
    mode: 'continuous',
    state: 'success',
    startedAt: '2026-04-28T10:00:00Z',
    ...overrides,
  };
}

describe('sortByStartedDesc', () => {
  it('sorts newest first and does not mutate the input', () => {
    const input = [
      exec({ id: 'a', startedAt: '2026-04-28T10:00:00Z' }),
      exec({ id: 'b', startedAt: '2026-04-29T10:00:00Z' }),
      exec({ id: 'c', startedAt: '2026-04-28T11:00:00Z' }),
    ];
    const out = sortByStartedDesc(input);
    expect(out.map((e) => e.id)).toEqual(['b', 'c', 'a']);
    expect(input.map((e) => e.id)).toEqual(['a', 'b', 'c']);
  });
});

describe('groupByBatch', () => {
  it('groups rows by batchId', () => {
    const rows = [
      exec({ id: '1', batchId: 'b-1' }),
      exec({ id: '2', batchId: 'b-1' }),
      exec({ id: '3', batchId: 'b-2' }),
      exec({ id: '4' }), // no batchId — skipped
    ];
    const map = groupByBatch(rows);
    expect(map.get('b-1')?.length).toBe(2);
    expect(map.get('b-2')?.length).toBe(1);
    expect(map.size).toBe(2);
  });
});

describe('formatProgress', () => {
  // Default `scn-a` is not in the fixture set so the lookup falls
  // back to the first scenario (session-mode → 3 steps). Tests below
  // pin that contract.
  it('renders interactive terminal as 3 / 3 (session-mode default)', () => {
    expect(
      formatProgress(
        exec({ mode: 'interactive', state: 'success' }),
        new Map(),
      ),
    ).toBe('3 / 3');
  });

  it('renders interactive running as `… / 3` (session-mode default)', () => {
    expect(
      formatProgress(
        exec({ mode: 'interactive', state: 'running' }),
        new Map(),
      ),
    ).toBe('… / 3');
  });

  it('renders continuous standalone (no batch) terminal as 1 / 1', () => {
    expect(formatProgress(exec({ state: 'success' }), new Map())).toBe(
      '1 / 1',
    );
  });

  it('renders continuous standalone running as 0 / 1', () => {
    expect(formatProgress(exec({ state: 'running' }), new Map())).toBe(
      '0 / 1',
    );
  });

  it('renders continuous batched as <terminal> / <total>', () => {
    const siblings = [
      exec({ id: '1', batchId: 'b', state: 'success' }),
      exec({ id: '2', batchId: 'b', state: 'success' }),
      exec({ id: '3', batchId: 'b', state: 'running' }),
      exec({ id: '4', batchId: 'b', state: 'failure' }),
    ];
    const map = groupByBatch(siblings);
    expect(formatProgress(siblings[0], map)).toBe('3 / 4');
  });
});

describe('formatDuration', () => {
  it('formats finished runs as mm:ss', () => {
    expect(
      formatDuration(
        exec({
          startedAt: '2026-04-28T10:00:00Z',
          finishedAt: '2026-04-28T10:01:23Z',
        }),
      ),
    ).toBe('1:23');
  });

  it('zero-pads single-digit seconds', () => {
    expect(
      formatDuration(
        exec({
          startedAt: '2026-04-28T10:00:00Z',
          finishedAt: '2026-04-28T10:00:05Z',
        }),
      ),
    ).toBe('0:05');
  });

  it('returns `–` for runs that never finished', () => {
    expect(formatDuration(exec({ finishedAt: undefined }))).toBe('–');
  });
});

describe('modeLabel / isTerminal', () => {
  it('maps modes to user-facing labels', () => {
    expect(modeLabel('interactive')).toBe('Interactive');
    expect(modeLabel('continuous')).toBe('Continuous');
  });

  it('treats success/failure/aborted/error as terminal', () => {
    expect(isTerminal('success')).toBe(true);
    expect(isTerminal('failure')).toBe(true);
    expect(isTerminal('aborted')).toBe(true);
    expect(isTerminal('error')).toBe(true);
    expect(isTerminal('running')).toBe(false);
    expect(isTerminal('paused')).toBe(false);
    expect(isTerminal('pending')).toBe(false);
  });
});
