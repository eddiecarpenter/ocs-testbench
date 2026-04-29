/**
 * Tests for the CCR preview resolver.
 *
 * Covers:
 *   - root step variable substitution
 *   - single-mscc step variable substitution + structural integrity
 *   - multi-mscc step services-enabled filter (none / some / all)
 *   - regenerate trace: changing a variable value produces a new tree
 *   - structural immutability: a deep-frozen scenario in still
 *     produces a fresh tree out
 *
 * The resolver is pure — no React, no Zustand. Tests construct
 * minimal scenarios directly from the schema-sourced types.
 */
import { describe, expect, it } from 'vitest';

import type {
  AvpNode,
  Scenario,
  Service,
  Variable,
} from '../scenarios/types';

import { resolveCcrPreview, type ContextVars } from './ccrPreview';

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

const baseAvpTree: AvpNode[] = [
  { name: 'Origin-Host', code: 264, valueRef: 'ORIGIN_HOST' },
  { name: 'Origin-Realm', code: 296, valueRef: 'ORIGIN_REALM' },
  {
    name: 'Subscription-Id',
    code: 443,
    children: [
      { name: 'Subscription-Id-Type', code: 450, valueRef: 'SUB_ID_TYPE' },
      { name: 'Subscription-Id-Data', code: 444, valueRef: 'MSISDN' },
    ],
  },
];

const literal: Variable['source'] = {
  kind: 'generator',
  strategy: 'literal',
  refresh: 'once',
  params: { value: 1 },
};

function buildScenario(opts: {
  serviceModel: Scenario['serviceModel'];
  services: Service[];
  unitType?: Scenario['unitType'];
}): Scenario {
  return {
    id: 'scn-test',
    name: 'Test scenario',
    description: 'Test',
    unitType: opts.unitType ?? 'OCTET',
    sessionMode: 'session',
    serviceModel: opts.serviceModel,
    origin: 'user',
    favourite: false,
    subscriberId: 'sub-1',
    peerId: 'peer-1',
    stepCount: 1,
    updatedAt: '2026-04-29T07:00:00Z',
    avpTree: baseAvpTree,
    services: opts.services,
    variables: [
      { name: 'ORIGIN_HOST', source: literal },
      { name: 'ORIGIN_REALM', source: literal },
      { name: 'SUB_ID_TYPE', source: literal },
      { name: 'MSISDN', source: literal },
    ],
    steps: [{ kind: 'request', requestType: 'INITIAL' }],
  };
}

const ROOT_SERVICE: Service = {
  id: 'root',
  requestedUnits: 'RSU_TOTAL',
};

const SINGLE_MSCC_SERVICE: Service = {
  id: '100',
  ratingGroup: 'RATING_GROUP',
  serviceIdentifier: 'SERVICE_ID',
  requestedUnits: 'RSU_TOTAL',
  usedUnits: 'USU_TOTAL',
};

const MULTI_MSCC_SERVICES: Service[] = [
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
];

const FULL_CONTEXT: ContextVars = {
  ORIGIN_HOST: 'ocs.example.com',
  ORIGIN_REALM: 'example.com',
  SUB_ID_TYPE: '0',
  MSISDN: '27821234567',
  RSU_TOTAL: 1_048_576,
  USU_TOTAL: 524_288,
  RATING_GROUP: 100,
  SERVICE_ID: 1,
  RG100_RATING_GROUP: 100,
  RG100_RSU_TOTAL: 1_048_576,
  RG100_USU_TOTAL: 524_288,
  RG200_RATING_GROUP: 200,
  RG200_RSU_TOTAL: 524_288,
  RG200_USU_TOTAL: 262_144,
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('resolveCcrPreview — root scenarios', () => {
  it('substitutes variables from the context', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [ROOT_SERVICE],
      unitType: 'TIME',
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    const originHost = tree.find((n) => n.name === 'Origin-Host');
    expect(originHost?.value).toBe('ocs.example.com');
    expect(originHost?.valueRef).toBe('ORIGIN_HOST');

    // RSU spliced at root
    const rsu = tree.find((n) => n.name === 'Requested-Service-Unit');
    expect(rsu).toBeDefined();
    expect(rsu?.value).toBe('1048576');
  });

  it('falls back to {{name}} when a variable is unbound', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [ROOT_SERVICE],
      unitType: 'TIME',
    });
    const ctx: ContextVars = { MSISDN: '27821234567' };
    const tree = resolveCcrPreview(scn, 0, ctx, new Set());
    const originHost = tree.find((n) => n.name === 'Origin-Host');
    expect(originHost?.value).toBe('{{ORIGIN_HOST}}');
    const subId = tree.find((n) => n.name === 'Subscription-Id');
    const dataLeaf = subId?.children?.find(
      (c) => c.name === 'Subscription-Id-Data',
    );
    expect(dataLeaf?.value).toBe('27821234567');
  });

  it('does not emit MSI or MSCC for root mode', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [ROOT_SERVICE],
      unitType: 'TIME',
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    expect(
      tree.find((n) => n.name === 'Multiple-Services-Indicator'),
    ).toBeUndefined();
    expect(
      tree.find((n) => n.name === 'Multiple-Services-Credit-Control'),
    ).toBeUndefined();
  });
});

