/**
 * Pure-logic tests for the Step Editor pane.
 *
 * The pane's React render is exercised through Storybook (out of MVP);
 * these unit tests pin the resolver wiring, the default-services rule,
 * the context flattening, and the step-header rendering.
 */
import { describe, expect, it } from 'vitest';

import type { ExecutionContextSnapshot } from '../../api/resources/executions';
import type {
  AvpNode,
  Scenario,
  Service,
  Variable,
} from '../scenarios/types';

import {
  buildStepHeader,
  defaultServicesForStep,
  flattenContext,
  resolvePreview,
} from './stepEditorLogic';

const literal: Variable['source'] = {
  kind: 'generator',
  strategy: 'literal',
  refresh: 'once',
  params: { value: 1 },
};

const baseAvpTree: AvpNode[] = [
  { name: 'Origin-Host', code: 264, valueRef: 'ORIGIN_HOST' },
];

const SVC_100: Service = {
  id: '100',
  ratingGroup: 'RG100_RG',
  requestedUnits: 'RG100_RSU',
};
const SVC_200: Service = {
  id: '200',
  ratingGroup: 'RG200_RG',
  requestedUnits: 'RG200_RSU',
};
const SVC_300: Service = {
  id: '300',
  ratingGroup: 'RG300_RG',
  requestedUnits: 'RG300_RSU',
};

function buildScenario(opts: {
  serviceModel: Scenario['serviceModel'];
  services: Service[];
  steps: Scenario['steps'];
}): Scenario {
  return {
    id: 'scn-test',
    name: 'Test',
    description: '',
    unitType: 'OCTET',
    sessionMode: 'session',
    serviceModel: opts.serviceModel,
    origin: 'user',
    favourite: false,
    subscriberId: 'sub-1',
    peerId: 'peer-1',
    stepCount: opts.steps.length,
    updatedAt: '2026-04-29T07:00:00Z',
    avpTree: baseAvpTree,
    services: opts.services,
    variables: [{ name: 'ORIGIN_HOST', source: literal }],
    steps: opts.steps,
  };
}

// ---------------------------------------------------------------------------
// defaultServicesForStep
// ---------------------------------------------------------------------------

describe('defaultServicesForStep', () => {
  it('root → all services (singleton) selected', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [{ id: 'root', requestedUnits: 'RSU' }],
      steps: [{ kind: 'request', requestType: 'INITIAL' }],
    });
    expect(defaultServicesForStep(scn, 0)).toEqual(new Set(['root']));
  });

  it('single-mscc → the single service selected', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [SVC_100],
      steps: [{ kind: 'request', requestType: 'INITIAL' }],
    });
    expect(defaultServicesForStep(scn, 0)).toEqual(new Set(['100']));
  });

  it('multi-mscc + fixed selection → just the listed ids', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: [SVC_100, SVC_200, SVC_300],
      steps: [
        {
          kind: 'request',
          requestType: 'INITIAL',
          services: { mode: 'fixed', serviceIds: ['100', '300'] },
        },
      ],
    });
    expect(defaultServicesForStep(scn, 0)).toEqual(new Set(['100', '300']));
  });

  it('multi-mscc + random pool → the candidate pool', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: [SVC_100, SVC_200, SVC_300],
      steps: [
        {
          kind: 'request',
          requestType: 'UPDATE',
          services: { mode: 'random', from: ['100', '200'], count: 1 },
        },
      ],
    });
    expect(defaultServicesForStep(scn, 0)).toEqual(new Set(['100', '200']));
  });

  it('multi-mscc + non-request kind → all services (fallback)', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: [SVC_100, SVC_200],
      steps: [{ kind: 'pause' }],
    });
    expect(defaultServicesForStep(scn, 0)).toEqual(new Set(['100', '200']));
  });
});

// ---------------------------------------------------------------------------
// flattenContext
// ---------------------------------------------------------------------------

