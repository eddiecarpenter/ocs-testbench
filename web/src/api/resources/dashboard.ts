import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type DashboardKpis = components['schemas']['DashboardKpis'];

export const dashboardKeys = {
  all: ['dashboard'] as const,
  kpis: () => [...dashboardKeys.all, 'kpis'] as const,
};

export const getDashboardKpis = (signal?: AbortSignal) =>
  ApiService.get<DashboardKpis>('/dashboard/kpis', { signal });

export function useDashboardKpis() {
  return useQuery({
    queryKey: dashboardKeys.kpis(),
    queryFn: ({ signal }) => getDashboardKpis(signal),
  });
}
