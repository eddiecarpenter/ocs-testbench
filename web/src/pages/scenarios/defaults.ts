/**
 * Helpers for constructing a fresh, valid `Scenario` for new-scenario
 * mode. Ships the canonical AVP-tree starter so the Builder has something
 * to render on the Frame tab from the moment it opens.
 */
import type {
  AvpNode,
  Scenario,
  ScenarioInput,
  Service,
  ServiceProfile,
  ServiceType,
  SessionMode,
  Variable,
} from './types';

/** Default session mode implied by each service type. */
export function defaultSessionMode(serviceType: ServiceType | undefined): SessionMode {
  return serviceType === 'USSD1_EVENT' ? 'event' : 'session';
}

/**
 * Build the Service-Information AvpNode (code 873, vendorId 10415) for the
 * given serviceType + serviceProfile. Returns null when no mapping exists.
 *
 * All nodes in the returned subtree are marked `locked: true` so the Frame
 * tab prevents structural changes (the service-type determines the shape).
 * Leaf valueRefs remain editable.
 */
export function buildServiceInfoNode(
  serviceType: ServiceType | undefined,
  serviceProfile: ServiceProfile | undefined,
): AvpNode | null {
  if (!serviceType) return null;
  const children =
    (serviceProfile ?? '3GPP') === 'HUAWEI'
      ? buildHuaweiChildren(serviceType)
      : build3GPPChildren(serviceType);
  if (children.length === 0) return null;
  return { name: 'Service-Information', code: 873, vendorId: 10415, locked: true, children };
}

function build3GPPChildren(serviceType: ServiceType): AvpNode[] {
  switch (serviceType) {
    case 'VOICE':
      return [
        {
          name: 'IMS-Information',
          code: 876,
          vendorId: 10415,
          locked: true,
          children: [
            { name: 'Node-Functionality',    code: 862, vendorId: 10415, locked: true, valueRef: '0' },
            { name: 'Role-Of-Node',          code: 829, vendorId: 10415, locked: true, valueRef: 'ROLE_OF_NODE' },
            { name: 'Calling-Party-Address', code: 831, vendorId: 10415, locked: true, valueRef: 'CALLING_PARTY_ADDRESS' },
            { name: 'Called-Party-Address',  code: 832, vendorId: 10415, locked: true, valueRef: 'CALLED_PARTY_ADDRESS' },
          ],
        },
      ];
    case 'DATA':
      return [{ name: 'PS-Information',   code: 874,  vendorId: 10415, locked: true }];
    case 'SMS':
      return [{ name: 'SMS-Information',  code: 2000, vendorId: 10415, locked: true }];
    case 'USSD1_EVENT':
    case 'USSD1_SESSION':
    case 'USSD2_SESSION':
      return [{ name: 'USSD-Information', code: 885,  vendorId: 10415, locked: true }];
    default:
      return [];
  }
}

function buildHuaweiChildren(serviceType: ServiceType): AvpNode[] {
  switch (serviceType) {
    case 'VOICE':
      return [{ name: 'IN_INFORMATION',  code: 20300, vendorId: 2011, locked: true }];
    case 'DATA':
      return [{ name: 'PS-Information',  code: 874,   vendorId: 10415, locked: true }];
    case 'SMS':
      return [{ name: 'SMS_INFORMATION', code: 20400, vendorId: 2011, locked: true }];
    case 'USSD1_EVENT':
    case 'USSD1_SESSION':
    case 'USSD2_SESSION':
      return [{ name: 'DCD_INFORMATION', code: 2115,  vendorId: 2011, locked: true }];
    default:
      return [];
  }
}

/**
 * Replace (or append) the Service-Information node in `tree` for the given
 * serviceType + serviceProfile. If no node applies, removes any existing one.
 */
export function replaceServiceInfoNode(
  tree: AvpNode[],
  serviceType: ServiceType | undefined,
  serviceProfile: ServiceProfile | undefined,
): AvpNode[] {
  const node = buildServiceInfoNode(serviceType, serviceProfile);
  const idx = tree.findIndex((n) => n.name === 'Service-Information');
  if (node) {
    if (idx >= 0) return tree.map((n, i) => (i === idx ? node : n));
    return [...tree, node];
  }
  if (idx >= 0) return tree.filter((_, i) => i !== idx);
  return tree;
}

