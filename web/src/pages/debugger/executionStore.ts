/**
 * Page-scoped Zustand store for the Execution Debugger.
 *
 * One instance per `DebuggerPage` mount (multiple tabs must not share
 * state). Created via `createExecutionStore()` and exposed through a
 * React Context provider so child components can `useStore` the
 * nearest instance.
 *
 * The store mirrors the engine's authoritative state (per
 * `docs/ARCHITECTURE.md` §6) — invalid state-machine transitions
 * are no-ops with a `console.warn`. Server / SSE events drive the
 * lifecycle; the frontend never invents a transition.
 *
 * Most action bodies are stubs in Task 1 (the route + shell task);
 * later tasks fill them in:
 *   - Task 2 wires `ingestSse` (the per-execution mock SSE driver).
 *   - Task 3 + Task 5 wire `toggleService`, `regenerate` and the
 *     resolved `previewTree`.
 *   - Task 4 wires `viewHistorical`.
 *   - Task 7 wires `sendCcr`, `skip`, `pause`, `resume`, `runToEnd`,
 *     `stop`, `restart` (the imperative actions).
 */
import { createStore, type StoreApi } from 'zustand/vanilla';

import ApiService from '../../api/ApiService';
import type {
  Execution,
  ExecutionState,
  ExecutionContextSnapshot,
  StepRecord,
} from '../../api/resources/executions';

import type { PreviewAvpNode } from './ccrPreview';

// ---------------------------------------------------------------------------
// State + edit shape
// ---------------------------------------------------------------------------

/**
 * Local-only view-state for the live edit pane. Reset whenever the
 * cursor advances to a new step.
 */
export interface DebuggerEditState {
  /** Service `id`s currently toggled on for the next CCR. */
  servicesEnabled: Set<string>;
  /** Resolved CCR preview tree for the current step + edit state. */
  previewTree: PreviewAvpNode[] | null;
  /** True after any edit since the last regenerate / cursor advance. */
  dirty: boolean;
}

export interface ExecutionStoreState {
  /** Execution id this store is bound to (constant for the page lifetime). */
  executionId: string;
  /** Authoritative state machine — server-driven via REST + SSE. */
  state: ExecutionState;
  /** 0-based cursor; equals `steps.length` when the run is terminal. */
  cursor: number;
  /** Total scenario steps (mirrors `Execution.totalSteps`). */
  totalSteps: number;
  /** Step-by-step history. Grows over time for running executions. */
  steps: StepRecord[];
  /** Live context snapshot. Read-only in MVP. */
  context: ExecutionContextSnapshot;
  /**
   * Index of a step the user clicked in the Progress pane to inspect
   * historical data. `null` = follow the live cursor. Reset whenever
   * the cursor advances past it.
   */
  historicalIndex: number | null;
  /** Local edit state for the Step editor. */
  edit: DebuggerEditState;
  /** Last failure detail (rendered in the failure banner when terminal). */
  failureReason: string | null;

  // -------------------------------------------------------------------
  // actions — most are stubs in Task 1
  // -------------------------------------------------------------------

  /** Seed the store with a fresh `useExecution` snapshot. */
  ingestSnapshot: (snapshot: Execution) => void;
  /**
   * Apply an SSE event payload. Driven by the per-execution emitter
   * Task 2 lands; in Task 1 the action exists but is a no-op.
   */
  ingestSse: (event: SseEventPayload) => void;
  /** Toggle a service id on / off in the local edit state. */
  toggleService: (serviceId: string) => void;
  /** Re-resolve the preview tree from the current edit state. */
  regenerate: () => void;
  /** Replace the resolved preview tree (called by the pane). */
  setPreviewTree: (tree: PreviewAvpNode[] | null) => void;
  /**
   * Reset `edit.servicesEnabled` to the supplied default and clear
   * `dirty`. The pane computes the default from the current step's
   * scenario service selection, so the store stays scenario-agnostic.
   */
  revertEdit: (defaultServices: Set<string>) => void;
  /** Click a completed step in the Progress pane. */
  viewHistorical: (stepIndex: number | null) => void;

  // imperative actions — Task 7
  sendCcr: () => Promise<void>;
  skip: () => Promise<void>;
  pause: () => Promise<void>;
  resume: () => Promise<void>;
  runToEnd: () => Promise<void>;
  stop: () => Promise<void>;
  restart: () => Promise<void>;
}

/**
 * Discriminated union of every SSE event the debugger consumes.
 * Defined here (not in `api/sse/events.ts`) to keep the cross-page
 * SSE contract decoupled from this store's internal vocabulary —
 * Task 2 maps the wire events into this shape.
 */
