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
