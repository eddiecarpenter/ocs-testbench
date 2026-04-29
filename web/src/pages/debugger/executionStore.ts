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

    // Stubbed in Task 1 — Task 2 fills in.
    ingestSse() {
      /* no-op until Task 2 */
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

    // Imperative actions — Task 7. Stubbed to satisfy the surface.
    async sendCcr() {
      /* no-op until Task 7 */
    },
    async skip() {
      /* no-op until Task 7 */
    },
    async pause() {
      /* no-op until Task 7 */
    },
    async resume() {
      /* no-op until Task 7 */
    },
    async runToEnd() {
      /* no-op until Task 7 */
    },
    async stop() {
      /* no-op until Task 7 */
    },
    async restart() {
      /* no-op until Task 7 */
    },
  }));
}
