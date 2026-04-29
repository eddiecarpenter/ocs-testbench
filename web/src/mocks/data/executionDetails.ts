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
      request: buildSampleRequest(label, i),
      response: buildSampleResponse(label, i, isTerminalFail),
      assertionResults: buildSampleAssertions(label, isTerminalFail),
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
      request: buildSampleRequest(STEP_TEMPLATE[i], i),
      response: buildSampleResponse(STEP_TEMPLATE[i], i, false),
      assertionResults: buildSampleAssertions(STEP_TEMPLATE[i], false),
    });
    cursor = finished;
  }
  return out;
}

// ---------------------------------------------------------------------------
// CCA / CCR sample payloads — three variants exercise the renderer:
//   - SUCCESS    (Result-Code 2001, all extractions populated)
//   - ASSERTION  (Result-Code 2001 but an assertion fails)
//   - EXTRACTION (Result-Code 2001 but a key extraction returns null)
// Tasks 6 + 8 lean on these so the right pane renders meaningful state
// without a real engine.
// ---------------------------------------------------------------------------

function buildSampleRequest(label: string, i: number): Record<string, unknown> {
  return {
    sessionId: `mock-session-${(1000 + i).toString(16)}`,
    requestType: requestTypeFor(label),
    msisdn: '27821234567',
    ccRequestNumber: i,
    'CC-Total-Octets': 1_048_576,
  };
}

function buildSampleResponse(
  label: string,
  i: number,
  isTerminalFail: boolean,
): Record<string, unknown> {
  if (isTerminalFail) {
    return {
      resultCode: 5012,
      requestType: requestTypeFor(label),
      ccRequestNumber: i,
      // No grant on the terminal-fail variant — extraction should
      // render `(not set)`.
      extractions: { GRANTED_TOTAL: null, EXPIRY: null },
    };
  }
  // Inject one ASSERTION-failure variant on update step 5 and one
  // EXTRACTION-failure variant on update step 7 so the renderer is
  // exercised across all three states without breaking the run.
  const assertionFailIndex = 4; // CCR-U (1) → CCA-U (1)
  const extractionFailIndex = 6; // CCR-U (2) → CCA-U (2)
  if (i === assertionFailIndex) {
    return {
      resultCode: 2001,
      requestType: requestTypeFor(label),
      ccRequestNumber: i,
      'CC-Total-Octets': 524_288,
      extractions: { GRANTED_TOTAL: 524_288, EXPIRY: '2026-04-29T10:30:00Z' },
    };
  }
  if (i === extractionFailIndex) {
    return {
      resultCode: 2001,
      requestType: requestTypeFor(label),
      ccRequestNumber: i,
      // Engine missed a Validity-Time extraction.
      extractions: { GRANTED_TOTAL: 1_048_576, EXPIRY: null },
    };
  }
  return {
    resultCode: 2001,
    requestType: requestTypeFor(label),
    ccRequestNumber: i,
    'CC-Total-Octets': 1_048_576,
    extractions: {
      GRANTED_TOTAL: 1_048_576,
      EXPIRY: '2026-04-29T11:00:00Z',
    },
  };
}

function buildSampleAssertions(
  label: string,
  isTerminalFail: boolean,
): NonNullable<StepRecord['assertionResults']> {
  if (isTerminalFail) {
    return [
      {
        expression: 'response.resultCode == 2001',
        passed: false,
        message: 'Expected 2001 but got 5012',
      },
    ];
  }
  // Per-step canonical assertion. Update step 5 (CCR-U / CCA-U pair
  // index 4) carries an extra failing assertion so the assertion-
  // failure variant is reachable from the fixture set.
  const out: NonNullable<StepRecord['assertionResults']> = [
    {
      expression: 'response.resultCode == 2001',
      passed: true,
    },
  ];
  if (label === 'CCA-U (1)') {
    out.push({
      expression: 'response.granted.total >= 1MiB',
      passed: false,
      message: 'Granted only 512 KiB; expected at least 1 MiB',
    });
  }
  return out;
}

function requestTypeFor(label: string): string {
  if (label.startsWith('CCR-I') || label.startsWith('CCA-I')) return 'INITIAL';
  if (label.startsWith('CCR-U') || label.startsWith('CCA-U')) return 'UPDATE';
  if (label.startsWith('CCR-T') || label.startsWith('CCA-T')) return 'TERMINATE';
  return label;
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
