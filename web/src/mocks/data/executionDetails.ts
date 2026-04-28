import type {
  Execution,
  ExecutionContextSnapshot,
  StepRecord,
} from '../../api/resources/executions';
import { executionFixtures } from './executions';

/**
 * Canonical 12-step Diameter scenario template. Real scenarios would vary
 * per-execution, but for MVP every execution uses the same outline and
 * failures are injected by swapping one step's `state` to `failure`.
 */
const STEP_TEMPLATE = [
  'CER',
  'CEA',
  'CCR-I',
  'CCA-I',
  'CCR-U (1)',
  'CCA-U (1)',
  'CCR-U (2)',
  'CCA-U (2)',
  'CCR-U (3)',
  'CCA-U (3)',
  'CCR-T',
  'CCA-T',
];

/** Empty context snapshot — mock executions do not populate live variables. */
const EMPTY_CONTEXT: ExecutionContextSnapshot = {
  system: {},
  user: {},
  extracted: {},
};

function buildCompletedSteps(
  startedAtMs: number,
  finishedAtMs: number,
  outcome: 'success' | 'failure',
): StepRecord[] {
  const total = STEP_TEMPLATE.length;
  const span = Math.max(1, finishedAtMs - startedAtMs);
  const bucket = Math.floor(span / total);
  // For failures, flip the last step to failure so the step list reads naturally.
  return STEP_TEMPLATE.map((label, i): StepRecord => {
    const started = startedAtMs + i * bucket;
    const finished = i === total - 1 ? finishedAtMs : started + bucket;
    const isTerminalFail = outcome === 'failure' && i === total - 1;
    return {
      n: i + 1,
      kind: 'request',
      label,
      state: isTerminalFail ? 'failure' : 'success',
      startedAt: new Date(started).toISOString(),
      finishedAt: new Date(finished).toISOString(),
      durationMs: finished - started,
      ...(isTerminalFail
        ? { errorDetail: 'DIAMETER_UNABLE_TO_COMPLY (5012)' }
        : {}),
    };
  });
}

function buildRunningSteps(
  startedAtMs: number,
  completedSoFar: number,
): StepRecord[] {
  const out: StepRecord[] = [];
  let cursor = startedAtMs;
  for (let i = 0; i < completedSoFar; i++) {
    const duration = 20 + ((i * 7) % 35); // deterministic-ish jitter
    const started = cursor;
    const finished = cursor + duration;
    out.push({
      n: i + 1,
      kind: 'request',
      label: STEP_TEMPLATE[i],
      state: 'success',
      startedAt: new Date(started).toISOString(),
      finishedAt: new Date(finished).toISOString(),
      durationMs: duration,
    });
    cursor = finished;
  }
  return out;
}

/**
 * Build a full `Execution` detail record from a summary. Running executions
 * get a partial `steps` array; completed executions get all 12.
 *
 * `completedStepsForRunning` lets the mock SSE emitter advance a running
 * execution by calling this with incrementing values.
 */
export function buildExecutionDetail(
  id: string,
  completedStepsForRunning?: number,
): Execution | undefined {
  const summary = executionFixtures.find((e) => e.id === id);
  if (!summary) return undefined;

  if (summary.state === 'running') {
    const startedAtMs = Date.parse(summary.startedAt);
    const completed = completedStepsForRunning ?? defaultRunningProgress(id);
    const steps = buildRunningSteps(startedAtMs, completed);
    return {
      ...summary,
      currentStep: completed,
      totalSteps: STEP_TEMPLATE.length,
      steps,
      context: EMPTY_CONTEXT,
    };
  }

  const startedAtMs = Date.parse(summary.startedAt);
  const finishedAtMs = Date.parse(summary.finishedAt ?? summary.startedAt);
  return {
    ...summary,
    currentStep: STEP_TEMPLATE.length,
    totalSteps: STEP_TEMPLATE.length,
    steps: buildCompletedSteps(
      startedAtMs,
      finishedAtMs,
      summary.state === 'failure' ? 'failure' : 'success',
    ),
    context: EMPTY_CONTEXT,
  };
}

/** Initial "how many steps done" for each running fixture at load time. */
function defaultRunningProgress(id: string): number {
  if (id === '43') return 4;
  if (id === '44') return 1;
  return 0;
}

export const TOTAL_STEPS = STEP_TEMPLATE.length;
