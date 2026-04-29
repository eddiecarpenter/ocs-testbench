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
  countRunsByScenario,
  filterScenariosByName,
  lastRunByScenario,
  selectScenarioForHeader,
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
