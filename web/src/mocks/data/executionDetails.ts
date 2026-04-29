/**
 * Mock execution-detail builder.
 *
 * Step records are derived from the source scenario's actual step list
 * — one `StepRecord` per `scenario.steps[i]` with both the request
 * (CCR) and the response (CCA) carried as `request` / `response`
 * payloads on the same record. This matches the architecture (§4 —
 * a step is a *send*; the CCA is part of that step's response, not a
 * separate step) and matches what the SSE driver emits, so the
 * snapshot-then-SSE handoff is coherent.
 *
 * Failures land on the LAST step (so the run reads naturally), and
 * one assertion-failure variant is injected on the UPDATE step (when
 * present) so the right pane's red-✗ branch is reachable.
 */
import type {
  Execution,
  ExecutionContextSnapshot,
  StepRecord,
} from '../../api/resources/executions';
import type { Scenario } from '../../pages/scenarios/types';
import { executionFixtures } from './executions';
import { scenarioFixtures } from './scenarios';

/** Empty context snapshot — mock executions do not populate live variables. */
const EMPTY_CONTEXT: ExecutionContextSnapshot = {
  system: {},
  user: {},
  extracted: {},
};

/** Look up the source scenario for an execution; falls back to first fixture. */
function scenarioFor(scenarioId: string): Scenario {
  return (
    scenarioFixtures.find((s) => s.id === scenarioId) ?? scenarioFixtures[0]
  );
}

/** Display label for a step row. CCR-INITIAL / CCR-UPDATE / etc. for requests; kind for others. */
function labelFor(step: Scenario['steps'][number]): string {
  if (step.kind === 'request' && step.requestType) {
    return `CCR-${step.requestType}`;
  }
  if (step.kind === 'pause') return step.label ?? 'pause';
  if (step.kind === 'wait') return 'wait';
  if (step.kind === 'consume') return 'consume';
  return step.kind;
}

/** Step count for an execution's source scenario. Used by the executions list. */
export function defaultTotalStepsForScenario(scenarioId: string): number {
  return scenarioFor(scenarioId).steps.length;
}

// ---------------------------------------------------------------------------
// CCR / CCA sample payloads — three variants exercise the renderer:
//   - SUCCESS    (Result-Code 2001, all extractions populated)
//   - ASSERTION  (Result-Code 2001 but an assertion fails)
//   - EXTRACTION (Result-Code 2001 but a key extraction returns null —
//                 used on the final step of a `failure`-outcome run)
// ---------------------------------------------------------------------------

function buildSampleRequest(
  step: Scenario['steps'][number],
  i: number,
): Record<string, unknown> {
  return {
    sessionId: `mock-session-${(1000 + i).toString(16)}`,
    requestType: step.kind === 'request' ? step.requestType : step.kind,
    msisdn: '27821234567',
    ccRequestNumber: i,
    'CC-Total-Octets': 1_048_576,
  };
}

function buildSampleResponse(
  step: Scenario['steps'][number],
  i: number,
  variant: 'success' | 'assertion-fail' | 'terminal-fail',
): Record<string, unknown> {
  const requestType = step.kind === 'request' ? step.requestType : step.kind;
  if (variant === 'terminal-fail') {
    return {
      resultCode: 5012,
      requestType,
      ccRequestNumber: i,
      // No grant on terminal-fail — extractions render `(not set)`.
      extractions: { GRANTED_TOTAL: null, EXPIRY: null },
    };
  }
  if (variant === 'assertion-fail') {
    return {
      resultCode: 2001,
      requestType,
      ccRequestNumber: i,
      'CC-Total-Octets': 524_288,
      extractions: { GRANTED_TOTAL: 524_288, EXPIRY: '2026-04-29T10:30:00Z' },
    };
  }
  return {
    resultCode: 2001,
    requestType,
    ccRequestNumber: i,
    'CC-Total-Octets': 1_048_576,
    extractions: {
      GRANTED_TOTAL: 1_048_576,
      EXPIRY: '2026-04-29T11:00:00Z',
    },
  };
}

function buildSampleAssertions(
  variant: 'success' | 'assertion-fail' | 'terminal-fail',
): NonNullable<StepRecord['assertionResults']> {
  if (variant === 'terminal-fail') {
    return [
      {
        expression: 'response.resultCode == 2001',
        passed: false,
        message: 'Expected 2001 but got 5012',
      },
    ];
  }
  const out: NonNullable<StepRecord['assertionResults']> = [
    { expression: 'response.resultCode == 2001', passed: true },
  ];
  if (variant === 'assertion-fail') {
    out.push({
      expression: 'response.granted.total >= 1MiB',
      passed: false,
      message: 'Granted only 512 KiB; expected at least 1 MiB',
    });
  }
  return out;
}

