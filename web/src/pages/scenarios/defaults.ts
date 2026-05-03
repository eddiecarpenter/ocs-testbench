/**
 * Helpers for constructing a fresh, valid `Scenario` for new-scenario
 * mode. The defaults hit a valid `serviceModel × unitType` combination
 * (`OCTET × single-mscc`) and ship the canonical AVP-tree starter so
 * the Builder has something to render on the Frame tab from the
 * moment it opens.
 */
import type {
  AvpNode,
  Scenario,
  ScenarioInput,
  Service,
  Variable,
} from './types';

const DEFAULT_AVP_TREE: AvpNode[] = [
  { name: 'Origin-Host', code: 264, valueRef: 'ORIGIN_HOST' },
  { name: 'Origin-Realm', code: 296, valueRef: 'ORIGIN_REALM' },
  { name: 'Destination-Realm', code: 283, valueRef: 'DEST_REALM' },
  { name: 'Service-Context-Id', code: 461, valueRef: 'SERVICE_CONTEXT' },
];

const DEFAULT_VARIABLES: Variable[] = [
  {
    name: 'MSISDN',
    description: 'Subscriber MSISDN — bound to the bound subscriber.',
    source: { kind: 'bound', from: 'subscriber', field: 'msisdn' },
  },
  {
    name: 'RATING_GROUP',
    description: 'Rating-Group AVP value.',
    source: {
      kind: 'generator',
      strategy: 'literal',
      refresh: 'once',
      params: { value: 100 },
    },
  },
  {
    name: 'RSU_TOTAL',
    description: 'Requested service-units quantity.',
    source: {
      kind: 'generator',
      strategy: 'literal',
      refresh: 'once',
      params: { value: 1_048_576 },
    },
  },
  {
    name: 'USU_TOTAL',
    description: 'Used service-units quantity reported in UPDATE and TERMINATE requests.',
    source: {
      kind: 'generator',
      strategy: 'literal',
      refresh: 'once',
      params: { value: 1_048_576 },
    },
  },
];

const DEFAULT_SERVICE: Service = {
  id: '100',
  ratingGroup: 'RATING_GROUP',
  requestedUnits: 'RSU_TOTAL',
};

/**
 * A fresh in-memory Scenario, used as the starting point for `/scenarios/new`.
 * The id is empty until the server assigns one on first save — the Builder
 * uses the `''` id as a "draft, never saved" sentinel.
 */
export function makeNewScenarioDraft(): Scenario {
  return {
    id: '',
    name: 'Untitled scenario',
    description: '',
    unitType: 'OCTET',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    origin: 'user',
    favourite: false,
    subscriberId: '',
    peerId: '',
    stepCount: 1,
    updatedAt: new Date().toISOString(),
    avpTree: DEFAULT_AVP_TREE,
    services: [DEFAULT_SERVICE],
    variables: DEFAULT_VARIABLES,
    steps: [
      {
        kind: 'request',
        requestType: 'INITIAL',
      },
    ],
  };
}

/** Coalesce a `Scenario` into the wire `ScenarioInput` shape for save. */
export function toScenarioInput(s: Scenario): ScenarioInput {
  return {
    name: s.name,
    description: s.description,
    unitType: s.unitType,
    sessionMode: s.sessionMode,
    serviceModel: s.serviceModel,
    favourite: s.favourite ?? false,
    subscriberId: s.subscriberId ?? '',
    peerId: s.peerId ?? '',
    avpTree: s.avpTree,
    services: s.services,
    variables: s.variables,
    steps: s.steps,
  };
}
