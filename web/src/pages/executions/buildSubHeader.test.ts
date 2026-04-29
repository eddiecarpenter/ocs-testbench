/**
 * Tests for the Executions header sub-header builder.
 *
 * Covers AC-3 — sub-header format: `<unit-type>-session · <peer> ·
 * <step-count> steps`.
 */
import { describe, expect, it } from 'vitest';

import type { ScenarioSummary } from '../scenarios/types';

import { buildSubHeader } from './buildSubHeader';

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
    stepCount: 4,
    updatedAt: '2026-04-28T10:00:00Z',
    ...overrides,
  };
}

describe('buildSubHeader', () => {
  it('joins unit-session, peer name, and step count with bullet separators', () => {
    const peerNameById = new Map([['peer-01', 'Acme East']]);
    expect(buildSubHeader(row({}), peerNameById)).toBe(
      'octet-session · Acme East · 4 steps',
    );
  });

  it('falls back to the peer id when the peer query has not resolved', () => {
    const peerNameById = new Map<string, string>();
    expect(buildSubHeader(row({ peerId: 'peer-99' }), peerNameById)).toBe(
      'octet-session · peer-99 · 4 steps',
    );
  });

  it('shows "no peer" when the scenario has no peer assigned', () => {
    expect(buildSubHeader(row({ peerId: undefined }), new Map())).toBe(
      'octet-session · no peer · 4 steps',
    );
  });

  it('singularises the step count word at 1 step', () => {
    expect(buildSubHeader(row({ stepCount: 1 }), new Map())).toBe(
      'octet-session · peer-01 · 1 step',
    );
  });

  it('lower-cases TIME / UNITS unit-types', () => {
    expect(buildSubHeader(row({ unitType: 'TIME' }), new Map())).toBe(
      'time-session · peer-01 · 4 steps',
    );
    expect(buildSubHeader(row({ unitType: 'UNITS' }), new Map())).toBe(
      'units-session · peer-01 · 4 steps',
    );
  });
});