export type SseEventPayload =
  | { type: 'execution.started'; data: Execution }
  | { type: 'step.sending'; data: { executionId: string; stepIndex: number } }
  | {
      type: 'step.responded';
      data: { executionId: string; step: StepRecord };
    }
  | { type: 'execution.paused'; data: { executionId: string; atStepIndex: number } }
  | { type: 'execution.resumed'; data: { executionId: string; fromStepIndex: number } }
  | { type: 'execution.completed'; data: Execution }
  | { type: 'execution.failed'; data: Execution & { failureReason?: string } }
  | { type: 'execution.aborted'; data: Execution };

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

const VALID_TRANSITIONS: Readonly<Record<ExecutionState, readonly ExecutionState[]>> = {
  pending: ['running', 'paused', 'aborted', 'error'],
  running: ['paused', 'success', 'failure', 'aborted', 'error'],
  paused: ['running', 'aborted', 'error'],
  // Terminal states have no outbound transitions.
  success: [],
  failure: [],
  aborted: [],
  error: [],
};

/**
 * Whether `next` is a legal transition from `current`. Same-state
 * "transitions" are accepted as no-ops so re-applying the current
 * snapshot doesn't warn.
 */
export function canTransition(
  current: ExecutionState,
  next: ExecutionState,
): boolean {
  if (current === next) return true;
  return VALID_TRANSITIONS[current].includes(next);
}

/**
 * Pure reducer — apply a candidate transition, returning the resolved
 * state (which equals `current` for invalid transitions). The `warn`
 * callback fires once per rejected transition; injected so tests can
 * spy without monkey-patching `console`.
 */
