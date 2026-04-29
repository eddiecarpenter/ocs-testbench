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
import {
  expandScenarioSteps,
  type ExpandedStep,
} from '../../pages/scenarios/expandSteps';
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

/**
 * Display label for an expanded step — matches the SSE driver's
 * `deriveLabel` shorthand (`CCR-I` / `CCR-U` / `CCR-T` / `CCR-E`)
 * and suffixes `(round N/M)` for repeating UPDATE rounds.
 */
function labelForExpanded(slot: ExpandedStep): string {
  const step = slot.source;
  if (step.kind === 'request' && step.requestType) {
    const base = (() => {
      switch (step.requestType) {
        case 'INITIAL':
          return 'CCR-I';
        case 'UPDATE':
          return 'CCR-U';
        case 'TERMINATE':
          return 'CCR-T';
        case 'EVENT':
          return 'CCR-E';
        default:
          return step.requestType;
      }
    })();
    if (slot.roundIndex && slot.totalRounds) {
      return `${base} (round ${slot.roundIndex}/${slot.totalRounds})`;
    }
    return base;
  }
  if (step.kind === 'pause') return step.label ?? 'pause';
  if (step.kind === 'wait') return 'wait';
  if (step.kind === 'consume') return 'consume';
  return step.kind;
}

/**
 * Step count for an execution's source scenario, after `repeat`
 * expansion. Used by the executions list to render `n / total`.
 */
export function defaultTotalStepsForScenario(scenarioId: string): number {
  return expandScenarioSteps(scenarioFor(scenarioId).steps).length;
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
 * Pick the per-slot variant used to colour the CCR/CCA payloads.
 *
 *   - The LAST timeline slot of a failure-outcome run → `terminal-fail` (red).
 *   - Any UPDATE slot → `assertion-fail` so the right pane's failed-
 *     assertion branch is reachable on a *successful* run.
 *   - Everything else → `success`.
 */
function variantForExpanded(
  expanded: ExpandedStep[],
  i: number,
  outcome: 'success' | 'failure',
): 'success' | 'assertion-fail' | 'terminal-fail' {
  const lastIndex = expanded.length - 1;
  if (outcome === 'failure' && i === lastIndex) return 'terminal-fail';
  const step = expanded[i].source;
  if (step.kind === 'request' && step.requestType === 'UPDATE') {
    // For repeating UPDATEs, only flag the first round as assertion-
    // fail so the right pane has one example without burying it under
    // every round of a noisy scenario.
    const slot = expanded[i];
    if (!slot.roundIndex || slot.roundIndex === 1) return 'assertion-fail';
  }
  return 'success';
}

function recordFor(
  slot: ExpandedStep,
  i: number,
  variant: 'success' | 'assertion-fail' | 'terminal-fail',
  startedAt: string,
  finishedAt: string,
  durationMs: number,
  state: StepRecord['state'],
): StepRecord {
  const step = slot.source;
  const isTerminalFail = variant === 'terminal-fail';
  return {
    n: i + 1,
    kind: step.kind,
    ...(step.kind === 'request' && step.requestType
      ? { requestType: step.requestType }
      : {}),
    label: labelForExpanded(slot),
    state,
    startedAt,
    finishedAt,
    durationMs,
    request: buildSampleRequest(step, i),
    response: buildSampleResponse(step, i, variant),
    assertionResults: buildSampleAssertions(variant),
    ...(isTerminalFail
      ? { errorDetail: 'DIAMETER_UNABLE_TO_COMPLY (5012)' }
      : {}),
  };
}

function buildCompletedSteps(
  scenario: Scenario,
  startedAtMs: number,
  finishedAtMs: number,
  outcome: 'success' | 'failure',
): StepRecord[] {
  const expanded = expandScenarioSteps(scenario.steps);
  const total = expanded.length;
  const span = Math.max(1, finishedAtMs - startedAtMs);
  const bucket = Math.max(1, Math.floor(span / total));
  return expanded.map((slot, i): StepRecord => {
    const started = startedAtMs + i * bucket;
    const finished = i === total - 1 ? finishedAtMs : started + bucket;
    const variant = variantForExpanded(expanded, i, outcome);
    const isTerminalFail = variant === 'terminal-fail';
    return recordFor(
      slot,
      i,
      variant,
      new Date(started).toISOString(),
      new Date(finished).toISOString(),
      finished - started,
      isTerminalFail ? 'failure' : 'success',
    );
  });
}

function buildRunningSteps(
  scenario: Scenario,
  startedAtMs: number,
  completedSoFar: number,
): StepRecord[] {
  const expanded = expandScenarioSteps(scenario.steps);
  const out: StepRecord[] = [];
  let cursor = startedAtMs;
  const limit = Math.min(completedSoFar, expanded.length);
  for (let i = 0; i < limit; i++) {
    const slot = expanded[i];
    const variant = variantForExpanded(expanded, i, 'success');
    const duration = 20 + ((i * 7) % 35); // deterministic-ish jitter
    const started = cursor;
    const finished = cursor + duration;
    out.push(
      recordFor(
        slot,
        i,
        variant,
        new Date(started).toISOString(),
        new Date(finished).toISOString(),
        duration,
        'success',
      ),
    );
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
  // Total reflects the post-expansion timeline so a repeating UPDATE
  // counts each round.
  const total = expandScenarioSteps(scenario.steps).length;

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
