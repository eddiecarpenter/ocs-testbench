/**
 * Tests for list grouping and filter helpers.
 *
 * Covers AC-1 / AC-2 / AC-3 from Feature #77 — grouping by unit type,
 * case-insensitive search across name + peer, and peer filter.
 */
import { describe, expect, it } from 'vitest';

import { filterScenarios, groupByUnit } from './listSelectors';
import type { ScenarioSummary } from '../store/types';

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

describe('groupByUnit', () => {
  it('groups rows under OCTET / TIME / UNITS in stable order', () => {
    const rows = [
      row({ id: 'a', unitType: 'TIME' }),
      row({ id: 'b', unitType: 'OCTET' }),
      row({ id: 'c', unitType: 'UNITS' }),
      row({ id: 'd', unitType: 'OCTET' }),
    ];
    const out = groupByUnit(rows);
    expect(out.OCTET.map((r) => r.id)).toEqual(['b', 'd']);
    expect(out.TIME.map((r) => r.id)).toEqual(['a']);
    expect(out.UNITS.map((r) => r.id)).toEqual(['c']);
  });

  it('returns empty arrays for groups with no rows', () => {
    const out = groupByUnit([row({ unitType: 'OCTET' })]);
    expect(out.TIME).toEqual([]);
    expect(out.UNITS).toEqual([]);
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
