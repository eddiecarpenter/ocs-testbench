/**
 * Per-execution mock SSE driver.
 *
 * Returns a handle whose lifecycle is owned by the Debugger page —
 * subscribe on mount, dispose on unmount. The handle exposes
 * `subscribe(handler)`, `pause()`, `resume()`, `dispose()`. Multiple
 * subscribers are supported (handler-set semantics) but the page only
 * needs one.
 *
 * The scheduler is `setTimeout`-based with pause / resume continuity:
 * `pause()` clears the pending timeout and stashes the elapsed-since-
 * tick offset; `resume()` schedules the remainder before reverting to
 * the regular cadence. The pause flag is **scoped to the driver
 * instance** — pausing one execution must not affect another (and the
 * existing app-wide `installMockSse` peer-status emitter keeps running
 * regardless).
 *
 * Event vocabulary mirrors `executionStore.SseEventPayload`:
 *   `execution.started`     — emitted immediately
 *   `step.sending` /
 *     `step.responded`     — paced for each scenario step
 *   `execution.completed` /
 *     `.failed` /
 *     `.aborted`           — terminal event
 *
 * For `mode === 'continuous'` scenarios the driver emits
 * `execution.batchStep` increments instead of the per-step pair, then
 * a terminal `execution.completed`.
 *
 * Tests drive the scheduler with `vi.useFakeTimers()` — see
 * `installExecutionSse.test.ts`.
 */
import type {
  Execution,
  ExecutionMode,
  ExecutionState,
  StepRecord,
} from '../../api/resources/executions';
import type { Scenario } from '../../pages/scenarios/types';
import type { SseEventPayload } from '../../pages/debugger/executionStore';
import {
  expandScenarioSteps,
  type ExpandedStep,
} from '../../pages/scenarios/expandSteps';

// ---------------------------------------------------------------------------
// Public surface
// ---------------------------------------------------------------------------

/**
 * Optional extension to the live SSE event vocabulary used only by the
 * per-execution mock driver. The store's wire vocabulary
 * (`SseEventPayload`) doesn't yet need a `batchStep` member — this
 * variant is kept as a sibling type so a continuous-mode subscriber
 * can opt in.
 */
export type ExecutionBatchStepEvent = {
  type: 'execution.batchStep';
  data: { executionId: string; n: number; total: number };
};

export type DriverEvent = SseEventPayload | ExecutionBatchStepEvent;

export type DriverEventHandler = (event: DriverEvent) => void;

export interface InstallExecutionSseHandle {
  /** Register a handler. Returns an unsubscribe fn. */
  subscribe(handler: DriverEventHandler): () => void;
  /** Suspend the scheduler. No-op when already paused or terminated. */
  pause(): void;
  /** Resume the scheduler from where it left off. No-op when running. */
  resume(): void;
  /** Cancel any pending timeout and detach all handlers. Idempotent. */
  dispose(): void;
}

export interface InstallExecutionSseOptions {
  /** Tick cadence in ms. Default 800. */
  tickMs?: number;
  /**
   * Total batched-iteration count for `continuous` mode. Default 5.
   * Ignored for `interactive` mode.
   */
  batchTotal?: number;
  /**
   * Optional scheduler injection — used by tests to substitute an
   * imperatively-controlled clock without monkey-patching globals.
   * Defaults to `setTimeout` / `clearTimeout` from `globalThis`.
   */
  scheduler?: SchedulerLike;
  /**
   * Outcome injected by the fixture variant — `'success'` (default),
   * `'failure'`, or `'aborted'`. Drives the terminal event.
   */
  outcome?: 'success' | 'failure' | 'aborted';
  /**
   * Wall-clock at which the execution started. Defaults to "now"
   * resolved at install. Used to stamp `startedAt` on emitted records.
   */
  startedAtMs?: number;
}

export interface SchedulerLike {
  setTimeout(handler: () => void, delayMs: number): unknown;
  clearTimeout(handle: unknown): void;
  now(): number;
}

const DEFAULT_TICK_MS = 800;
const DEFAULT_BATCH_TOTAL = 5;

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/**
 * Build a per-execution SSE driver bound to `executionId` + `scenario`.
 * Caller is responsible for `dispose()` — the driver leaks the timer
 * if abandoned.
 */