describe('resolveCcrPreview — single-mscc scenarios', () => {
  it('emits exactly one MSCC block with MSI=0 and resolved leaves', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [SINGLE_MSCC_SERVICE],
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    const msi = tree.find((n) => n.name === 'Multiple-Services-Indicator');
    expect(msi?.value).toBe('0');

    const msccs = tree.filter(
      (n) => n.name === 'Multiple-Services-Credit-Control',
    );
    expect(msccs).toHaveLength(1);

    const mscc = msccs[0];
    const ratingGroup = mscc.children?.find((c) => c.name === 'Rating-Group');
    expect(ratingGroup?.value).toBe('100');
    expect(ratingGroup?.valueRef).toBe('RATING_GROUP');

    const rsu = mscc.children?.find(
      (c) => c.name === 'Requested-Service-Unit',
    );
    expect(rsu?.value).toBe('1048576');
    const usu = mscc.children?.find((c) => c.name === 'Used-Service-Unit');
    expect(usu?.value).toBe('524288');
  });

  it('preserves root AVP-tree structure (Subscription-Id grouping)', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [SINGLE_MSCC_SERVICE],
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    const subId = tree.find((n) => n.name === 'Subscription-Id');
    expect(subId).toBeDefined();
    expect(subId?.children).toHaveLength(2);
    expect(subId?.children?.[0].name).toBe('Subscription-Id-Type');
    expect(subId?.children?.[1].name).toBe('Subscription-Id-Data');
  });
});

describe('resolveCcrPreview — multi-mscc scenarios', () => {
  it('emits MSI=1 and one MSCC per enabled service', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: MULTI_MSCC_SERVICES,
    });
    const tree = resolveCcrPreview(
      scn,
      0,
      FULL_CONTEXT,
      new Set(['100', '200']),
    );
    const msi = tree.find((n) => n.name === 'Multiple-Services-Indicator');
    expect(msi?.value).toBe('1');
    const msccs = tree.filter(
      (n) => n.name === 'Multiple-Services-Credit-Control',
    );
    expect(msccs).toHaveLength(2);
  });

  it('servicesEnabled filter — only the selected services appear', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: MULTI_MSCC_SERVICES,
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set(['100']));
    const msccs = tree.filter(
      (n) => n.name === 'Multiple-Services-Credit-Control',
    );
    expect(msccs).toHaveLength(1);
    const ratingGroup = msccs[0].children?.find(
      (c) => c.name === 'Rating-Group',
    );
    expect(ratingGroup?.value).toBe('100');
  });

  it('servicesEnabled empty — MSI=1 still emitted but no MSCCs', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: MULTI_MSCC_SERVICES,
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    expect(
      tree.find((n) => n.name === 'Multiple-Services-Indicator')?.value,
    ).toBe('1');
    expect(
      tree.filter((n) => n.name === 'Multiple-Services-Credit-Control'),
    ).toHaveLength(0);
  });
});

describe('resolveCcrPreview — regenerate trace', () => {
  it('changing a variable value produces a new tree with the new value', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [SINGLE_MSCC_SERVICE],
    });
    const before = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    const after = resolveCcrPreview(
      scn,
      0,
      { ...FULL_CONTEXT, RSU_TOTAL: 9_999_999 },
      new Set(),
    );

    function rsuValue(tree: ReturnType<typeof resolveCcrPreview>): string | undefined {
      const mscc = tree.find(
        (n) => n.name === 'Multiple-Services-Credit-Control',
      );
      return mscc?.children?.find((c) => c.name === 'Requested-Service-Unit')
        ?.value;
    }

    expect(rsuValue(before)).toBe('1048576');
    expect(rsuValue(after)).toBe('9999999');
  });

  it('toggling a service updates the resolved tree', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: MULTI_MSCC_SERVICES,
    });
    const before = resolveCcrPreview(
      scn,
      0,
      FULL_CONTEXT,
      new Set(['100', '200']),
    );
    const after = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set(['100']));
    expect(
      before.filter((n) => n.name === 'Multiple-Services-Credit-Control'),
    ).toHaveLength(2);
    expect(
      after.filter((n) => n.name === 'Multiple-Services-Credit-Control'),
    ).toHaveLength(1);
  });
});

describe('resolveCcrPreview — structural immutability', () => {
  it('a deep-frozen scenario still produces a fresh tree', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [SINGLE_MSCC_SERVICE],
    });
    deepFreeze(scn);
    expect(() => resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set())).not.toThrow();
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    // The output must be a fresh array, not the same reference.
    expect(tree).not.toBe(scn.avpTree);
  });

  it('mutating the resolved tree does not affect the input scenario', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [SINGLE_MSCC_SERVICE],
    });
    const tree = resolveCcrPreview(scn, 0, FULL_CONTEXT, new Set());
    // Mutate the resolved tree.
    tree[0].name = 'Tampered';
    expect(scn.avpTree[0].name).not.toBe('Tampered');
  });
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function deepFreeze<T>(obj: T): T {
  if (obj === null || typeof obj !== 'object') return obj;
  Object.freeze(obj);
  for (const key of Object.keys(obj as object)) {
    const v = (obj as Record<string, unknown>)[key];
    if (v && typeof v === 'object' && !Object.isFrozen(v)) deepFreeze(v);
  }
  return obj;
}
