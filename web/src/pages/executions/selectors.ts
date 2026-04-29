/**
 * Pure selectors for the Executions list page.
 *
 * Lives separately from the React component so tests can exercise the
 * filtering / lookup logic without touching the DOM (matches the
 * scenarios `listSelectors.ts` / `selectors.ts` pattern).
 */
import type { ExecutionSummary } from '../../api/resources/executions';
import type { ScenarioSummary } from '../scenarios/types';

/**
 * Resolve a scenario row by URL `?scenario=<id>` for the right-pane
 * header. Returns `undefined` when no id is set or the id does not
 * match any loaded scenario (e.g. a stale deep-link), in which case
 * the page falls back to the "All runs" header.
 */
export function selectScenarioForHeader(
  scenarios: readonly ScenarioSummary[],
  scenarioId: string | null | undefined,
): ScenarioSummary | undefined {
  if (!scenarioId) return undefined;
  return scenarios.find((s) => s.id === scenarioId);
}

/**
 * Per-scenario run-count aggregate, used by the sidebar badge.
 * Map keyed by `scenarioId`. Scenarios without any runs are absent
 * from the map; consumers fall back to 0.
 */
export function countRunsByScenario(
  executions: readonly ExecutionSummary[],
): Map<string, number> {
  const counts = new Map<string, number>();
  for (const exec of executions) {
    counts.set(exec.scenarioId, (counts.get(exec.scenarioId) ?? 0) + 1);
  }
  return counts;
}

/**
 * Most-recent `startedAt` per scenario for the sidebar's "last run"
 * line. ISO-string compare is safe because the format is
 * lexicographically ordered when consistent (Z-terminated) — every
 * fixture and POST handler uses `new Date(...).toISOString()`.
 */
export function lastRunByScenario(
  executions: readonly ExecutionSummary[],
): Map<string, string> {
  const last = new Map<string, string>();
  for (const exec of executions) {
    const cur = last.get(exec.scenarioId);
    if (!cur || exec.startedAt > cur) last.set(exec.scenarioId, exec.startedAt);
  }
  return last;
}

/**
 * Case-insensitive name filter for the sidebar search input. Empty /
 * whitespace search returns the input unchanged. Order is preserved.
 */
export function filterScenariosByName<T extends { name: string }>(
  scenarios: readonly T[],
  search: string,
): T[] {
  const needle = search.trim().toLowerCase();
  if (!needle) return [...scenarios];
  return scenarios.filter((s) => s.name.toLowerCase().includes(needle));
}

/**
 * Filter-chip status keys. The page UI exposes four chips matching
 * design — `all` is the reset, the rest map to OpenAPI v0.2 execution
 * states grouped to user-facing buckets.
 */
export type StatusFilter = 'all' | 'running' | 'completed' | 'failed';

const VALID_STATUS_FILTERS: ReadonlySet<StatusFilter> = new Set([
  'all',
  'running',
  'completed',
  'failed',
]);

/**
 * Map a URL `?status=` value onto the typed filter, falling back to
 * `'all'` when the param is missing or unrecognised.
 */
export function parseStatusFilter(raw: string | null): StatusFilter {
  if (raw && VALID_STATUS_FILTERS.has(raw as StatusFilter)) {
    return raw as StatusFilter;
  }
  return 'all';
}

/** True when the chip's filter bucket includes `state`. */
export function statusFilterMatches(
  filter: StatusFilter,
  state: ExecutionSummary['state'],
): boolean {
  switch (filter) {
    case 'all':
      return true;
    case 'running':
      return state === 'running' || state === 'paused' || state === 'pending';
    case 'completed':
      return state === 'success';
    case 'failed':
      return state === 'failure' || state === 'error' || state === 'aborted';
    default:
      return true;
  }
}

/**
 * Per-bucket counts for the filter chips. `all` reflects the entire
 * input set; the rest count rows in their respective buckets.
 */
export function countByStatusFilter(
  executions: readonly ExecutionSummary[],
): Record<StatusFilter, number> {
  const out: Record<StatusFilter, number> = {
    all: executions.length,
    running: 0,
    completed: 0,
    failed: 0,
  };
  for (const exec of executions) {
    if (statusFilterMatches('running', exec.state)) out.running += 1;
    if (statusFilterMatches('completed', exec.state)) out.completed += 1;
    if (statusFilterMatches('failed', exec.state)) out.failed += 1;
  }
  return out;
}

/**
 * Apply the status chip to the candidate executions list — pure, so
 * tests can drive it directly. (The previous peer-dropdown filter was
 * removed; peer is now visible as a column in the table and can be
 * column-sorted.)
 */
export function applyTableFilters(
  executions: readonly ExecutionSummary[],
  filters: { status: StatusFilter },
): ExecutionSummary[] {
  return executions.filter((e) =>
    statusFilterMatches(filters.status, e.state),
  );
}

/**
 * Pick the most-recent run for a given scenario. Used by Re-run
 * latest. Returns `undefined` when the scenario has never been run.
 */
export function selectLatestRunForScenario(
  executions: readonly ExecutionSummary[],
  scenarioId: string,
): ExecutionSummary | undefined {
  let best: ExecutionSummary | undefined;
  for (const exec of executions) {
    if (exec.scenarioId !== scenarioId) continue;
    if (!best || exec.startedAt > best.startedAt) best = exec;
  }
  return best;
}
