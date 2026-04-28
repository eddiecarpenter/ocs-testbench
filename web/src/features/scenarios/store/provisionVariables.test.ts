/**
 * Tests for the auto-provisioning naming convention.
 *
 * Covers AC-32 from Feature #77 — names are flat for `root` /
 * `single-mscc` and `RG<rg>_…` for `multi-mscc`.
 */
import { describe, expect, it } from 'vitest';

import { provisionVariables } from './provisionVariables';
import type { Scenario } from './types';

function baseScenario(overrides: Partial<Scenario> = {}): Scenario {
  return {
    id: 'scn',
    name: 'test',
    description: '',
    unitType: 'OCTET',
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
      unitType: 'TIME',
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
      unitType: 'OCTET',
      serviceModel: 'single-mscc',
      services: [
        {
          id: '100',
          ratingGroup: 'RATING_GROUP',
          requestedUnits: 'RSU_TOTAL',
          usedUnits: 'USU_TOTAL',
        },
      ],
    });
    const next = provisionVariables(s);
    const names = next.map((v) => v.name);
    expect(names).toEqual(
      expect.arrayContaining(['RATING_GROUP', 'RSU_TOTAL', 'USU_TOTAL']),
    );
    expect(names.some((n) => n.startsWith('RG'))).toBe(false);
  });

  it('uses RG<rg>_ prefix for every multi-mscc service', () => {
    const s = baseScenario({
      unitType: 'OCTET',
      serviceModel: 'multi-mscc',
      services: [
        {
          id: '100',
          ratingGroup: 'RG100_RATING_GROUP',
          requestedUnits: 'RG100_RSU_TOTAL',
          usedUnits: 'RG100_USU_TOTAL',
        },
        {
          id: '200',
          ratingGroup: 'RG200_RATING_GROUP',
          requestedUnits: 'RG200_RSU_TOTAL',
          usedUnits: 'RG200_USU_TOTAL',
        },
      ],
    });
    const next = provisionVariables(s);
    const names = next.map((v) => v.name);
    expect(names).toEqual(
      expect.arrayContaining([
        'RG100_RATING_GROUP',
        'RG100_RSU_TOTAL',
        'RG100_USU_TOTAL',
        'RG200_RATING_GROUP',
        'RG200_RSU_TOTAL',
        'RG200_USU_TOTAL',
      ]),
    );
  });

  it('does not overwrite an already-declared variable', () => {
    const s = baseScenario({
      unitType: 'OCTET',
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
