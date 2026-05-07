/**
 * Tests for the auto-provisioning naming convention.
 *
 * Covers AC-32 from Feature #77 — names are flat for `root` /
 * `single-mscc` and `RG<pos>_…` (1-based position) for `multi-mscc`.
 */
import { describe, expect, it } from 'vitest';

import { provisionVariables } from './provisionVariables';
import type { Scenario } from './types';

function baseScenario(overrides: Partial<Scenario> = {}): Scenario {
  return {
    id: 'scn',
    name: 'test',
    description: '',
    serviceType: 'DATA',
    serviceProfile: '3GPP',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    origin: 'user',
    favourite: false,
    subscriberId: 'sub-001',
    peerId: 'peer-01',
    stepCount: 0,
    updatedAt: '2026-04-28T10:00:00Z',
    avpTree: [],
    services: [],
    variables: [],
    steps: [],
    ...overrides,
  };
}

describe('provisionVariables', () => {
  it('uses flat names for root scenarios (RSU_TOTAL, USU_TOTAL)', () => {
    const s = baseScenario({
      serviceType: 'VOICE',
      serviceModel: 'root',
      services: [{ id: 'root', requestedUnits: 'RSU_TOTAL', usedUnits: 'USU_TOTAL' }],
    });
    const next = provisionVariables(s);
    const names = next.map((v) => v.name);
    expect(names).toEqual(expect.arrayContaining(['RSU_TOTAL', 'USU_TOTAL']));
    expect(names.some((n) => n.startsWith('RG'))).toBe(false);
  });

  it('uses flat names for single-mscc scenarios', () => {
    const s = baseScenario({
      serviceType: 'DATA',
      serviceModel: 'single-mscc',
      services: [
        {
          id: '100',
          ratingGroup: 'RATING_GROUP',
          requestedUnits: 'UNITS_REQ',
          usedUnits: 'UNITS_USED',
        },
      ],
    });
    const next = provisionVariables(s);
    const names = next.map((v) => v.name);
    expect(names).toEqual(
      expect.arrayContaining(['RATING_GROUP', 'UNITS_REQ', 'UNITS_USED']),
    );
    expect(names.some((n) => n.startsWith('RG'))).toBe(false);
  });

  it('uses RG<pos>_ prefix (1-based position) for every multi-mscc service', () => {
    const s = baseScenario({
      serviceType: 'DATA',
      serviceModel: 'multi-mscc',
      services: [
        {
          id: '100',
          ratingGroup: 'RG1_RATING_GROUP',
          requestedUnits: 'RG1_UNITS_REQ',
          usedUnits: 'RG1_UNITS_USED',
        },
        {
          id: '200',
          ratingGroup: 'RG2_RATING_GROUP',
          requestedUnits: 'RG2_UNITS_REQ',
          usedUnits: 'RG2_UNITS_USED',
        },
      ],
    });
    const next = provisionVariables(s);
    const names = next.map((v) => v.name);
    expect(names).toEqual(
      expect.arrayContaining([
        'RG1_RATING_GROUP',
        'RG1_UNITS_REQ',
        'RG1_UNITS_USED',
        'RG2_RATING_GROUP',
        'RG2_UNITS_REQ',
        'RG2_UNITS_USED',
      ]),
    );
  });

  it('does not overwrite an already-declared variable', () => {
    const s = baseScenario({
      serviceType: 'DATA',
      serviceModel: 'single-mscc',
      services: [
        {
          id: '100',
          ratingGroup: 'RATING_GROUP',
          requestedUnits: 'RSU_TOTAL',
        },
      ],
      variables: [
        {
          name: 'RATING_GROUP',
          source: {
            kind: 'generator',
            strategy: 'literal',
            refresh: 'once',
            params: { value: 42 },
          },
        },
      ],
    });
    const next = provisionVariables(s);
    const rg = next.find((v) => v.name === 'RATING_GROUP');
    expect(rg?.source.kind).toBe('generator');
    if (rg?.source.kind === 'generator') {
      expect(rg.source.params?.value).toBe(42);
    }
  });
});
