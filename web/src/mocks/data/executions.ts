import type { ExecutionSummary } from '../../api/resources/executions';

/** Reference "now" — keep fixtures deterministic. */
const NOW = Date.parse('2026-04-21T09:15:00Z');
const sec = 1_000;
const min = 60 * sec;

function iso(msAgo: number): string {
  return new Date(NOW - msAgo).toISOString();
}

/**
 * 30 recent executions, newest first. 2 are still running — matches the
 * "Active runs: 2" KPI on the dashboard.
 */
export const executionFixtures: ExecutionSummary[] = [
  {
    id: '42',
    scenarioName: 'Data session — happy path',
    mode: 'continuous',
    peerId: 'peer-01',
    peerName: 'peer-01',
    result: 'success',
    startedAt: iso(12 * sec),
    finishedAt: iso(2 * sec),
  },
  {
    id: '41',
    scenarioName: 'Voice call charging',
    mode: 'interactive',
    peerId: 'peer-01',
    peerName: 'peer-01',
    result: 'success',
    startedAt: iso(1 * min),
    finishedAt: iso(45 * sec),
  },
  {
    id: '40',
    scenarioName: 'SMS burst load',
    mode: 'continuous',
    peerId: 'peer-02',
    peerName: 'peer-02',
    result: 'failure',
    startedAt: iso(5 * min),
    finishedAt: iso(4 * min),
  },
  {
    id: '39',
    scenarioName: 'FUI-TERMINATE compliance',
    mode: 'interactive',
    peerId: 'peer-01',
    peerName: 'peer-01',
    result: 'success',
    startedAt: iso(8 * min),
    finishedAt: iso(7 * min),
  },
  {
    id: '38',
    scenarioName: 'Validity-Time re-auth',
    mode: 'continuous',
    peerId: 'peer-04',
    peerName: 'peer-04',
    result: 'success',
    startedAt: iso(15 * min),
    finishedAt: iso(13 * min),
  },
  // Active runs (no finishedAt)
  {
    id: '43',
    scenarioName: 'Throughput baseline (100 TPS)',
    mode: 'continuous',
    peerId: 'peer-04',
    peerName: 'peer-04',
    result: 'running',
    startedAt: iso(30 * sec),
  },
  {
    id: '44',
    scenarioName: 'Handover CCR-Update',
    mode: 'interactive',
    peerId: 'peer-01',
    peerName: 'peer-01',
    result: 'running',
    startedAt: iso(3 * sec),
  },
  // Older history
  ...Array.from({ length: 23 }, (_, i): ExecutionSummary => {
    const id = String(37 - i);
    const startMin = 20 + i * 3;
    return {
      id,
      scenarioName: [
        'Data session — quota exhausted',
        'Multi-MSCC rating group',
        'Policy change mid-session',
        'Peer fallback on CER/CEA timeout',
      ][i % 4],
      mode: i % 2 === 0 ? 'continuous' : 'interactive',
      peerId: `peer-0${1 + (i % 4)}`,
      peerName: `peer-0${1 + (i % 4)}`,
      result: i % 7 === 3 ? 'failure' : 'success',
      startedAt: iso(startMin * min),
      finishedAt: iso((startMin - 1) * min),
    };
  }),
];
