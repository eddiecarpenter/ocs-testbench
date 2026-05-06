/**
 * Tests for list grouping and filter helpers.
 *
 * Covers AC-1 / AC-2 / AC-3 from Feature #77 — grouping by service type,
 * case-insensitive search across name + peer, and peer filter.
 */
import { describe, expect, it } from 'vitest';

import { filterScenarios, groupByServiceType } from './listSelectors';
import type { ScenarioSummary } from './types';

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
    stepCount: 1,
    updatedAt: '2026-04-28T10:00:00Z',
    ...overrides,
  };
}

describe('groupByServiceType', () => {
  it('groups rows under each service type in stable order', () => {
    const rows = [
      row({ id: 'a', serviceType: 'DATA' }),
      row({ id: 'b', serviceType: 'VOICE' }),
      row({ id: 'c', serviceType: 'SMS' }),
      row({ id: 'd', serviceType: 'VOICE' }),
    ];
    const out = groupByServiceType(rows);
    expect(out.VOICE.map((r) => r.id)).toEqual(['b', 'd']);
    expect(out.DATA.map((r) => r.id)).toEqual(['a']);
    expect(out.SMS.map((r) => r.id)).toEqual(['c']);
  });

  it('returns empty arrays for groups with no rows', () => {
    const out = groupByServiceType([row({ serviceType: 'VOICE' })]);
    expect(out.DATA).toEqual([]);
    expect(out.SMS).toEqual([]);
  });
});

describe('filterScenarios', () => {
  const rows = [
    row({ id: 'a', name: 'Voice call charging', peerId: 'peer-01' }),
    row({ id: 'b', name: 'Data session', peerId: 'peer-02' }),
    row({ id: 'c', name: 'SMS burst', peerId: 'peer-01' }),
  ];
  const peerNameById = new Map([
    ['peer-01', 'Acme East'],
    ['peer-02', 'Acme West'],
  ]);

  it('matches case-insensitively across name', () => {
    const out = filterScenarios(rows, {
      search: 'voice',
      peerFilter: null,
      peerNameById,
    });
    expect(out.map((r) => r.id)).toEqual(['a']);
  });

  it('matches case-insensitively across peer name', () => {
    const out = filterScenarios(rows, {
      search: 'east',
      peerFilter: null,
      peerNameById,
    });
    expect(out.map((r) => r.id).sort()).toEqual(['a', 'c']);
  });

  it('peerFilter narrows by peerId', () => {
    const out = filterScenarios(rows, {
      search: '',
      peerFilter: 'peer-01',
      peerNameById,
    });
    expect(out.map((r) => r.id)).toEqual(['a', 'c']);
  });

  it('returns all rows when search and peerFilter are both empty', () => {
    const out = filterScenarios(rows, {
      search: '',
      peerFilter: null,
      peerNameById,
    });
    expect(out).toHaveLength(rows.length);
  });

  it('search and peer filter compose (AND, not OR)', () => {
    const out = filterScenarios(rows, {
      search: 'sms',
      peerFilter: 'peer-02',
      peerNameById,
    });
    expect(out).toEqual([]);
  });
});