export function installExecutionSse(
  executionId: string,
  scenario: Scenario,
  mode: ExecutionMode,
  options: InstallExecutionSseOptions = {},
): InstallExecutionSseHandle {
  const tickMs = options.tickMs ?? DEFAULT_TICK_MS;
  const scheduler: SchedulerLike = options.scheduler ?? defaultScheduler();
  const outcome = options.outcome ?? 'success';
  const startedAtMs = options.startedAtMs ?? scheduler.now();
  const batchTotal = options.batchTotal ?? DEFAULT_BATCH_TOTAL;

  const handlers = new Set<DriverEventHandler>();

  // Plan: an ordered list of events the scheduler will fire one-tick
  // apart. The leading `execution.started` is fired synchronously from
  // `install()` so consumers see the run alive immediately.
  const plan: DriverEvent[] = buildPlan({
    executionId,
    scenario,
    mode,
    outcome,
    startedAtMs,
    batchTotal,
    tickMs,
  });

  // Scheduler state.
  let nextIndex = 0; // index into `plan` of the next event to fire
  let pendingHandle: unknown = null;
  let tickStartedAt = 0;
  let remainingMs = 0; // for resume after pause
  let pausedFlag = false;
  let disposed = false;

  function fan(event: DriverEvent): void {
    for (const h of handlers) h(event);
  }

  function scheduleNext(delay: number): void {
    if (disposed) return;
    if (nextIndex >= plan.length) return; // run finished
    tickStartedAt = scheduler.now();
    remainingMs = delay;
    pendingHandle = scheduler.setTimeout(() => {
      pendingHandle = null;
      const ev = plan[nextIndex];
      nextIndex += 1;
      fan(ev);
      // Schedule the next event at the regular cadence.
      scheduleNext(tickMs);
    }, delay);
  }

  function clearPending(): void {
    if (pendingHandle != null) {
      scheduler.clearTimeout(pendingHandle);
      pendingHandle = null;
    }
  }

  // Schedule `execution.started` for the next tick (delay 0) — NOT
  // synchronous. Firing it inside `install()` would deliver to an
  // empty handler set, since the caller can only register via
  // `.subscribe()` *after* `installExecutionSse` returns. After this
  // first event, the regular cadence (`tickMs`) takes over via
  // `scheduleNext` calling itself recursively from the timer
  // callback at line ~167.
  scheduleNext(0);

  return {
    subscribe(handler) {
      handlers.add(handler);
      return () => handlers.delete(handler);
    },

    pause() {
      if (disposed || pausedFlag) return;
      if (nextIndex >= plan.length) return; // already done
      // Stash the remaining time on the in-flight tick.
      const elapsed = scheduler.now() - tickStartedAt;
      remainingMs = Math.max(0, remainingMs - elapsed);
      clearPending();
      pausedFlag = true;
    },

    resume() {
      if (disposed || !pausedFlag) return;
      pausedFlag = false;
      if (nextIndex >= plan.length) return;
      // Honour the leftover before reverting to regular cadence (the
      // scheduleNext branch coming after the leftover fires uses
      // `tickMs` automatically).
      scheduleNext(remainingMs > 0 ? remainingMs : tickMs);
    },

    dispose() {
      if (disposed) return;
      disposed = true;
      clearPending();
      handlers.clear();
    },
  };
}

// ---------------------------------------------------------------------------
// Plan builder — pure
// ---------------------------------------------------------------------------

interface BuildPlanInput {
  executionId: string;
  scenario: Scenario;
  mode: ExecutionMode;
  outcome: 'success' | 'failure' | 'aborted';
  startedAtMs: number;
  batchTotal: number;
  tickMs: number;
}

/**
 * Build the ordered event sequence the scheduler will fire (excluding
 * the leading immediate `execution.started`). Pure so tests can pin
 * the schedule shape without invoking the scheduler.
 */
export function buildPlan(input: BuildPlanInput): DriverEvent[] {
  const out: DriverEvent[] = [];

  // Pre-expand the scenario steps so a repeating UPDATE produces one
  // timeline slot per round. Both the step-event indices and the
  // terminal `totalSteps` count off this expanded length, so the
  // Progress pane and the cursor stay coherent.
  const expanded = expandScenarioSteps(input.scenario.steps);

  // 1. Always lead with `execution.started` (paced[0] — fired
  //    synchronously by `installExecutionSse`).
  out.push({
    type: 'execution.started',
    data: snapshotForState(input, 'running', 0, [], expanded.length),
  });

  if (input.mode === 'continuous') {
    // Continuous: batchStep × N, then completed.
    for (let n = 1; n <= input.batchTotal; n += 1) {
      out.push({
        type: 'execution.batchStep',
        data: { executionId: input.executionId, n, total: input.batchTotal },
      });
    }
    out.push({
      type: 'execution.completed',
      data: snapshotForState(
        input,
        terminalState(input.outcome),
        expanded.length,
        completedSteps(input, expanded),
        expanded.length,
      ),
    });
    return out;
  }

  // Interactive: per-expanded-step (sending → responded), then terminal.
  const steps: StepRecord[] = [];
  for (let i = 0; i < expanded.length; i += 1) {
    out.push({
      type: 'step.sending',
      data: { executionId: input.executionId, stepIndex: i },
    });
    const step = buildStepRecord(input, expanded, i);
    steps.push(step);
    out.push({
      type: 'step.responded',
      data: { executionId: input.executionId, step },
    });
  }
  const total = expanded.length;
  // Terminal: the outcome decides which event variant fires.
  if (input.outcome === 'failure') {
    out.push({
      type: 'execution.failed',
      data: {
        ...snapshotForState(input, 'failure', total, steps, total),
        failureReason: 'Assertion failed (mock fixture)',
      },
    });
  } else if (input.outcome === 'aborted') {
    out.push({
      type: 'execution.aborted',
      data: snapshotForState(input, 'aborted', total, steps, total),
    });
  } else {
    out.push({
      type: 'execution.completed',
      data: snapshotForState(input, 'success', total, steps, total),
    });
  }
  return out;
}

