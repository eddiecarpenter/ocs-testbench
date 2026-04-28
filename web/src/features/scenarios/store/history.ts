/**
 * Bounded undo / redo history middleware for the scenario draft store.
 *
 * The contract is intentionally minimal:
 *
 * - `record(snapshot)`     — append a snapshot to the undo stack.
 *                             Clears the redo stack (per-edit branching is
 *                             not supported and rarely useful in builders).
 * - `undo()`               — pop the most recent snapshot off the undo
 *                             stack and push the *current* state onto
 *                             the redo stack; return the popped snapshot.
 * - `redo()`               — pop the most recent snapshot off the redo
 *                             stack and push the *current* state onto
 *                             the undo stack; return the popped snapshot.
 * - `clear()`              — drop both stacks (used after a successful Save).
 * - `canUndo()` / `canRedo()` — UI-state predicates.
 *
 * Capacity is bounded at 100 snapshots per stack; older entries are
 * dropped from the bottom. The middleware does not record on every
 * keystroke — input components debounce text edits to a single commit
 * on blur (Task 3 lands the debounce; Task 8 wires the hotkeys).
 */
import type { Scenario } from './types';

/** Capacity of each stack — bounded so the heap never grows unbounded. */
export const HISTORY_CAPACITY = 100;

export interface HistoryStacks {
  undo: Scenario[];
  redo: Scenario[];
}

export interface HistoryAPI {
  record: (snapshot: Scenario) => void;
  undo: (current: Scenario) => Scenario | null;
  redo: (current: Scenario) => Scenario | null;
  clear: () => void;
  canUndo: () => boolean;
  canRedo: () => boolean;
  /** Read-only view of the stacks for tests / debug. */
  snapshot: () => HistoryStacks;
}

/**
 * Construct a new history controller. Snapshots are stored as deep
 * structural clones via `structuredClone` — the controller assumes the
 * caller hands it a value safe to clone (no functions, no DOM nodes).
 */
export function createHistory(): HistoryAPI {
  const stacks: HistoryStacks = { undo: [], redo: [] };

  return {
    record(snapshot) {
      stacks.undo.push(structuredClone(snapshot));
      // Branching after an undo discards the redo stack — same convention
      // as VS Code, IntelliJ, every text editor.
      stacks.redo.length = 0;
      while (stacks.undo.length > HISTORY_CAPACITY) stacks.undo.shift();
    },
    undo(current) {
      const entry = stacks.undo.pop();
      if (entry === undefined) return null;
      stacks.redo.push(structuredClone(current));
      while (stacks.redo.length > HISTORY_CAPACITY) stacks.redo.shift();
      return entry;
    },
    redo(current) {
      const entry = stacks.redo.pop();
      if (entry === undefined) return null;
      stacks.undo.push(structuredClone(current));
      while (stacks.undo.length > HISTORY_CAPACITY) stacks.undo.shift();
      return entry;
    },
    clear() {
      stacks.undo.length = 0;
      stacks.redo.length = 0;
    },
    canUndo: () => stacks.undo.length > 0,
    canRedo: () => stacks.redo.length > 0,
    snapshot: () => ({ undo: [...stacks.undo], redo: [...stacks.redo] }),
  };
}
