/**
 * Pure helpers for the Progress pane.
 *
 * Lives outside the React component so the click semantics and the
 * status-icon mapping are unit-testable without mounting a DOM (the
 * codebase's existing test setup runs against `node` and does not pull
 * in `@testing-library/react`).
 */
import type { StepRecord } from '../../api/resources/executions';

/** State key the row uses to pick the icon + colour. */
export type StepRowState =
  | 'success'
  | 'failure'
  | 'error'
  | 'skipped'
  | 'running'
  | 'paused'
  | 'pending';

/**
 * Resolve the visual state of a row from the `StepRecord` (if any),
 * the cursor flag, and the run's overall state. Encapsulates the rule
 * "the cursor on a paused run renders as paused, regardless of the
 * step record's `state` (which is `running` or `pending`)".
 */
export function computeRowState(
  record: StepRecord | undefined,
  isCursor: boolean,
  runState: string,
): StepRowState {
  if (!record) return isCursor && runState === 'paused' ? 'paused' : 'pending';
  switch (record.state) {
    case 'success':
      return 'success';
    case 'failure':
      return 'failure';
    case 'error':
      return 'error';
    case 'skipped':
      return 'skipped';
    case 'running':
      return isCursor && runState === 'paused' ? 'paused' : 'running';
    case 'pending':
    default:
      return isCursor && runState === 'paused' ? 'paused' : 'pending';
  }
}

/**
 * A step is "completed" — and therefore clickable for the historical
 * view — when its state is one of the terminal step states. `running`
 * is not clickable (it's the cursor itself); `pending` and `skipped`
 * are not clickable either (no historical CCR).
 */
export function isStepCompleted(record: StepRecord | undefined): boolean {
  if (!record) return false;
  return (
    record.state === 'success' ||
    record.state === 'failure' ||
    record.state === 'error'
  );
}

/**
 * Format the per-step duration for display. Sub-1s renders as `Nms`,
 * sub-1m as `Ns`, otherwise `MmSSs`.
 */
export function formatDuration(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  if (totalSec < 1) return `${ms}ms`;
  if (m === 0) return `${s}s`;
  return `${m}m ${String(s).padStart(2, '0')}s`;
}

/**
 * Translate the click on a row into the action the store should
 * receive — `null` for a no-op (pending / future steps), or the step
 * index for the historical view.
 */
export function clickAction(
  record: StepRecord | undefined,
  stepIndex: number,
): { kind: 'viewHistorical'; stepIndex: number } | null {
  if (!isStepCompleted(record)) return null;
  return { kind: 'viewHistorical', stepIndex };
}

export function kindLabel(kind: StepRecord['kind']): string {
  switch (kind) {
    case 'request':
      return 'Request';
    case 'consume':
      return 'Consume';
    case 'wait':
      return 'Wait';
    case 'pause':
      return 'Pause';
    default:
      return String(kind);
  }
}

export function kindColor(kind: StepRecord['kind']): string {
  switch (kind) {
    case 'request':
      return 'blue';
    case 'consume':
      return 'grape';
    case 'wait':
      return 'gray';
    case 'pause':
      return 'yellow';
    default:
      return 'gray';
  }
}
