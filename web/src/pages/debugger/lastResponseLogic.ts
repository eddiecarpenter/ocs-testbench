/**
 * Pure helpers for the Last-response pane.
 *
 * Responsible for:
 *   - picking which step record to render (historical view first,
 *     else the most recently completed step);
 *   - mapping a result-code value to a chip palette key;
 *   - shaping the assertion + extraction summaries the renderer
 *     consumes.
 *
 * Lives outside the React component so it can be unit-tested without
 * a DOM and without re-importing the resolver.
 */
import type { StepRecord } from '../../api/resources/executions';

// ---------------------------------------------------------------------------
// Picking the displayed step
// ---------------------------------------------------------------------------

/**
 * The most recently completed step record, given a `steps` history and
 * the live cursor. Returns `undefined` when no step has yet completed.
 */
export function lastCompletedStep(
  steps: ReadonlyArray<StepRecord>,
  cursor: number,
): StepRecord | undefined {
  // Walk backwards from `cursor - 1` and return the first record
  // with a "completed" state. Skipped / pending records are not
  // suitable for the response pane (no CCR payload to inspect).
  for (let i = Math.min(cursor - 1, steps.length - 1); i >= 0; i -= 1) {
    const r = steps[i];
    if (!r) continue;
    if (
      r.state === 'success' ||
      r.state === 'failure' ||
      r.state === 'error'
    ) {
      return r;
    }
  }
  return undefined;
}

/**
 * Resolve which step the pane should display.
 *
 * - When `historicalIndex` is non-null AND points at a completed step,
 *   that record wins.
 * - Otherwise, fall back to `lastCompletedStep`.
 */
export function pickDisplayedStep(
  steps: ReadonlyArray<StepRecord>,
  cursor: number,
  historicalIndex: number | null,
): StepRecord | undefined {
  if (historicalIndex !== null) {
    const cand = steps[historicalIndex];
    if (cand) {
      // Render historical even if the step state isn't a terminal one
      // — clicking a row on the Progress pane is the user's request,
      // and Task 4 already restricts clicks to completed rows.
      return cand;
    }
  }
  return lastCompletedStep(steps, cursor);
}

// ---------------------------------------------------------------------------
// Result-code palette
// ---------------------------------------------------------------------------

/**
 * Mantine palette key for a Diameter Result-Code chip.
 *
 * 2xxx (success) → teal; 4xxx (transient) → yellow; 5xxx (permanent)
 * → red; anything else → gray.
 */
export function resultCodeColor(code: number | undefined): string {
  if (code === undefined) return 'gray';
  if (code >= 2000 && code < 3000) return 'teal';
  if (code >= 4000 && code < 5000) return 'yellow';
  if (code >= 5000 && code < 6000) return 'red';
  return 'gray';
}

/**
 * Display label for a result-code chip — strips the protocol prefix
 * and adds the canonical name when known.
 */
export function resultCodeLabel(code: number | undefined): string {
  if (code === undefined) return '—';
  switch (code) {
    case 2001:
      return '2001 SUCCESS';
    case 4010:
      return '4010 END_USER_SERVICE_DENIED';
    case 4011:
      return '4011 CREDIT_CONTROL_NOT_APPLICABLE';
    case 4012:
      return '4012 CREDIT_LIMIT_REACHED';
    case 5012:
      return '5012 UNABLE_TO_COMPLY';
    case 5030:
      return '5030 USER_UNKNOWN';
    default:
      return String(code);
  }
}

// ---------------------------------------------------------------------------
// Response field extraction
// ---------------------------------------------------------------------------

/**
 * Pull the result-code value out of a wire-level response record. The
 * field is conventionally `resultCode` (camelCase) in mock fixtures;
 * Diameter's `Result-Code` AVP key is also accepted to cover real
 * traffic.
 */
export function extractResultCode(
  response: Record<string, unknown> | undefined,
): number | undefined {
  if (!response) return undefined;
  const candidates: Array<unknown> = [
    response.resultCode,
    (response['Result-Code'] as unknown),
  ];
  for (const v of candidates) {
    if (typeof v === 'number') return v;
  }
  return undefined;
}

/**
 * Pull the `extractions` map out of a wire-level response — the per-
 * step result of the scenario's variable-extraction rules. Mock
 * fixtures put it under `extractions: { name: value | null }` where
 * `null` signals an extraction that didn't fire (e.g. the source AVP
 * was missing).
 */
export function extractExtractions(
  response: Record<string, unknown> | undefined,
): Record<string, unknown> {
  if (!response) return {};
  const e = response.extractions;
  if (e && typeof e === 'object' && !Array.isArray(e)) {
    return e as Record<string, unknown>;
  }
  return {};
}

/** Best-effort byte size of a request / response object. JSON.stringify is
 *  good enough for mock-data display; the real engine sends precise sizes
 *  that will replace this when the backend lands. */
export function approximateSize(
  payload: Record<string, unknown> | undefined,
): number {
  if (!payload) return 0;
  try {
    return new TextEncoder().encode(JSON.stringify(payload)).length;
  } catch {
    return 0;
  }
}

/** Human-readable size — `123 B`, `1.2 KiB`. */
export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

/** Format `durationMs` as the response RTT — sub-1ms shows as `<1 ms`. */
export function formatRtt(ms: number | undefined): string {
  if (ms === undefined) return '—';
  if (ms < 1) return '<1 ms';
  return `${ms} ms`;
}
