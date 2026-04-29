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
 * Round counting:
 *   - With `repeat.times` set, that's the count.
 *   - With only `repeat.until`, we *simulate* the predicate against a
 *     synthetic context (per-round consumption fed by `USU_TOTAL`-style
 *     variables) until the predicate fires. This makes "consume up to
 *     10 MB" scenarios produce a realistic round count even though the
 *     real engine evaluates the predicate at runtime.
 *   - When simulation can't determine a count (predicate references a
 *     variable we don't know how to derive), we fall back to
 *     `DEFAULT_UNTIL_ROUNDS`.
 *   - Random-int per-send variables are sampled deterministically
 *     when `rng` is seeded from the executionId (caller's choice);
 *     otherwise `Math.random` is used and each call is non-deterministic.
 */
import type {
  Predicate,
  Scenario,
  ScenarioStep,
  Variable,
} from './types';

/** Fallback round count when simulation can't determine a number. */
export const DEFAULT_UNTIL_ROUNDS = 10;

/** Hard safety cap on simulated rounds — protects against runaway predicates. */
export const MAX_SIMULATED_ROUNDS = 100;

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

export interface ExpandOptions {
  /** Random number generator (`[0,1)`). Defaults to `Math.random`. */
  rng?: () => number;
}

/**
 * Walk `scenario.steps` and produce the expanded execution sequence.
 *
 * Examples (just the labels):
 *   [INITIAL, UPDATE, TERMINATE]                 → 3 entries
 *   [INITIAL, UPDATE(repeat:{times:4}), TERMINATE] → 6 entries (1 + 4 + 1)
 *   [INITIAL, UPDATE(repeat:{until: USED ≥ 10 MB}), TERMINATE]
 *     → 1 + simulateUntilRounds(...) + 1
 */
export function expandScenarioSteps(
  steps: readonly ScenarioStep[],
  scenario?: Pick<Scenario, 'variables'>,
  options: ExpandOptions = {},
): ExpandedStep[] {
  const out: ExpandedStep[] = [];
  for (let i = 0; i < steps.length; i += 1) {
    const step = steps[i];
    const rounds = totalRoundsFor(step, scenario, options);
    if (rounds === 1) {
      out.push({ sourceIndex: i, source: step });
      continue;
    }
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
 * - Non-UPDATE steps and UPDATEs without `repeat` always return `1`.
 * - UPDATE with `repeat.times` returns that count (truncated by `until`
 *   only when `scenario` is passed and simulation fires earlier).
 * - UPDATE with only `repeat.until` runs the simulator (see
 *   `simulateUntilRounds`); without `scenario` the simulator can't run
 *   and we fall back to `DEFAULT_UNTIL_ROUNDS`.
 */
export function totalRoundsFor(
  step: ScenarioStep,
  scenario?: Pick<Scenario, 'variables'>,
  options: ExpandOptions = {},
): number {
  if (step.kind !== 'request') return 1;
  if (step.requestType !== 'UPDATE') return 1;
  if (!step.repeat) return 1;
  const cap =
    typeof step.repeat.times === 'number' && step.repeat.times >= 1
      ? step.repeat.times
      : MAX_SIMULATED_ROUNDS;
  if (step.repeat.until) {
    if (!scenario) {
      // Without scenario context the simulator can't sample variables.
      // Fall back to the lesser of `times` (if any) and the default.
      return Math.min(cap, DEFAULT_UNTIL_ROUNDS);
    }
    return simulateUntilRounds(scenario, step.repeat.until, cap, options);
  }
  // `times` set, no `until`.
  return cap;
}

/**
 * Synthetic predicate evaluator. Walks rounds 1..cap, accumulating a
 * per-round usage value into `until.variable`, until the predicate
 * fires (or `cap` is reached).
 *
 * Heuristic for the per-round value: derive from the scenario variable
 * whose name matches the predicate variable with `USED_TOTAL` swapped
 * to `USU_TOTAL` — that's the convention `variableLib` uses. If that
 * variable can't be resolved to a number, the simulator can't proceed
 * and returns `DEFAULT_UNTIL_ROUNDS` capped at `cap`.
 *
 * Real-engine territory replaces this with an expression evaluator
 * over the live execution context.
 */
function simulateUntilRounds(
  scenario: Pick<Scenario, 'variables'>,
  until: Predicate,
  cap: number,
  options: ExpandOptions,
): number {
  const rng = options.rng ?? Math.random;
  const perRoundVarName = inferPerRoundVar(until.variable);
  const perRoundDef = scenario.variables.find((v) => v.name === perRoundVarName);
  if (!perRoundDef) return Math.min(cap, DEFAULT_UNTIL_ROUNDS);

  // Synthetic accumulator — predicate variable starts at 0.
  let accumulated = 0;
  for (let r = 1; r <= cap; r += 1) {
    const sample = sampleValue(perRoundDef, rng);
    if (sample === null) return Math.min(cap, DEFAULT_UNTIL_ROUNDS);
    accumulated += sample;
    if (predicateFires(until, accumulated)) return r;
  }
  return cap;
}

/** Map `USED_TOTAL` → `USU_TOTAL`, `RG100_USED_TOTAL` → `RG100_USU_TOTAL`, etc. */
function inferPerRoundVar(predicateVar: string): string {
  return predicateVar.replace('USED_TOTAL', 'USU_TOTAL');
}

/**
 * Sample a single value from a generator-sourced numeric variable.
 * Returns `null` if the variable's source is opaque (bound, extracted,
 * or a non-numeric strategy).
 */
function sampleValue(variable: Variable, rng: () => number): number | null {
  if (variable.source.kind !== 'generator') return null;
  const src = variable.source;
  const params = (src.params ?? {}) as Record<string, unknown>;
  if (src.strategy === 'literal') {
    const v = params.value;
    return typeof v === 'number' ? v : null;
  }
  if (src.strategy === 'random-int') {
    const min = Number(params.min ?? 0);
    const max = Number(params.max ?? min);
    if (!Number.isFinite(min) || !Number.isFinite(max) || max < min) {
      return null;
    }
    return Math.floor(rng() * (max - min + 1)) + min;
  }
  return null;
}

/** Compare an accumulated number against a `Predicate`. */
function predicateFires(predicate: Predicate, lhs: number): boolean {
  const rhs = predicate.value;
  if (typeof rhs !== 'number') {
    // Mock can't reason about non-numeric comparisons here.
    return false;
  }
  switch (predicate.op) {
    case 'eq':
      return lhs === rhs;
    case 'ne':
      return lhs !== rhs;
    case 'lt':
      return lhs < rhs;
    case 'lte':
      return lhs <= rhs;
    case 'gt':
      return lhs > rhs;
    case 'gte':
      return lhs >= rhs;
    default:
      return false;
  }
}
