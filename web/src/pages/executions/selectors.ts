/**
 * Pure selectors for the Executions list page.
 *
 * Lives separately from the React component so tests can exercise the
 * filtering / lookup logic without touching the DOM (matches the
 * scenarios `listSelectors.ts` / `selectors.ts` pattern).
 *
 * Subsequent tasks add per-scenario aggregates (run counts, status
 * counts) here — Task 2 ships only the header lookup.
 */
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
