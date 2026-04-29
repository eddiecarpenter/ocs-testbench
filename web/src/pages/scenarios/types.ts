/**
 * Re-exports of every Scenario-shaped type from the generated OpenAPI
 * client so feature code never hand-rolls a scenario type.
 *
 * Source of truth: `src/api/schema.d.ts` (regenerated from OpenAPI v0.2
 * via `npm run gen:api`). If a field is wrong here, fix the spec and
 * re-run the generator — do not patch the type locally.
 */
import type { components } from '../../api/schema';

export type Scenario = components['schemas']['Scenario'];
export type ScenarioInput = components['schemas']['ScenarioInput'];
export type ScenarioSummary = components['schemas']['ScenarioSummary'];
export type ScenarioDuplicateInput =
  components['schemas']['ScenarioDuplicateInput'];
export type ScenarioOrigin = components['schemas']['ScenarioOrigin'];

export type UnitType = components['schemas']['UnitType'];
export type SessionMode = components['schemas']['SessionMode'];
export type ServiceModel = components['schemas']['ServiceModel'];
export type RequestType = components['schemas']['RequestType'];

export type AvpNode = components['schemas']['AvpNode'];
export type Service = components['schemas']['Service'];
export type Variable = components['schemas']['Variable'];
export type VariableSource = components['schemas']['VariableSource'];
export type VariableSourceGenerator =
  components['schemas']['VariableSourceGenerator'];
export type VariableSourceBound = components['schemas']['VariableSourceBound'];
export type VariableSourceExtracted =
  components['schemas']['VariableSourceExtracted'];
export type GeneratorStrategy = components['schemas']['GeneratorStrategy'];
export type GeneratorRefresh = components['schemas']['GeneratorRefresh'];

export type ScenarioStep = components['schemas']['ScenarioStep'];
export type RequestStep = components['schemas']['RequestStep'];
export type ConsumeStep = components['schemas']['ConsumeStep'];
export type WaitStep = components['schemas']['WaitStep'];
export type PauseStep = components['schemas']['PauseStep'];
export type ServiceSelection = components['schemas']['ServiceSelection'];
export type VarValue = components['schemas']['VarValue'];

/**
 * Repeat policy for an UPDATE step. Causes the engine to expand the
 * step at runtime into a series of CCR-UPDATEs separated by
 * `delayMs`, bounded by `times` and/or `until`.
 *
 * The schema-level `if/then/else` constraint scopes this to UPDATE
 * steps only — INITIAL / TERMINATE / EVENT cannot carry it.
 */
export type UpdateRepeatPolicy =
  components['schemas']['UpdateRepeatPolicy'];

/**
 * Predicate used by `repeat.until`. Either a single comparison or an
 * OR-list of comparisons (loop exits when ANY sub-comparison fires —
 * lets authors express multiple independent stop conditions).
 */
export type Predicate = components['schemas']['Predicate'];
export type PredicateComparison = components['schemas']['PredicateComparison'];
export type PredicateAny = components['schemas']['PredicateAny'];

/**
 * Type guard — `true` when the predicate is the OR-list shape. Used
 * by editors and evaluators that want to handle both shapes
 * uniformly without committing to one in storage.
 */
export function isPredicateAny(p: Predicate): p is PredicateAny {
  return (p as PredicateAny).any !== undefined;
}

/**
 * Normalise either predicate shape into an array of comparisons. A
 * bare comparison becomes a 1-element list. Editors and simulators
 * read this and treat the predicate uniformly.
 */
export function predicateComparisons(p: Predicate): PredicateComparison[] {
  return isPredicateAny(p) ? p.any : [p];
}

/**
 * Build a Predicate from an array of comparisons. Inverse of
 * `predicateComparisons` — single-element arrays unwrap to a bare
 * `PredicateComparison` so simple cases keep their natural shape on
 * disk.
 */
export function predicateFromComparisons(
  list: PredicateComparison[],
): Predicate | undefined {
  if (list.length === 0) return undefined;
  if (list.length === 1) return list[0];
  return { any: list };
}

export type Problem = components['schemas']['Problem'];
