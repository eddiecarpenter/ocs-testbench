/**
 * Scenario fixtures used by the in-memory mock layer.
 *
 * Coverage requirement (Task 1, AC: one per valid combination):
 *
 *   VOICE  × single-mscc (3GPP)
 *   DATA   × multi-mscc  (3GPP)
 *   DATA   × single-mscc (3GPP)
 *   SMS    × root        (3GPP)
 *   SMS    × single-mscc (3GPP)
 *   VOICE  × single-mscc (HUAWEI)
 *
 * Plus a single force-failure id (`error-`) used in failure-path tests.
 *
 * Every fixture is hand-built to satisfy the OpenAPI v0.2 Scenario shape
 * — types come straight from `schema.d.ts` so a future schema change
 * surfaces here as a TS error rather than as a runtime mock drift.
 */
import type {
  AvpNode,
  Scenario,
  Service,
  Variable,
} from '../../pages/scenarios/types';

const baseAvpTree: AvpNode[] = [
  { name: 'Origin-Host', code: 264, valueRef: 'ORIGIN_HOST' },
  { name: 'Origin-Realm', code: 296, valueRef: 'ORIGIN_REALM' },
  { name: 'Destination-Realm', code: 283, valueRef: 'DEST_REALM' },
  { name: 'Service-Context-Id', code: 461, valueRef: 'SERVICE_CONTEXT' },
  {
    name: 'Subscription-Id',
    code: 443,
    children: [
      { name: 'Subscription-Id-Type', code: 450, valueRef: 'SUB_ID_TYPE' },
      { name: 'Subscription-Id-Data', code: 444, valueRef: 'MSISDN' },
    ],
  },
];

const variableLib = {
  msisdn: (): Variable => ({
    name: 'MSISDN',
    description: 'Subscriber MSISDN — bound to the bound subscriber.',
    source: { kind: 'bound', from: 'subscriber', field: 'msisdn' },
  }),
  rsuTotal: (name: string, value: number): Variable => ({
    name,
    description: 'Requested service-units quantity.',
    source: {
      kind: 'generator',
      strategy: 'literal',
      refresh: 'once',
      params: { value },
    },
  }),
  usuTotal: (name: string, value: number): Variable => ({
    name,
    description: 'Used service-units quantity.',
    source: {
      kind: 'generator',
      strategy: 'literal',
      refresh: 'once',
      params: { value },
    },
  }),
  ratingGroup: (name: string, value: number): Variable => ({
    name,
    description: 'Rating-Group AVP value.',
    source: {
      kind: 'generator',
      strategy: 'literal',
      refresh: 'once',
      params: { value },
    },
  }),
};

const rootService = (): Service => ({
  id: 'root',
  requestedUnits: 'RSU_TOTAL',
  usedUnits: 'USU_TOTAL',
});

const singleMsccService = (rg: number): Service => ({
  id: String(rg),
  ratingGroup: 'RATING_GROUP',
  requestedUnits: 'RSU_TOTAL',
  usedUnits: 'USU_TOTAL',
});

const multiMsccServices = (rgs: number[]): Service[] =>
  rgs.map((rg) => ({
    id: String(rg),
    ratingGroup: `RG${rg}_RATING_GROUP`,
    requestedUnits: `RG${rg}_RSU_TOTAL`,
    usedUnits: `RG${rg}_USU_TOTAL`,
  }));

function makeScenario(opts: {
  id: string;
  name: string;
  description: string;
  serviceType: Scenario['serviceType'];
  serviceProfile?: Scenario['serviceProfile'];
  sessionMode: Scenario['sessionMode'];
  serviceModel: Scenario['serviceModel'];
  services: Service[];
  variables: Variable[];
  origin?: Scenario['origin'];
  subscriberId?: string;
  peerId?: string;
}): Scenario {
  const steps: Scenario['steps'] =
    opts.sessionMode === 'session'
      ? [
          {
            kind: 'request',
            requestType: 'INITIAL',
          },
          {
            kind: 'request',
            requestType: 'UPDATE',
          },
          {
            kind: 'request',
            requestType: 'TERMINATE',
          },
        ]
      : [
          {
            kind: 'request',
            requestType: 'EVENT',
          },
        ];

  return {
    id: opts.id,
    name: opts.name,
    description: opts.description,
    serviceType: opts.serviceType,
    serviceProfile: opts.serviceProfile ?? '3GPP',
    sessionMode: opts.sessionMode,
    serviceModel: opts.serviceModel,
    origin: opts.origin ?? 'user',
    favourite: false,
    subscriberId: opts.subscriberId ?? 'sub-001',
    peerId: opts.peerId ?? 'peer-01',
    stepCount: steps.length,
    updatedAt: '2026-04-28T10:00:00Z',
    avpTree: baseAvpTree,
    services: opts.services,
    variables: opts.variables,
    steps,
  };
}

