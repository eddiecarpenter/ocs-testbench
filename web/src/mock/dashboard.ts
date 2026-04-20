import type {
  ExecutionSummary,
  KpiStat,
  PeerSummary,
  ResponseTimePoint,
} from '../types/dashboard';

export const mockKpis: KpiStat[] = [
  {
    label: 'Peers',
    value: '3 / 5',
    subtitle: 'connected / total',
    to: '/peers',
  },
  {
    label: 'Subscribers',
    value: '142',
    subtitle: 'registered subscribers',
    to: '/subscribers',
  },
  {
    label: 'Templates',
    value: '8',
    subtitle: 'AVP templates',
    to: '/templates',
  },
  {
    label: 'Scenarios',
    value: '24',
    subtitle: 'defined scenarios',
    to: '/scenarios',
  },
  {
    label: 'Active runs',
    value: '2',
    subtitle: 'in progress',
    to: '/execution',
  },
];

export const mockPeers: PeerSummary[] = [
  {
    id: 'peer-01',
    name: 'peer-01',
    endpoint: '10.0.1.5:3868',
    originHost: 'ctf-01.test.local',
    status: 'connected',
    detail: '10.0.1.5:3868 · ctf-01.test',
  },
  {
    id: 'peer-02',
    name: 'peer-02',
    endpoint: '10.0.1.6:3868',
    originHost: 'ctf-02.test.local',
    status: 'disconnected',
    detail: '10.0.1.6:3868 · ctf-02.test',
  },
  {
    id: 'peer-03',
    name: 'peer-03',
    endpoint: '10.0.2.5:3868',
    originHost: 'ctf-03.test.local',
    status: 'error',
    detail: '10.0.2.5:3868 · ctf-03.test',
  },
];

export const mockExecutions: ExecutionSummary[] = [
  {
    id: '42',
    name: 'Data session — happy path',
    mode: 'Continuous',
    peer: 'peer-01',
    result: 'success',
    relativeTime: '12s ago',
  },
  {
    id: '41',
    name: 'Voice call charging',
    mode: 'Interactive',
    peer: 'peer-01',
    result: 'success',
    relativeTime: '1m ago',
  },
  {
    id: '40',
    name: 'SMS burst load',
    mode: 'Continuous',
    peer: 'peer-02',
    result: 'failure',
    relativeTime: '5m ago',
  },
  {
    id: '39',
    name: 'FUI-TERMINATE compliance',
    mode: 'Interactive',
    peer: 'peer-01',
    result: 'success',
    relativeTime: '8m ago',
  },
  {
    id: '38',
    name: 'Validity-Time re-auth',
    mode: 'Continuous',
    peer: 'peer-04',
    result: 'success',
    relativeTime: '15m ago',
  },
];

// Deterministic pseudo-random series so re-renders don't flicker
function rng(seed: number) {
  return () => {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    return seed / 0xffffffff;
  };
}

function series(seed: number, base: number, amp: number, n = 30) {
  const r = rng(seed);
  let v = base;
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    v += (r() - 0.5) * amp * 0.8;
    v = Math.max(base - amp, Math.min(base + amp, v));
    out.push(Math.round(v));
  }
  return out;
}

const labelFor = (i: number, n: number) => {
  const minsAgo = Math.round(60 - (i / (n - 1)) * 60);
  return minsAgo === 0 ? 'now' : `-${minsAgo}m`;
};

export function mockResponseTime(n = 30): ResponseTimePoint[] {
  const p50 = series(42, 35, 15, n);
  const p95 = series(17, 80, 25, n);
  const p99 = series(91, 130, 40, n);
  return p50.map((_, i) => ({
    t: labelFor(i, n),
    p50: p50[i],
    p95: p95[i],
    p99: p99[i],
  }));
}
