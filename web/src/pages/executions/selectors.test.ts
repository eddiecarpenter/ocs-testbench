/**
 * Tests for the Executions page selectors.
 *
 * Covers AC-3 — the right-pane header lookup must resolve a scenario
 * by URL `?scenario=<id>` and fall through cleanly when the id is
 * missing or stale.
 */
import { describe, expect, it } from 'vitest';

import type { ScenarioSummary } from '../scenarios/types';

import { selectScenarioForHeader } from './selectors';

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