function terminalState(outcome: 'success' | 'failure' | 'aborted'): ExecutionState {
  if (outcome === 'failure') return 'failure';
  if (outcome === 'aborted') return 'aborted';
  return 'success';
}

function snapshotForState(
  input: BuildPlanInput,
  state: ExecutionState,
  cursor: number,
  steps: StepRecord[],
  totalSteps: number,
): Execution {
  return {
    id: input.executionId,
    scenarioId: input.scenario.id,
    scenarioName: input.scenario.name,
    mode: input.mode,
    state,
    startedAt: new Date(input.startedAtMs).toISOString(),
    finishedAt:
      state === 'success' || state === 'failure' || state === 'aborted'
        ? new Date(
            input.startedAtMs + totalSteps * input.tickMs * 2,
          ).toISOString()
        : undefined,
    currentStep: cursor,
    totalSteps,
    steps,
    context: { system: {}, user: {}, extracted: {} },
  };
}

function buildStepRecord(
  input: BuildPlanInput,
  expanded: ExpandedStep[],
  i: number,
): StepRecord {
  const slot = expanded[i];
  const scenarioStep = slot.source;
  const startedAtMs = input.startedAtMs + i * input.tickMs * 2;
  const finishedAtMs = startedAtMs + input.tickMs;
  // Only the LAST timeline slot honours `failure` outcome; intermediate
  // steps always succeed. Mirrors the existing fixtures' behaviour.
  const lastIndex = expanded.length - 1;
  const isTerminalFail = input.outcome === 'failure' && i === lastIndex;
  const state: StepRecord['state'] = isTerminalFail ? 'failure' : 'success';

  const requestType =
    scenarioStep.kind === 'request' ? scenarioStep.requestType : undefined;

  return {
    n: i + 1,
    kind: scenarioStep.kind,
    label: deriveLabel(scenarioStep, requestType, slot),
    state,
    startedAt: new Date(startedAtMs).toISOString(),
    finishedAt: new Date(finishedAtMs).toISOString(),
    durationMs: input.tickMs,
    ...(requestType ? { requestType } : {}),
    ...(isTerminalFail
      ? {
          errorDetail: 'DIAMETER_UNABLE_TO_COMPLY (5012)',
          assertionResults: [
            {
              expression: 'response.resultCode == 2001',
              passed: false,
              message: 'Expected 2001 but got 5012',
            },
          ],
        }
      : {}),
  };
}

function deriveLabel(
  step: BuildPlanInput['scenario']['steps'][number],
  requestType: string | undefined,
  slot?: ExpandedStep,
): string {
  if (step.kind === 'request' && requestType) {
    // Map the RequestType enum to the wire-CCR shorthand the rest of
    // the UI uses ('CCR-I' / 'CCR-U' / 'CCR-T' / 'CCR-E').
    const base = (() => {
      switch (requestType) {
        case 'INITIAL':
          return 'CCR-I';
        case 'UPDATE':
          return 'CCR-U';
        case 'TERMINATE':
          return 'CCR-T';
        case 'EVENT':
          return 'CCR-E';
        default:
          return requestType;
      }
    })();
    // For repeating UPDATE rounds, suffix the round position so the
    // Progress pane reads `CCR-U (round 2/4)` rather than 4 identical
    // `CCR-U` rows. Single-shot UPDATEs render unsuffixed.
    if (slot?.roundIndex && slot?.totalRounds) {
      return `${base} (round ${slot.roundIndex}/${slot.totalRounds})`;
    }
    return base;
  }
  if (step.kind === 'pause') return step.label ?? 'pause';
  if (step.kind === 'wait') return 'wait';
  if (step.kind === 'consume') return 'consume';
  return step.kind;
}

function completedSteps(
  input: BuildPlanInput,
  expanded: ExpandedStep[],
): StepRecord[] {
  return expanded.map((_, i) => buildStepRecord(input, expanded, i));
}

// ---------------------------------------------------------------------------
// Default scheduler (real `setTimeout`).
// ---------------------------------------------------------------------------

function defaultScheduler(): SchedulerLike {
  return {
    setTimeout(handler, delayMs) {
      return globalThis.setTimeout(handler, delayMs);
    },
    clearTimeout(handle) {
      globalThis.clearTimeout(handle as ReturnType<typeof globalThis.setTimeout>);
    },
    now() {
      return Date.now();
    },
  };
}