/**
 * Pick the per-step variant used to colour the CCR/CCA payloads.
 *
 *   - The LAST step of a failure-outcome run → `terminal-fail` (red).
 *   - The UPDATE step (if the scenario has one) → `assertion-fail`
 *     so the right pane's failed-assertion branch is reachable on
 *     a *successful* run.
 *   - Everything else → `success`.
 */
function variantForStep(
  scenario: Scenario,
  i: number,
  outcome: 'success' | 'failure',
): 'success' | 'assertion-fail' | 'terminal-fail' {
  const lastIndex = scenario.steps.length - 1;
  if (outcome === 'failure' && i === lastIndex) return 'terminal-fail';
  const step = scenario.steps[i];
  if (step.kind === 'request' && step.requestType === 'UPDATE') {
    return 'assertion-fail';
  }
  return 'success';
}

function buildCompletedSteps(
  scenario: Scenario,
  startedAtMs: number,
  finishedAtMs: number,
  outcome: 'success' | 'failure',
): StepRecord[] {
  const total = scenario.steps.length;
  const span = Math.max(1, finishedAtMs - startedAtMs);
  const bucket = Math.max(1, Math.floor(span / total));
  return scenario.steps.map((step, i): StepRecord => {
    const started = startedAtMs + i * bucket;
    const finished = i === total - 1 ? finishedAtMs : started + bucket;
    const variant = variantForStep(scenario, i, outcome);
    const isTerminalFail = variant === 'terminal-fail';
    return {
      n: i + 1,
      kind: step.kind,
      ...(step.kind === 'request' && step.requestType
        ? { requestType: step.requestType }
        : {}),
      label: labelFor(step),
      state: isTerminalFail ? 'failure' : 'success',
      startedAt: new Date(started).toISOString(),
      finishedAt: new Date(finished).toISOString(),
      durationMs: finished - started,
      request: buildSampleRequest(step, i),
      response: buildSampleResponse(step, i, variant),
      assertionResults: buildSampleAssertions(variant),
      ...(isTerminalFail
        ? { errorDetail: 'DIAMETER_UNABLE_TO_COMPLY (5012)' }
        : {}),
    };
  });
}

function buildRunningSteps(
  scenario: Scenario,
  startedAtMs: number,
  completedSoFar: number,
): StepRecord[] {
  const out: StepRecord[] = [];
  let cursor = startedAtMs;
  const limit = Math.min(completedSoFar, scenario.steps.length);
  for (let i = 0; i < limit; i++) {
    const step = scenario.steps[i];
    const variant = variantForStep(scenario, i, 'success');
    const duration = 20 + ((i * 7) % 35); // deterministic-ish jitter
    const started = cursor;
    const finished = cursor + duration;
    out.push({
      n: i + 1,
      kind: step.kind,
      ...(step.kind === 'request' && step.requestType
        ? { requestType: step.requestType }
        : {}),
      label: labelFor(step),
      state: 'success',
      startedAt: new Date(started).toISOString(),
      finishedAt: new Date(finished).toISOString(),
      durationMs: duration,
      request: buildSampleRequest(step, i),
      response: buildSampleResponse(step, i, variant),
      assertionResults: buildSampleAssertions(variant),
    });
    cursor = finished;
  }
  return out;
}

/**
 * Build a full `Execution` detail record from a summary. Running
 * executions get a partial `steps` array up to
 * `completedStepsForRunning`; completed/failed executions get every
 * step.
 */
export function buildExecutionDetail(
  id: string,
  completedStepsForRunning?: number,
): Execution | undefined {
  const summary = executionFixtures.find((e) => e.id === id);
  if (!summary) return undefined;
  const scenario = scenarioFor(summary.scenarioId);
  const total = scenario.steps.length;

  if (summary.state === 'running') {
    const startedAtMs = Date.parse(summary.startedAt);
    const completed = completedStepsForRunning ?? defaultRunningProgress();
    const steps = buildRunningSteps(scenario, startedAtMs, completed);
    return {
      ...summary,
      currentStep: Math.min(completed, total),
      totalSteps: total,
      steps,
      context: EMPTY_CONTEXT,
    };
  }

  const startedAtMs = Date.parse(summary.startedAt);
  const finishedAtMs = Date.parse(summary.finishedAt ?? summary.startedAt);
  return {
    ...summary,
    currentStep: total,
    totalSteps: total,
    steps: buildCompletedSteps(
      scenario,
      startedAtMs,
      finishedAtMs,
      summary.state === 'failure' ? 'failure' : 'success',
    ),
    context: EMPTY_CONTEXT,
  };
}

/**
 * Initial "how many steps done" for each running fixture at load
 * time. With the per-scenario step count in play, just start every
 * running run at step 0 — the SSE driver advances them realistically.
 */
function defaultRunningProgress(): number {
  return 0;
}
