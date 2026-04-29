/**
 * Pure validation helpers for the Steps tab. Lives outside the tab
 * component file so React Fast Refresh does not complain about mixing
 * components and helpers.
 */
import type { RequestType, ScenarioStep, SessionMode } from '../types';

/** Request types legal under each session mode (architecture §4). */
export function legalRequestTypes(mode: SessionMode): RequestType[] {
  return mode === 'session'
    ? ['INITIAL', 'UPDATE', 'TERMINATE']
    : ['EVENT'];
}

export function isLegalRequestType(
  mode: SessionMode,
  type: RequestType,
): boolean {
  return legalRequestTypes(mode).includes(type);
}

/**
 * Per-step validation surface. Returns a list of human-readable
 * problems (empty when the step is valid). Exists so the Save flow
 * can surface a single rolled-up validation error before posting an
 * invalid scenario at the schema. The schema's own `if/then/else` and
 * `anyOf` constraints are the contractual backstop; this is the
 * UX-friendly layer.
 */
export function validateStep(
  step: ScenarioStep,
  mode: SessionMode,
): string[] {
  const problems: string[] = [];
  if (step.kind === 'request') {
    if (!isLegalRequestType(mode, step.requestType)) {
      problems.push(
        `Request type ${step.requestType} is not legal under sessionMode ${mode}`,
      );
    }
    if (step.repeat) {
      // Schema: repeat is UPDATE-only (if/then/else).
      if (step.requestType !== 'UPDATE') {
        problems.push(
          `Repeat policy is only valid on UPDATE steps (got ${step.requestType})`,
        );
      }
      // Schema: anyOf [times, until] — at least one bound must be set.
      const hasTimes =
        typeof step.repeat.times === 'number' && step.repeat.times >= 1;
      const hasUntil = step.repeat.until !== undefined;
      if (!hasTimes && !hasUntil) {
        problems.push(
          'Repeat policy needs either a times cap or a stop-when condition — unbounded loops are not allowed',
        );
      }
      // Predicate sanity (when present): variable must be non-empty.
      if (
        step.repeat.until &&
        (!step.repeat.until.variable || step.repeat.until.variable.trim() === '')
      ) {
        problems.push('Repeat stop-when condition is missing a variable');
      }
    }
  }
  return problems;
}

/** Aggregate `validateStep` over a step list with positional context. */
export function validateSteps(
  steps: readonly ScenarioStep[],
  mode: SessionMode,
): string[] {
  const out: string[] = [];
  steps.forEach((step, i) => {
    for (const problem of validateStep(step, mode)) {
      out.push(`Step ${i + 1}: ${problem}`);
    }
  });
  return out;
}
