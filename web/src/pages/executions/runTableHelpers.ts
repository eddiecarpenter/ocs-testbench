/**
 * Pure helpers for the Executions run table.
 *
 * Lives outside the React component so tests can validate the
 * formatting / sorting / progress logic without touching the DOM.
 */
import type {
  ExecutionMode,
  ExecutionState,
  ExecutionSummary,
} from '../../api/resources/executions';
import { TOTAL_STEPS } from '../../mocks/data/executionDetails';

/** Mantine palette key per execution state — matches the dashboard card. */
export const STATE_COLOR: Record<ExecutionState, string> = {
  pending: 'gray',
  running: 'blue',
  paused: 'yellow',
  success: 'teal',
  failure: 'red',
  aborted: 'orange',
  error: 'red',
};

/** User-facing label per execution state. Matches the design's chip copy. */
export const STATE_LABEL: Record<ExecutionState, string> = {
  pending: 'Pending',
  running: 'Running',
  paused: 'Paused',
  success: 'Completed',
  failure: 'Failed',
  aborted: 'Stopped',
  error: 'Error',
};

export function modeLabel(mode: ExecutionMode): string {
  return mode === 'continuous' ? 'Continuous' : 'Interactive';
}

const TERMINAL_STATES: ReadonlySet<ExecutionState> = new Set([
  'success',
  'failure',
  'aborted',
  'error',
]);

export function isTerminal(state: ExecutionState): boolean {
  return TERMINAL_STATES.has(state);
}

/**
 * Sort executions by `startedAt` descending (newest first). Pure —
 * returns a fresh array so callers don't mutate query data.
 */
export function sortByStartedDesc(
  rows: readonly ExecutionSummary[],
): ExecutionSummary[] {
  return [...rows].sort((a, b) => b.startedAt.localeCompare(a.startedAt));
}

/** Sortable column keys exposed by the run table. */
export type SortKey =
  | 'id'
  | 'scenarioName'
  | 'state'
  | 'mode'
  | 'subscriber'
  | 'peer'
  | 'progress'
  | 'duration'
  | 'startedAt';

export type SortDir = 'asc' | 'desc';

/** Duration in ms, or -1 when the run never finished — used for column sort. */
function durationMs(row: ExecutionSummary): number {
  if (!row.finishedAt) return -1;
  const diff = Date.parse(row.finishedAt) - Date.parse(row.startedAt);
  return Number.isNaN(diff) ? -1 : diff;
}

/**
 * Generic column sort. `progress` and `duration` sort on their numeric
 * representation (terminal-sibling count / ms). Pure — returns a fresh
 * array.
 */
export function sortRows(
  rows: readonly ExecutionSummary[],
  key: SortKey,
  dir: SortDir,
  runsByBatch: Map<string, ExecutionSummary[]>,
): ExecutionSummary[] {
  const sign = dir === 'asc' ? 1 : -1;

  const keyFn = (r: ExecutionSummary): string | number => {
    switch (key) {
      case 'id':
        return r.id;
      case 'scenarioName':
        return r.scenarioName ?? '';
      case 'state':
        return STATE_LABEL[r.state];
      case 'mode':
        return modeLabel(r.mode);
      case 'subscriber':
        return r.subscriberMsisdn ?? r.subscriberId ?? '';
      case 'peer':
        return r.peerName ?? r.peerId ?? '';
      case 'progress': {
        if (r.batchId) {
          const siblings = runsByBatch.get(r.batchId) ?? [];
          return siblings.filter((s) => isTerminal(s.state)).length;
        }
        return isTerminal(r.state) ? 1 : 0;
      }
      case 'duration':
        return durationMs(r);
      case 'startedAt':
        return r.startedAt;
      default:
        return '';
    }
  };

  return [...rows].sort((a, b) => {
    const av = keyFn(a);
    const bv = keyFn(b);
    if (typeof av === 'number' && typeof bv === 'number') {
      return (av - bv) * sign;
    }
    return String(av).localeCompare(String(bv)) * sign;
  });
}

/**
 * Group executions by `batchId` to compute `done / total` for
 * Continuous rows. The map is keyed by batchId; rows without a
 * batchId are not included.
 */
export function groupByBatch(
  rows: readonly ExecutionSummary[],
): Map<string, ExecutionSummary[]> {
  const map = new Map<string, ExecutionSummary[]>();
  for (const r of rows) {
    if (!r.batchId) continue;
    const arr = map.get(r.batchId);
    if (arr) arr.push(r);
    else map.set(r.batchId, [r]);
  }
  return map;
}

/**
 * Render the Progress cell text per AC-10 / AC-11.
 *
 *   Continuous + batched (batchId present): `<terminal-siblings> / <batch-size>`
 *                                            e.g. `23 / 50`
 *   Continuous + standalone:                `1 / 1` (terminal) or `0 / 1` (running)
 *   Interactive:                            `<step> / <steps>` — `12 / 12`
 *                                            for terminal rows, `… / 12`
 *                                            while running (live progress
 *                                            arrives via SSE in Feature #95).
 *
 * `runsByBatch` should come from `groupByBatch(<all-rows>)` so siblings
 * across the entire feed are visible to the cell.
 */
export function formatProgress(
  row: ExecutionSummary,
  runsByBatch: Map<string, ExecutionSummary[]>,
): string {
  if (row.mode === 'interactive') {
    return isTerminal(row.state)
      ? `${TOTAL_STEPS} / ${TOTAL_STEPS}`
      : `… / ${TOTAL_STEPS}`;
  }
  // Continuous
  if (row.batchId) {
    const siblings = runsByBatch.get(row.batchId) ?? [];
    const total = siblings.length;
    const done = siblings.filter((s) => isTerminal(s.state)).length;
    return `${done} / ${total}`;
  }
  return isTerminal(row.state) ? '1 / 1' : '0 / 1';
}

/** Format a row's duration as `mm:ss` (or `–` for runs that never finished). */
export function formatDuration(row: ExecutionSummary): string {
  if (!row.finishedAt) return '–';
  const diffMs = Date.parse(row.finishedAt) - Date.parse(row.startedAt);
  if (Number.isNaN(diffMs) || diffMs < 0) return '–';
  const totalSec = Math.floor(diffMs / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
}
