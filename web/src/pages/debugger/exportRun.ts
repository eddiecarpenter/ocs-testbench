/**
 * Pure helpers for the "Export run" affordance.
 *
 * Builds the JSON shape the user downloads when the run is in a
 * terminal state. Live in their own file so the shape is unit-testable
 * and the React component stays focused on the click + Blob plumbing.
 */
import type {
  Execution,
  ExecutionContextSnapshot,
  StepRecord,
} from '../../api/resources/executions';

/**
 * Stable export shape — keeps the JSON download contract distinct
 * from the underlying schema (which can churn). Bumping
 * `exportVersion` is how a future change communicates.
 */
export interface ExportPayload {
  exportVersion: 1;
  execution: ExportExecution;
  steps: StepRecord[];
  responses: Array<{ stepN: number; response: Record<string, unknown> | undefined }>;
  extractedVariables: ExecutionContextSnapshot['extracted'];
  totals: ExportTotals;
}

export interface ExportExecution {
  id: string;
  scenarioId: string;
  scenarioName: string;
  mode: Execution['mode'];
  state: Execution['state'];
  startedAt: string;
  finishedAt?: string;
  peerName?: string;
  subscriberMsisdn?: string;
}

export interface ExportTotals {
  steps: number;
  passed: number;
  failed: number;
  skipped: number;
  durationMs: number;
}

/**
 * Build the export payload from an `Execution` snapshot. Pure —
 * deterministic for the same input, regardless of when it's called.
 */
export function buildExportPayload(execution: Execution): ExportPayload {
  const steps = execution.steps ?? [];
  return {
    exportVersion: 1,
    execution: {
      id: execution.id,
      scenarioId: execution.scenarioId,
      scenarioName: execution.scenarioName,
      mode: execution.mode,
      state: execution.state,
      startedAt: execution.startedAt,
      ...(execution.finishedAt ? { finishedAt: execution.finishedAt } : {}),
      ...(execution.peerName ? { peerName: execution.peerName } : {}),
      ...(execution.subscriberMsisdn
        ? { subscriberMsisdn: execution.subscriberMsisdn }
        : {}),
    },
    steps,
    responses: steps.map((s) => ({
      stepN: s.n,
      response: s.response as Record<string, unknown> | undefined,
    })),
    extractedVariables: execution.context.extracted,
    totals: computeTotals(steps),
  };
}

function computeTotals(steps: ReadonlyArray<StepRecord>): ExportTotals {
  let passed = 0;
  let failed = 0;
  let skipped = 0;
  let durationMs = 0;
  for (const s of steps) {
    if (s.state === 'success') passed += 1;
    if (s.state === 'failure' || s.state === 'error') failed += 1;
    if (s.state === 'skipped') skipped += 1;
    if (s.durationMs !== undefined) durationMs += s.durationMs;
  }
  return { steps: steps.length, passed, failed, skipped, durationMs };
}

/**
 * Build a default download filename for the export. Pure — the
 * timestamp is *not* embedded so two exports of the same run produce
 * identical filenames (idempotent).
 */
export function exportFilename(executionId: string): string {
  return `execution-${executionId}.json`;
}

/**
 * Stringify the export payload with a stable indent. Keeps tests
 * readable.
 */
export function exportToString(payload: ExportPayload): string {
  return JSON.stringify(payload, null, 2);
}