export const scenarioFixtures: Scenario[] = [
  makeScenario({
    id: 'scn-voice-single-001',
    name: 'Voice — single-MSCC (3GPP)',
    description: 'Modern single-MSCC voice charging with IMS-Information.',
    serviceType: 'VOICE',
    serviceProfile: '3GPP',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    services: [singleMsccService(50)],
    variables: [
      variableLib.msisdn(),
      variableLib.ratingGroup('RATING_GROUP', 50),
      variableLib.rsuTotal('RSU_TOTAL', 600),
      variableLib.usuTotal('USU_TOTAL', 300),
    ],
  }),
  makeScenario({
    id: 'scn-data-multi-001',
    name: 'Data — multi-MSCC (3GPP)',
    description: 'Two MSCCs (RG 100 and RG 200) sharing a data session.',
    serviceType: 'DATA',
    serviceProfile: '3GPP',
    sessionMode: 'session',
    serviceModel: 'multi-mscc',
    services: multiMsccServices([100, 200]),
    variables: [
      variableLib.msisdn(),
      variableLib.ratingGroup('RG100_RATING_GROUP', 100),
      variableLib.rsuTotal('RG100_RSU_TOTAL', 1_048_576),
      variableLib.usuTotal('RG100_USU_TOTAL', 524_288),
      variableLib.ratingGroup('RG200_RATING_GROUP', 200),
      variableLib.rsuTotal('RG200_RSU_TOTAL', 524_288),
      variableLib.usuTotal('RG200_USU_TOTAL', 262_144),
    ],
  }),
  makeScenario({
    id: 'scn-data-single-001',
    name: 'Data — single-MSCC (3GPP)',
    description: 'Single MSCC data session with PS-Information.',
    serviceType: 'DATA',
    serviceProfile: '3GPP',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    services: [singleMsccService(100)],
    variables: [
      variableLib.msisdn(),
      variableLib.ratingGroup('RATING_GROUP', 100),
      variableLib.rsuTotal('RSU_TOTAL', 1_048_576),
      variableLib.usuTotal('USU_TOTAL', 524_288),
    ],
  }),
  makeScenario({
    id: 'scn-sms-root-001',
    name: 'SMS — root (3GPP)',
    description: 'Root-level SMS event charging.',
    serviceType: 'SMS',
    serviceProfile: '3GPP',
    sessionMode: 'event',
    serviceModel: 'root',
    services: [rootService()],
    variables: [
      variableLib.msisdn(),
      variableLib.rsuTotal('RSU_TOTAL', 1),
    ],
  }),
  makeScenario({
    id: 'scn-sms-single-001',
    name: 'SMS — single-MSCC (3GPP)',
    description: 'MSCC-wrapped SMS charging.',
    serviceType: 'SMS',
    serviceProfile: '3GPP',
    sessionMode: 'event',
    serviceModel: 'single-mscc',
    services: [singleMsccService(70)],
    variables: [
      variableLib.msisdn(),
      variableLib.ratingGroup('RATING_GROUP', 70),
      variableLib.rsuTotal('RSU_TOTAL', 5),
    ],
  }),
  makeScenario({
    id: 'scn-voice-single-huawei-001',
    name: 'Voice — single-MSCC (Huawei)',
    description: 'Huawei IN_INFORMATION voice charging.',
    serviceType: 'VOICE',
    serviceProfile: 'HUAWEI',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    services: [singleMsccService(50)],
    variables: [
      variableLib.msisdn(),
      variableLib.ratingGroup('RATING_GROUP', 50),
      variableLib.rsuTotal('RSU_TOTAL', 600),
      variableLib.usuTotal('USU_TOTAL', 300),
    ],
  }),
];

/** Reserved id-prefix for force-failure scenarios in PUT path. */
export const FORCE_FAILURE_ID_PREFIX = 'error-';

/**
 * A pre-seeded force-failure scenario the UI can open and attempt to
 * save — the PUT handler always returns 5xx for any id starting with
 * `error-`.
 */
export const forceFailureScenario: Scenario = makeScenario({
  id: 'error-force-fail',
  name: 'FAILURE — saving this returns 5xx',
  description: 'Force-failure fixture exercising the error path.',
  serviceType: 'DATA',
  serviceProfile: '3GPP',
  sessionMode: 'session',
  serviceModel: 'single-mscc',
  services: [singleMsccService(999)],
  variables: [
    variableLib.msisdn(),
    variableLib.ratingGroup('RATING_GROUP', 999),
    variableLib.rsuTotal('RSU_TOTAL', 1024),
  ],
});

/** Combined seed used by the mock store on boot. */
export const initialScenarioStore: Scenario[] = [
  ...scenarioFixtures,
  forceFailureScenario,
];
