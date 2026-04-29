/**
 * Tests for the Executions page selectors.
 *
 * Covers AC-1 / AC-3 / AC-4:
 *   - sidebar run-count aggregate
 *   - last-run timestamp lookup
 *   - case-insensitive name filter
 *   - right-pane header lookup
 */
import { describe, expect, it } from 'vitest';

import type { ExecutionSummary } from '../../api/resources/executions';
import type { ScenarioSummary } from '../scenarios/types';

import {
  applyTableFilters,
  countByStatusFilter,
  countRunsByScenario,
  filterScenariosByName,
  lastRunByScenario,
  parseStatusFilter,
  selectLatestRunForScenario,
  selectScenarioForHeader,
  statusFilterMatches,
} from './selectors';

function row(overrides: Partial<ScenarioSummary>): ScenarioSummary {
  return {
    id: 'scn',
    name: 'name',
    description: '',
    unitType: 'OCTET',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    origin: 'user',
    favourite: false,
    subscriberId: 'sub-001',
    peerId: 'peer-01',
    stepCount: 1,
    updatedAt: '2026-04-28T10:00:00Z',
    ...overrides,
  };
}

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

describe('selectScenarioForHeader', () => {
  const scenarios = [
    row({ id: 'a', name: 'Alpha' }),
    row({ id: 'b', name: 'Bravo' }),
  ];

  it('returns the matching scenario when id is set', () => {
    expect(selectScenarioForHeader(scenarios, 'b')?.name).toBe('Bravo');
  });

  it('returns undefined when id is null (All runs view)', () => {
    expect(selectScenarioForHeader(scenarios, null)).toBeUndefined();
  });

  it('returns undefined when id is empty string', () => {
    expect(selectScenarioForHeader(scenarios, '')).toBeUndefined();
  });

  it('returns undefined when id is stale / unknown', () => {
    expect(selectScenarioForHeader(scenarios, 'gone')).toBeUndefined();
  });

  it('returns undefined when the list is empty', () => {
    expect(selectScenarioForHeader([], 'a')).toBeUndefined();
  });
});

describe('countRunsByScenario', () => {
  it('groups counts per scenarioId', () => {
    const counts = countRunsByScenario([
      exec({ id: '1', scenarioId: 'a' }),
      exec({ id: '2', scenarioId: 'a' }),
      exec({ id: '3', scenarioId: 'b' }),
    ]);
    expect(counts.get('a')).toBe(2);
    expect(counts.get('b')).toBe(1);
  });

  it('omits scenarios with zero runs', () => {
    const counts = countRunsByScenario([exec({ scenarioId: 'a' })]);
    expect(counts.has('b')).toBe(false);
  });

  it('returns an empty map for empty input', () => {
    expect(countRunsByScenario([]).size).toBe(0);
  });
});

describe('lastRunByScenario', () => {
  it('returns the latest startedAt per scenario', () => {
    const last = lastRunByScenario([
      exec({ id: '1', scenarioId: 'a', startedAt: '2026-04-28T10:00:00Z' }),
      exec({ id: '2', scenarioId: 'a', startedAt: '2026-04-29T11:00:00Z' }),
      exec({ id: '3', scenarioId: 'a', startedAt: '2026-04-29T10:00:00Z' }),
      exec({ id: '4', scenarioId: 'b', startedAt: '2026-04-28T09:00:00Z' }),
    ]);
    expect(last.get('a')).toBe('2026-04-29T11:00:00Z');
    expect(last.get('b')).toBe('2026-04-28T09:00:00Z');
  });

  it('returns an empty map for empty input', () => {
    expect(lastRunByScenario([]).size).toBe(0);
  });
});

describe('parseStatusFilter', () => {
  it('returns the typed filter for valid params', () => {
    expect(parseStatusFilter('running')).toBe('running');
    expect(parseStatusFilter('completed')).toBe('completed');
    expect(parseStatusFilter('failed')).toBe('failed');
    expect(parseStatusFilter('all')).toBe('all');
  });

  it('falls back to "all" for unknown / missing values', () => {
    expect(parseStatusFilter(null)).toBe('all');
    expect(parseStatusFilter('')).toBe('all');
    expect(parseStatusFilter('garbage')).toBe('all');
  });
});

