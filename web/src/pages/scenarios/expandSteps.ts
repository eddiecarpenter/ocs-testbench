/**
 * Expand a scenario's authored step list into the flat per-round
 * sequence the engine actually executes.
 *
 * Today this only matters for `request` steps with `requestType: 'UPDATE'`
 * carrying a `repeat` policy — those expand into `N` synthetic entries
 * (one per iteration). All other step kinds pass through 1-for-1.
 *
 * Pure helper — no side effects, no scenario mutation. Both the mock
 * SSE driver (`installExecutionSse`) and the snapshot builder
 * (`buildExecutionDetail`) call this so the timeline they emit agrees
 * row-for-row.
 *
 * The "infinite loop" case (no `times`, only `until`) is not solvable
 * at design time without evaluating the predicate against a live
 * context, which the mock cannot do. We cap such loops at
 * `DEFAULT_UNTIL_ROUNDS` and pretend the predicate terminated exactly
 * there. Real-engine work will replace this with a runtime predicate
 * evaluator.
 */
import type { ScenarioStep } from './types';

/** Fallback round count when only `until` is specified (mock-only). */
export const DEFAULT_UNTIL_ROUNDS = 5;

export interface ExpandedStep {
  /** Index of the source step in `scenario.steps`. */
  sourceIndex: number;
  /** The authored step this expanded entry derives from. */
  source: ScenarioStep;
  /**
   * When the source is an UPDATE with `repeat`, the 1-based iteration
   * index (`1`..`totalRounds`). Otherwise `undefined`.
   */
  roundIndex?: number;
  /** Total rounds the source produces. `undefined` for non-repeating steps. */
  totalRounds?: number;
  /**
   * Inter-round delay in milliseconds. Mirrored from
   * `repeat.delayMs` for repeating steps; `undefined` otherwise.
   * Consumers that pace events (the SSE driver) honour this between
   * sibling rounds.
   */
  delayMs?: number;
}

/**
 * Walk `scenario.steps` and produce the expanded execution sequence.
 *
 * Examples (just the labels):
 *   [INITIAL, UPDATE, TERMINATE]                 → 3 entries
 *   [INITIAL, UPDATE(repeat:{times:4}), TERMINATE] → 6 entries (1 + 4 + 1)
 *   [INITIAL, UPDATE(repeat:{until:…}), TERMINATE] → 1 + DEFAULT_UNTIL_ROUNDS + 1
 */
export function expandScenarioSteps(steps: readonly ScenarioStep[]): ExpandedStep[] {
  const out: ExpandedStep[] = [];
  for (let i = 0; i < steps.length; i += 1) {
    const step = steps[i];
    const rounds = totalRoundsFor(step);
    if (rounds === 1) {
      out.push({ sourceIndex: i, source: step });
      continue;
    }
    // Repeating UPDATE — emit one entry per round.
    const delayMs =
      step.kind === 'request' &&
      step.requestType === 'UPDATE' &&
      step.repeat?.delayMs !== undefined
        ? step.repeat.delayMs
        : 0;
    for (let r = 1; r <= rounds; r += 1) {
      out.push({
        sourceIndex: i,
        source: step,
        roundIndex: r,
        totalRounds: rounds,
        delayMs,
      });
    }
  }
  return out;
}

/**
 * Number of rounds an authored step produces.
 *
 * Non-UPDATE steps and UPDATEs without `repeat` always return `1`. UPDATE
 * with `repeat.times` returns that count. UPDATE with only `repeat.until`
 * (no `times`) returns `DEFAULT_UNTIL_ROUNDS` — see the file header for
 * the rationale.
 */
export function totalRoundsFor(step: ScenarioStep): number {
  if (step.kind !== 'request') return 1;
  if (step.requestType !== 'UPDATE') return 1;
  if (!step.repeat) return 1;
  if (typeof step.repeat.times === 'number' && step.repeat.times >= 1) {
    return step.repeat.times;
  }
  if (step.repeat.until) return DEFAULT_UNTIL_ROUNDS;
  return 1;
}
