import type { ScenarioSummary } from '../../api/resources/scenarios';

/** 24 scenarios — matches dashboard KPIs. */
const names = [
  'Data session — happy path',
  'Data session — quota exhausted',
  'Voice call charging',
  'Voice call early termination',
  'SMS burst load',
  'SMS event reject',
  'FUI-TERMINATE compliance',
  'FUI-REDIRECT compliance',
  'FUI-RESTRICT compliance',
  'Validity-Time re-auth',
  'Multi-MSCC rating group',
  'Multi-session same subscriber',
  'Result code 4010 handling',
  'Result code 4011 handling',
  'Result code 5030 handling',
  'Re-validation after timeout',
  'Background session renewal',
  'Handover CCR-Update',
  'Roaming partner charging',
  'Zero-rated APN',
  'Policy change mid-session',
  'Peer fallback on CER/CEA timeout',
  'Subscriber not provisioned',
  'Throughput baseline (100 TPS)',
];

type UnitType = ScenarioSummary['unitType'];
type SessionMode = ScenarioSummary['sessionMode'];
type ServiceModel = ScenarioSummary['serviceModel'];

/** Deterministic variation so the list view surfaces a mix of types. */
function unitTypeFor(i: number): UnitType {
  const choices: UnitType[] = ['OCTET', 'TIME', 'UNITS'];
  return choices[i % 3];
}

function sessionModeFor(i: number): SessionMode {
  // SMS / USSD events (positions 4, 5 in `names`) get `event`, otherwise `session`
  return i === 4 || i === 5 ? 'event' : 'session';
}

function serviceModelFor(unit: UnitType, session: SessionMode): ServiceModel {
  // Compatibility matrix (architecture §4): OCTET → multi-mscc,
  // TIME/UNITS → single-mscc (root is the legacy simple variant).
  if (unit === 'OCTET') return 'multi-mscc';
  if (session === 'event') return 'single-mscc';
  return 'single-mscc';
}

export const scenarioFixtures: ScenarioSummary[] = names.map((name, i) => {
  const unitType = unitTypeFor(i);
  const sessionMode = sessionModeFor(i);
  return {
    id: `scn-${String(i + 1).padStart(3, '0')}`,
    name,
    description: `Scenario covering ${name.toLowerCase()}`,
    unitType,
    sessionMode,
    serviceModel: serviceModelFor(unitType, sessionMode),
    origin: i < 5 ? 'system' : 'user',
    stepCount: 3 + ((i * 7) % 6),
    updatedAt: `2026-04-${String(1 + (i % 20)).padStart(2, '0')}T10:00:00Z`,
  };
});
