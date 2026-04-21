import type { QueryClient } from '@tanstack/react-query';

import { dashboardKeys, type DashboardKpis } from '../resources/dashboard';
import {
  executionKeys,
  type Execution,
  type ExecutionPage,
  type ExecutionSummary,
} from '../resources/executions';
import { peerKeys, type Peer } from '../resources/peers';
import type { SseEvent } from './events';

/**
 * Apply a decoded SSE event to the React Query cache.
 *
 * Kept as a pure function (no React) so it can be unit-tested without
 * mounting a component tree.
 */
export function applyEventToCache(
  queryClient: QueryClient,
  event: SseEvent,
): void {
  switch (event.type) {
    case 'peer.updated':
      applyPeerUpdated(queryClient, event.data);
      return;

    case 'execution.progress':
      applyExecutionProgress(queryClient, event.data);
      return;

    case 'execution.created':
      // New row → refetch the list; detail queries for unknown ids aren't
      // mounted yet so no detail cache to seed.
      queryClient.invalidateQueries({ queryKey: executionKeys.all });
      return;

    case 'dashboard.kpi':
      queryClient.setQueryData<DashboardKpis>(
        dashboardKeys.kpis(),
        event.data,
      );
      return;
  }
}

function applyPeerUpdated(queryClient: QueryClient, peer: Peer): void {
  // Seed / refresh the detail cache.
  queryClient.setQueryData<Peer>(peerKeys.detail(peer.id), peer);

  // Patch the list cache in place — no refetch needed.
  queryClient.setQueryData<Peer[]>(peerKeys.list(), (prev) => {
    if (!prev) return prev;
    const idx = prev.findIndex((p) => p.id === peer.id);
    if (idx === -1) return [...prev, peer];
    const next = prev.slice();
    next[idx] = peer;
    return next;
  });
}

function applyExecutionProgress(
  queryClient: QueryClient,
  execution: Execution,
): void {
  // Detail cache — full replace.
  queryClient.setQueryData<Execution>(
    executionKeys.detail(execution.id),
    execution,
  );

  // Patch every list cache we've got (each `status` filter is its own key).
  queryClient.setQueriesData<ExecutionPage>(
    { queryKey: executionKeys.all },
    (prev) => {
      if (!prev || !Array.isArray(prev.items)) return prev;
      const summary: ExecutionSummary = toSummary(execution);
      const idx = prev.items.findIndex((e) => e.id === execution.id);
      if (idx === -1) return prev;
      const items = prev.items.slice();
      items[idx] = summary;
      return { ...prev, items };
    },
  );
}

/** Strip an `Execution` back down to its `ExecutionSummary` fields. */
function toSummary(execution: Execution): ExecutionSummary {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { currentStep, totalSteps, steps, ...summary } = execution;
  return summary;
}
