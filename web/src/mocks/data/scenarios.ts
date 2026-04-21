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

export const scenarioFixtures: ScenarioSummary[] = names.map((name, i) => ({
  id: `scn-${String(i + 1).padStart(3, '0')}`,
  name,
  description: `Scenario covering ${name.toLowerCase()}`,
  stepCount: 3 + ((i * 7) % 6),
  updatedAt: `2026-04-${String(1 + (i % 20)).padStart(2, '0')}T10:00:00Z`,
}));
