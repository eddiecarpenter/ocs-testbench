import type { DashboardKpis } from '../../api/resources/dashboard';

/** Display shape consumed by `KpiCard` — purely presentational. */
export interface KpiStat {
  label: string;
  value: string;
  subtitle?: string;
  /** Optional route the tile navigates to when clicked. */
  to?: string;
}

/** Project the backend KPI counters into the 5 tiles shown on the dashboard. */
export function toKpiStats(kpis: DashboardKpis): KpiStat[] {
  return [
    {
      label: 'Peers',
      value: `${kpis.peers.connected} / ${kpis.peers.total}`,
      subtitle: 'connected / total',
      to: '/peers',
    },
    {
      label: 'Subscribers',
      value: String(kpis.subscribers),
      subtitle: 'registered subscribers',
      to: '/subscribers',
    },
    {
      label: 'Scenarios',
      value: String(kpis.scenarios),
      subtitle: 'defined scenarios',
      to: '/scenarios',
    },
    {
      label: 'Active executions',
      value: String(kpis.activeRuns),
      subtitle: 'in progress',
      to: '/executions',
    },
  ];
}
