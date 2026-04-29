/**
 * Header sub-line builder for the Executions list — extracted into a
 * standalone module so the page component remains an "only-exports-
 * components" file (the project's React Refresh rule). The page imports
 * `buildSubHeader` from here; tests exercise it without needing to
 * touch the DOM.
 */
import type { ScenarioSummary } from '../scenarios/types';

/**
 * Sub-header per AC-3: `<unit-type>-session · <peer> · <step-count> steps`.
 *
 * Falls back to the raw peer id when the peer query hasn't loaded yet,
 * and to "no peer" when the scenario has none assigned. Step count is
 * always present on `ScenarioSummary`.
 */
export function buildSubHeader(
  scenario: ScenarioSummary,
  peerNameById: Map<string, string>,
): string {
  const session = `${scenario.unitType.toLowerCase()}-session`;
  const peer = scenario.peerId
    ? (peerNameById.get(scenario.peerId) ?? scenario.peerId)
    : 'no peer';
  const stepWord = scenario.stepCount === 1 ? 'step' : 'steps';
  return `${session} · ${peer} · ${scenario.stepCount} ${stepWord}`;
}