describe('statusFilterMatches', () => {
  it('all matches every state', () => {
    for (const state of ['running', 'success', 'failure', 'aborted'] as const) {
      expect(statusFilterMatches('all', state)).toBe(true);
    }
  });

  it('running matches running / paused / pending', () => {
    expect(statusFilterMatches('running', 'running')).toBe(true);
    expect(statusFilterMatches('running', 'paused')).toBe(true);
    expect(statusFilterMatches('running', 'pending')).toBe(true);
    expect(statusFilterMatches('running', 'success')).toBe(false);
  });

  it('completed maps to success only', () => {
    expect(statusFilterMatches('completed', 'success')).toBe(true);
    expect(statusFilterMatches('completed', 'failure')).toBe(false);
  });

  it('failed maps to failure / error / aborted', () => {
    expect(statusFilterMatches('failed', 'failure')).toBe(true);
    expect(statusFilterMatches('failed', 'error')).toBe(true);
    expect(statusFilterMatches('failed', 'aborted')).toBe(true);
    expect(statusFilterMatches('failed', 'success')).toBe(false);
  });
});

describe('countByStatusFilter', () => {
  it('counts each bucket independently', () => {
    const counts = countByStatusFilter([
      exec({ state: 'running' }),
      exec({ state: 'paused' }),
      exec({ state: 'success' }),
      exec({ state: 'failure' }),
      exec({ state: 'aborted' }),
    ]);
    expect(counts.all).toBe(5);
    expect(counts.running).toBe(2); // running + paused
    expect(counts.completed).toBe(1);
    expect(counts.failed).toBe(2); // failure + aborted
  });

  it('returns all-zero buckets for empty input', () => {
    const counts = countByStatusFilter([]);
    expect(counts).toEqual({ all: 0, running: 0, completed: 0, failed: 0 });
  });
});

describe('applyTableFilters', () => {
  const rows = [
    exec({ id: '1', state: 'running', peerId: 'peer-01' }),
    exec({ id: '2', state: 'success', peerId: 'peer-01' }),
    exec({ id: '3', state: 'failure', peerId: 'peer-02' }),
    exec({ id: '4', state: 'success', peerId: 'peer-02' }),
  ];

  it('filters by status', () => {
    const out = applyTableFilters(rows, { status: 'completed' });
    expect(out.map((r) => r.id)).toEqual(['2', '4']);
  });

  it('returns the input set when status is "all"', () => {
    const out = applyTableFilters(rows, { status: 'all' });
    expect(out).toHaveLength(rows.length);
  });
});

describe('selectLatestRunForScenario', () => {
  it('returns the highest startedAt for the scenario', () => {
    const rows = [
      exec({ id: '1', scenarioId: 'a', startedAt: '2026-04-28T10:00:00Z' }),
      exec({ id: '2', scenarioId: 'a', startedAt: '2026-04-29T10:00:00Z' }),
      exec({ id: '3', scenarioId: 'b', startedAt: '2026-04-29T11:00:00Z' }),
    ];
    expect(selectLatestRunForScenario(rows, 'a')?.id).toBe('2');
  });

  it('returns undefined for a scenario with no runs', () => {
    expect(
      selectLatestRunForScenario([exec({ scenarioId: 'a' })], 'b'),
    ).toBeUndefined();
  });
});

describe('filterScenariosByName', () => {
  const scenarios = [
    row({ id: 'a', name: 'Voice call charging' }),
    row({ id: 'b', name: 'Data session — happy path' }),
    row({ id: 'c', name: 'SMS burst load' }),
  ];

  it('returns all rows when search is empty', () => {
    expect(filterScenariosByName(scenarios, '')).toHaveLength(3);
  });

  it('matches case-insensitively', () => {
    const out = filterScenariosByName(scenarios, 'VOICE');
    expect(out.map((r) => r.id)).toEqual(['a']);
  });

  it('treats whitespace-only as empty', () => {
    expect(filterScenariosByName(scenarios, '   ')).toHaveLength(3);
  });

  it('matches a substring anywhere in the name', () => {
    const out = filterScenariosByName(scenarios, 'session');
    expect(out.map((r) => r.id)).toEqual(['b']);
  });

  it('preserves the input order', () => {
    const out = filterScenariosByName(scenarios, ' ');
    expect(out.map((r) => r.id)).toEqual(['a', 'b', 'c']);
  });
});