export function reduceTransition(
  current: ExecutionState,
  next: ExecutionState,
  warn: (msg: string) => void = (m) => console.warn(m),
): ExecutionState {
  if (canTransition(current, next)) return next;
  warn(
    `[executionStore] invalid transition ${current} → ${next}; ignoring (no-op).`,
  );
  return current;
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

const EMPTY_CONTEXT: ExecutionContextSnapshot = {
  system: {},
  user: {},
  extracted: {},
};

const EMPTY_EDIT: DebuggerEditState = {
  servicesEnabled: new Set<string>(),
  previewTree: null,
  dirty: false,
};

export type ExecutionStore = StoreApi<ExecutionStoreState>;

/**
 * Build a fresh store bound to `executionId`. Use one per page mount;
 * the store is disposed implicitly when the component tree owning the
 * provider unmounts.
 */
export function createExecutionStore(executionId: string): ExecutionStore {
  return createStore<ExecutionStoreState>((set, get) => ({
    executionId,
    state: 'pending',
    cursor: 0,
    totalSteps: 0,
    steps: [],
    context: EMPTY_CONTEXT,
    historicalIndex: null,
    edit: EMPTY_EDIT,
    failureReason: null,

    ingestSnapshot(snapshot) {
      const cur = get();
      const nextState = reduceTransition(cur.state, snapshot.state);
      set(() => ({
        state: nextState,
        cursor: snapshot.currentStep,
        totalSteps: snapshot.totalSteps,
        steps: snapshot.steps,
        context: snapshot.context,
        // A fresh snapshot resets transient edit state.
        edit: {
          servicesEnabled: new Set<string>(),
          previewTree: null,
          dirty: false,
        },
        // Following the live cursor on every snapshot is the right
        // default — historical inspection is opt-in per click.
        historicalIndex: null,
      }));
    },

    ingestSse(event: SseEventPayload) {
      const cur = get();
      switch (event.type) {
        case 'execution.started': {
          // The driver fires this immediately on install; the snapshot
          // ingestion path already seeded our state, so this is mostly
          // a confirmation. Promote `pending` → `running` if needed.
          set(() => ({
            state: reduceTransition(cur.state, 'running'),
          }));
          return;
        }
        case 'step.sending': {
          // Update cursor to the active step and flip to running. The
          // step's row turns into a spinner via the `running` state.
          set(() => ({
            state: reduceTransition(cur.state, 'running'),
            cursor: event.data.stepIndex,
          }));
          return;
        }
        case 'step.responded': {
          // Append the step record at its position. If a record at
          // that index already exists, replace it.
          const next = [...cur.steps];
          const idx = event.data.step.n - 1;
          next[idx] = event.data.step;
          set(() => ({
            steps: next,
            cursor: Math.max(cur.cursor, idx + 1),
          }));
          return;
        }
        case 'execution.paused': {
          set(() => ({
            state: reduceTransition(cur.state, 'paused'),
            cursor: event.data.atStepIndex,
          }));
          return;
        }
        case 'execution.resumed': {
          set(() => ({
            state: reduceTransition(cur.state, 'running'),
            cursor: event.data.fromStepIndex,
          }));
          return;
        }
        case 'execution.completed': {
          // Drive through running → success when arriving directly
          // from paused (legal because `paused → running → success`).
          let s = cur.state;
          if (s === 'paused') s = reduceTransition(s, 'running');
          s = reduceTransition(s, 'success');
          set(() => ({
            state: s,
            cursor: event.data.totalSteps,
            totalSteps: event.data.totalSteps,
            steps: event.data.steps,
            context: event.data.context,
          }));
          return;
        }
        case 'execution.failed': {
          let s = cur.state;
          if (s === 'paused') s = reduceTransition(s, 'running');
          s = reduceTransition(s, 'failure');
          set(() => ({
            state: s,
            cursor: event.data.totalSteps,
            totalSteps: event.data.totalSteps,
            steps: event.data.steps,
            failureReason: event.data.failureReason ?? null,
          }));
          return;
        }
        case 'execution.aborted': {
          set(() => ({
            state: reduceTransition(cur.state, 'aborted'),
            cursor: event.data.totalSteps,
            totalSteps: event.data.totalSteps,
            steps: event.data.steps,
          }));
          return;
        }
      }
    },

    // Stubbed in Task 1 — Task 5 fills in.
    toggleService(serviceId) {
      const cur = get();
      const next = new Set(cur.edit.servicesEnabled);
      if (next.has(serviceId)) next.delete(serviceId);
      else next.add(serviceId);
      set(() => ({
        edit: {
          ...cur.edit,
          servicesEnabled: next,
          dirty: true,
        },
      }));
    },

    regenerate() {
      // The pane re-runs the resolver on every edit/state change via
      // a useMemo + setPreviewTree effect; calling regenerate() is a
      // user-driven request to clear the dirty flag explicitly.
      const cur = get();
      set(() => ({ edit: { ...cur.edit, dirty: false } }));
    },

    setPreviewTree(tree) {
      const cur = get();
      set(() => ({ edit: { ...cur.edit, previewTree: tree } }));
    },

    revertEdit(defaultServices) {
      set(() => ({
        edit: {
          servicesEnabled: new Set(defaultServices),
          previewTree: null,
          dirty: false,
        },
      }));
    },

    viewHistorical(stepIndex) {
      set(() => ({ historicalIndex: stepIndex }));
    },

    // Imperative actions. Each posts to the corresponding endpoint
    // and immediately drives the local state machine so the UI
    // responds without waiting for the server's SSE round-trip
    // (which still arrives and reconciles via `ingestSse`). The
    // store is intentionally unaware of whether the run is mock or
    // real — the same code path runs in both worlds.
    async sendCcr() {
      const cur = get();
      // Optimistically flip to running; the next `step.responded`
      // SSE event will append the step record and freeze us here
      // again at `paused` (single-step semantics — see OpenAPI
      // `stepExecution`).
      set(() => ({ state: reduceTransition(cur.state, 'running') }));
      await ApiService.post<void>(
        `/executions/${encodeURIComponent(cur.executionId)}/step`,
      );
    },
    async skip() {
      const cur = get();
      // No CCR is sent — advance the cursor locally and confirm
      // server-side. The fakeApi `/skip` endpoint exists but is a
      // no-op on the engine until that surface lands.
      set(() => ({
        cursor: Math.min(cur.cursor + 1, cur.totalSteps),
      }));
      await ApiService.post<void>(
        `/executions/${encodeURIComponent(cur.executionId)}/skip`,
      );
    },
    async pause() {
      const cur = get();
      set(() => ({ state: reduceTransition(cur.state, 'paused') }));
      await ApiService.post<void>(
        `/executions/${encodeURIComponent(cur.executionId)}/pause`,
      );
    },
    async resume() {
      const cur = get();
      set(() => ({ state: reduceTransition(cur.state, 'running') }));
      await ApiService.post<void>(
        `/executions/${encodeURIComponent(cur.executionId)}/resume`,
      );
    },
    async runToEnd() {
      // Run-to-end is the same wire call as resume — the engine just
      // doesn't pause again until terminal. Naming differs in the UI
      // because the user sees "I'm letting it run".
      const cur = get();
      set(() => ({ state: reduceTransition(cur.state, 'running') }));
      await ApiService.post<void>(
        `/executions/${encodeURIComponent(cur.executionId)}/resume`,
      );
    },
    async stop() {
      const cur = get();
      // Optimistically transition. Aborted is a terminal state so the
      // store stops emitting elapsed ticks.
      set(() => ({ state: reduceTransition(cur.state, 'aborted') }));
      await ApiService.post<void>(
        `/executions/${encodeURIComponent(cur.executionId)}/abort`,
      );
    },
    async restart() {
      // Restart is the only action that creates a new execution; the
      // page-level handler owns the navigate, so the store action
      // throws — the caller must use `useCreateExecution` directly so
      // it can navigate to the new id. Kept on the surface for
      // completeness per Task 1.
      throw new Error('Restart is page-level — use useCreateExecution');
    },
  }));
}
