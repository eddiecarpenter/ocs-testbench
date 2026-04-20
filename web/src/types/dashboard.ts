export type PeerStatus = 'connected' | 'disconnected' | 'error' | 'connecting';
export type ExecutionResult = 'success' | 'failure' | 'running';

export interface KpiStat {
  label: string;
  value: string;
  subtitle?: string;
}

export interface PeerSummary {
  id: string;
  name: string;
  endpoint: string;
  originHost: string;
  status: PeerStatus;
  detail: string;
}

export interface ExecutionSummary {
  id: string;
  name: string;
  mode: 'Interactive' | 'Continuous';
  peer: string;
  result: ExecutionResult;
  relativeTime: string;
}

export interface ResponseTimePoint {
  t: string;
  p50: number;
  p95: number;
  p99: number;
}