describe('flattenContext', () => {
  it('merges all three scopes into a flat map', () => {
    const ctx: ExecutionContextSnapshot = {
      system: { SESSION_ID: 'abc' },
      user: { MSISDN: '27821234567' },
      extracted: { GRANTED_TOTAL: 1024 },
    };
    expect(flattenContext(ctx)).toEqual({
      SESSION_ID: 'abc',
      MSISDN: '27821234567',
      GRANTED_TOTAL: 1024,
    });
  });

  it('user > system > extracted on key collision', () => {
    const ctx: ExecutionContextSnapshot = {
      extracted: { X: 'extracted-wins' },
      system: { X: 'system-wins' },
      user: { X: 'user-wins' },
    };
    expect(flattenContext(ctx)).toEqual({ X: 'user-wins' });
  });
});

// ---------------------------------------------------------------------------
// resolvePreview — wiring against Task 3 resolver
// ---------------------------------------------------------------------------

describe('resolvePreview', () => {
  it('toggling a service produces a tree without that MSCC', () => {
    const scn = buildScenario({
      serviceModel: 'multi-mscc',
      services: [SVC_100, SVC_200],
      steps: [
        {
          kind: 'request',
          requestType: 'UPDATE',
          services: { mode: 'fixed', serviceIds: ['100', '200'] },
        },
      ],
    });
    const ctx = { ORIGIN_HOST: 'host', RG100_RG: 100, RG100_RSU: 1024,
                  RG200_RG: 200, RG200_RSU: 2048 };
    const both = resolvePreview(scn, 0, ctx, new Set(['100', '200']));
    const just100 = resolvePreview(scn, 0, ctx, new Set(['100']));
    expect(
      both.filter((n) => n.name === 'Multiple-Services-Credit-Control'),
    ).toHaveLength(2);
    expect(
      just100.filter((n) => n.name === 'Multiple-Services-Credit-Control'),
    ).toHaveLength(1);
  });

  it('regenerate trace — different contextVars → different value', () => {
    const scn = buildScenario({
      serviceModel: 'single-mscc',
      services: [
        { id: '100', ratingGroup: 'RG', requestedUnits: 'RSU' },
      ],
      steps: [{ kind: 'request', requestType: 'INITIAL' }],
    });
    const before = resolvePreview(
      scn,
      0,
      { ORIGIN_HOST: 'h', RG: 100, RSU: 100 },
      new Set(),
    );
    const after = resolvePreview(
      scn,
      0,
      { ORIGIN_HOST: 'h', RG: 100, RSU: 9999 },
      new Set(),
    );

    function rsu(
      tree: ReturnType<typeof resolvePreview>,
    ): string | undefined {
      const m = tree.find(
        (n) => n.name === 'Multiple-Services-Credit-Control',
      );
      return m?.children?.find((c) => c.name === 'Requested-Service-Unit')
        ?.value;
    }
    expect(rsu(before)).toBe('100');
    expect(rsu(after)).toBe('9999');
  });
});

// ---------------------------------------------------------------------------
// buildStepHeader
// ---------------------------------------------------------------------------

describe('buildStepHeader', () => {
  it('request step renders the request type as a friendly title', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [{ id: 'root', requestedUnits: 'RSU' }],
      steps: [
        { kind: 'request', requestType: 'INITIAL' },
        { kind: 'request', requestType: 'UPDATE' },
        { kind: 'request', requestType: 'TERMINATE' },
        { kind: 'request', requestType: 'EVENT' },
      ],
    });
    expect(buildStepHeader(scn, 0)?.title).toMatch(/CCR-INITIAL/);
    expect(buildStepHeader(scn, 1)?.title).toMatch(/CCR-UPDATE/);
    expect(buildStepHeader(scn, 2)?.title).toMatch(/CCR-TERMINATE/);
    expect(buildStepHeader(scn, 3)?.title).toMatch(/CCR-EVENT/);
  });

  it('non-request kinds get distinctive titles', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [{ id: 'root', requestedUnits: 'RSU' }],
      steps: [
        { kind: 'wait', durationMs: 500 },
        { kind: 'pause', label: 'top-up' },
      ],
    });
    expect(buildStepHeader(scn, 0)?.title).toBe('Wait 500 ms');
    expect(buildStepHeader(scn, 1)?.title).toBe('top-up');
  });

  it('out-of-range step → null', () => {
    const scn = buildScenario({
      serviceModel: 'root',
      services: [{ id: 'root', requestedUnits: 'RSU' }],
      steps: [{ kind: 'request', requestType: 'INITIAL' }],
    });
    expect(buildStepHeader(scn, 99)).toBeNull();
  });
});
