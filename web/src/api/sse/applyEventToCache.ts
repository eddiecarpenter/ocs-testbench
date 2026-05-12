import type { QueryClient } from '@tanstack/react-query';

import { dashboardKeys, type DashboardKpis } from '../resources/dashboard';
import {
  executionKeys,
  type Execution,
  type ExecutionPage,
  type ExecutionSummary,
  type ListExecutionsParams,
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
      applyExecutionCreated(queryClient, event.data);
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

function applyExecutionCreated(
  queryClient: QueryClient,
  summary: ExecutionSummary,
): void {
  // Walk every cached list query and only prepend the new summary where the
  // query's scenarioId filter matches (or is absent, meaning "show all").
  // Key shape: ['executions', 'list', ListExecutionsParams]
  const entries = queryClient.getQueriesData<ExecutionPage>({
    queryKey: executionKeys.all,
  });

  for (const [key, prev] of entries) {
    if (!prev || !Array.isArray(prev.items)) continue;
    if (prev.items.some((e) => e.id === summary.id)) continue;

    // key[2] is the params object when this is a list query; detail queries
    // have a string id at key[2] and are skipped below.
    const params = key[2] as ListExecutionsParams | string | undefined;
    if (typeof params === 'string') continue; // detail cache — skip
    if (params?.scenarioId && params.scenarioId !== summary.scenarioId) continue;

    queryClient.setQueryData<ExecutionPage>(key, {
      ...prev,
      items: [summary, ...prev.items],
      page: { ...prev.page, total: prev.page.total + 1 },
    });
  }
}

/** Strip an `Execution` back down to its `ExecutionSummary` fields. */
function toSummary(execution: Execution): ExecutionSummary {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { currentStep, totalSteps, steps, ...summary } = execution;
  return summary;
}