/** Variables required by the Service-Information AVP for the given serviceType. */
export function defaultVariablesForServiceType(serviceType: ServiceType): Variable[] {
  if (serviceType === 'VOICE') {
    return [
      {
        name: 'ROLE_OF_NODE',
        description: 'Role-Of-Node AVP (0 = Originating, 1 = Terminating).',
        source: { kind: 'generator', strategy: 'literal', refresh: 'once', params: { value: 0 } },
      },
      {
        name: 'CALLED_PARTY_ADDRESS',
        description: 'Called-party E164 number (raw digits, no +).',
        source: { kind: 'generator', strategy: 'literal', refresh: 'once', params: { value: '' } },
      },
    ];
  }
  return [];
}

/** Merge `additions` into `existing`, skipping names already present. */
export function mergeVariables(existing: Variable[], additions: Variable[]): Variable[] {
  const known = new Set(existing.map((v) => v.name));
  return [...existing, ...additions.filter((v) => !known.has(v.name))];
}

/**
 * The system-mandatory AVPs that appear at the top of every avpTree.
 * All are locked (no delete). Leaf values reference engine-seeded system
 * variables (ORIGIN_HOST, ORIGIN_REALM, DEST_REALM, SERVICE_CONTEXT_ID)
 * or literal values (Auth-Application-Id = 4).
 *
 * Session-Id (263) is NOT in this list — it is purely engine-generated
 * and shown only as an informational row above the tree.
 */
const SYSTEM_AVPS: AvpNode[] = [
  { name: 'Origin-Host',        code: 264, locked: true, valueRef: 'ORIGIN_HOST' },
  { name: 'Origin-Realm',       code: 296, locked: true, valueRef: 'ORIGIN_REALM' },
  { name: 'Destination-Realm',  code: 283, locked: true, valueRef: 'DEST_REALM' },
  { name: 'Service-Context-Id', code: 461, locked: true, valueRef: 'SERVICE_CONTEXT_ID' },
];

const BASE_AVP_TREE: AvpNode[] = [
  ...SYSTEM_AVPS,
  {
    name: 'Subscription-Id',
    code: 443,
    children: [
      { name: 'Subscription-Id-Type', code: 450, valueRef: 'SUB_ID_TYPE' },
      { name: 'Subscription-Id-Data', code: 444, valueRef: 'MSISDN' },
    ],
  },
];

const BASE_VARIABLES: Variable[] = [
  {
    name: 'MSISDN',
    description: 'Subscriber MSISDN — bound to the bound subscriber.',
    source: { kind: 'bound', from: 'subscriber', field: 'msisdn' },
  },
  {
    name: 'RATING_GROUP',
    description: 'Rating-Group AVP value.',
    source: { kind: 'generator', strategy: 'literal', refresh: 'once', params: { value: 100 } },
  },
  {
    name: 'RSU_TOTAL',
    description: 'Requested service-units quantity.',
    source: { kind: 'generator', strategy: 'literal', refresh: 'once', params: { value: 10 } },
  },
  {
    name: 'USU_TOTAL',
    description: 'Used service-units quantity reported in UPDATE and TERMINATE requests.',
    source: { kind: 'generator', strategy: 'literal', refresh: 'once', params: { value: 10 } },
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
  const serviceType: ServiceType = 'VOICE';
  const serviceProfile: ServiceProfile = '3GPP';
  const serviceInfoNode = buildServiceInfoNode(serviceType, serviceProfile)!;
  return {
    id: '',
    name: 'Untitled scenario',
    description: '',
    serviceType,
    serviceProfile,
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    origin: 'user',
    favourite: false,
    subscriberId: '',
    peerId: '',
    stepCount: 1,
    updatedAt: new Date().toISOString(),
    avpTree: [...BASE_AVP_TREE, serviceInfoNode],
    services: [DEFAULT_SERVICE],
    variables: mergeVariables(BASE_VARIABLES, defaultVariablesForServiceType(serviceType)),
    steps: [{ kind: 'request', requestType: 'INITIAL' }],
  };
}

/** Coalesce a `Scenario` into the wire `ScenarioInput` shape for save. */
export function toScenarioInput(s: Scenario): ScenarioInput {
  return {
    name: s.name,
    description: s.description,
    serviceType: s.serviceType,
    serviceProfile: s.serviceProfile,
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
