/**
 * Pure helper for the Start-Run dialog's optimistic-prepend update.
 *
 * Lives in its own module so the cache-update logic is unit-testable
 * without TanStack Query: the page calls
 * `queryClient.setQueriesData(<key>, prev => prependExecutions(prev, created))`
 * and the helper handles the merge + total-count maintenance.
 */
import type {
  ExecutionPage,
  ExecutionSummary,
} from '../../api/resources/executions';

/**
 * Return a new `ExecutionPage` with `created` rows prepended onto
 * `prev.items`. The list's `total` count is incremented by the number
 * of created rows so paging still feels coherent before the
 * authoritative refetch lands.
 *
 * Returns `prev` unchanged when it is `undefined` (no cache yet) or
 * when `created` is empty (no-op).
 */
export function prependExecutions(
  prev: ExecutionPage | undefined,
  created: readonly ExecutionSummary[],
): ExecutionPage | undefined {
  if (!prev) return prev;
  if (created.length === 0) return prev;
  // Defensive: if a cache entry under the same key prefix is reached
  // (e.g. an `Execution` detail under `['executions', ...]`), it will
  // not have an `items` array — leave it untouched rather than crash
  // when spreading.
  if (!Array.isArray(prev.items)) return prev;
  return {
    items: [...created, ...prev.items],
    page: { ...prev.page, total: prev.page.total + created.length },
  };
}
