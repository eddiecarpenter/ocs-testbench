/**
 * Zustand draft store for the Scenario Builder.
 *
 * Owns the in-memory editing draft (TanStack Query owns the persisted
 * server state). Tabs read selectors off this store; mutations go
 * through dispatcher actions. Save coalesces the draft into a TanStack
 * mutation; on success the consumer calls `markSaved` which clears
 * the dirty flag and the undo/redo stacks.
 *
 * Capacity 100 history snapshots; the middleware records on store
 * action commit, not on every keystroke. Debouncing of text edits
 * is the consumer's responsibility (Task 3).
 */
import { create } from 'zustand';

import { createHistory } from './history';
import type {
  AvpNode,
  Scenario,
  ScenarioStep,
  Service,
  Variable,
} from './types';

export interface ScenarioDraftState {
  /** Currently-edited scenario, or `null` when no scenario is loaded. */
  draft: Scenario | null;
  /** True when `draft` differs from the last saved server state. */
  dirty: boolean;
  /** Internal history controller (kept on the store for ergonomic access). */
  _history: ReturnType<typeof createHistory>;

  // ------------------------------------------------------------------
  // lifecycle
  // ------------------------------------------------------------------

  /** Load a freshly-fetched scenario as the draft. Clears dirty + history. */
  load: (scenario: Scenario) => void;
  /** Drop the draft (Builder unmounted / user navigated away). */
  reset: () => void;
  /** Mark the draft as saved (mutation succeeded). Clears dirty + history. */
  markSaved: (scenario: Scenario) => void;

  // ------------------------------------------------------------------
  // header dispatchers
  // ------------------------------------------------------------------

  setName: (name: string) => void;
  setDescription: (description: string) => void;
  setUnitType: (unitType: Scenario['unitType']) => void;
  setSessionMode: (sessionMode: Scenario['sessionMode']) => void;
  setServiceModel: (serviceModel: Scenario['serviceModel']) => void;
  setSubscriberId: (subscriberId: string | undefined) => void;
  setPeerId: (peerId: string | undefined) => void;
  setFavourite: (favourite: boolean) => void;

  // ------------------------------------------------------------------
  // collection dispatchers
  // ------------------------------------------------------------------

  setSteps: (steps: ScenarioStep[]) => void;
  setAvpTree: (avpTree: AvpNode[]) => void;
  setServices: (services: Service[]) => void;
  setVariables: (variables: Variable[]) => void;

  // ------------------------------------------------------------------
  // history
  // ------------------------------------------------------------------

  undo: () => void;
  redo: () => void;
  canUndo: () => boolean;
  canRedo: () => boolean;
}

/**
 * Apply a mutation to the current draft, recording the *previous* state
 * onto the undo stack first. No-op when no draft is loaded.
 */
function commit(
  set: (
    fn: (prev: ScenarioDraftState) => Partial<ScenarioDraftState>,
  ) => void,
  get: () => ScenarioDraftState,
  mutate: (draft: Scenario) => Scenario,
) {
  const cur = get();
  if (!cur.draft) return;
  cur._history.record(cur.draft);
  set(() => ({ draft: mutate(cur.draft as Scenario), dirty: true }));
}

export const useScenarioDraftStore = create<ScenarioDraftState>((set, get) => ({
  draft: null,
  dirty: false,
  _history: createHistory(),

  load(scenario) {
    const history = createHistory();
    set(() => ({ draft: structuredClone(scenario), dirty: false, _history: history }));
  },
  reset() {
    set(() => ({ draft: null, dirty: false, _history: createHistory() }));
  },
  markSaved(scenario) {
    const history = createHistory();
    set(() => ({ draft: structuredClone(scenario), dirty: false, _history: history }));
  },

  setName: (name) => commit(set, get, (d) => ({ ...d, name })),
  setDescription: (description) =>
    commit(set, get, (d) => ({ ...d, description })),
  setUnitType: (unitType) => commit(set, get, (d) => ({ ...d, unitType })),
  setSessionMode: (sessionMode) =>
    commit(set, get, (d) => ({ ...d, sessionMode })),
  setServiceModel: (serviceModel) =>
    commit(set, get, (d) => ({ ...d, serviceModel })),
  setSubscriberId: (subscriberId) =>
    commit(set, get, (d) => ({ ...d, subscriberId: subscriberId ?? '' })),
  setPeerId: (peerId) =>
    commit(set, get, (d) => ({ ...d, peerId: peerId ?? '' })),
  setFavourite: (favourite) => commit(set, get, (d) => ({ ...d, favourite })),

  setSteps: (steps) => commit(set, get, (d) => ({ ...d, steps })),
  setAvpTree: (avpTree) => commit(set, get, (d) => ({ ...d, avpTree })),
  setServices: (services) => commit(set, get, (d) => ({ ...d, services })),
  setVariables: (variables) => commit(set, get, (d) => ({ ...d, variables })),

  undo() {
    const cur = get();
    if (!cur.draft) return;
    const prev = cur._history.undo(cur.draft);
    if (prev) set(() => ({ draft: prev, dirty: true }));
  },
  redo() {
    const cur = get();
    if (!cur.draft) return;
    const next = cur._history.redo(cur.draft);
    if (next) set(() => ({ draft: next, dirty: true }));
  },
  canUndo: () => get()._history.canUndo(),
  canRedo: () => get()._history.canRedo(),
}));
