/**
 * Tests for the Executions header sub-header builder.
 *
 * Covers AC-3 — sub-header format: `<service-type>-session · <peer> ·
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
    serviceType: 'VOICE',
    serviceProfile: '3GPP',
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
  it('joins service-type-session, peer name, and step count with bullet separators', () => {
    const peerNameById = new Map([['peer-01', 'Acme East']]);
    expect(buildSubHeader(row({}), peerNameById)).toBe(
      'voice-session · Acme East · 4 steps',
    );
  });

  it('falls back to the peer id when the peer query has not resolved', () => {
    const peerNameById = new Map<string, string>();
    expect(buildSubHeader(row({ peerId: 'peer-99' }), peerNameById)).toBe(
      'voice-session · peer-99 · 4 steps',
    );
  });

  it('shows "no peer" when the scenario has no peer assigned', () => {
    expect(buildSubHeader(row({ peerId: undefined }), new Map())).toBe(
      'voice-session · no peer · 4 steps',
    );
  });

  it('singularises the step count word at 1 step', () => {
    expect(buildSubHeader(row({ stepCount: 1 }), new Map())).toBe(
      'voice-session · peer-01 · 1 step',
    );
  });

  it('lower-cases other service types', () => {
    expect(buildSubHeader(row({ serviceType: 'DATA' }), new Map())).toBe(
      'data-session · peer-01 · 4 steps',
    );
    expect(buildSubHeader(row({ serviceType: 'SMS' }), new Map())).toBe(
      'sms-session · peer-01 · 4 steps',
    );
  });
});
