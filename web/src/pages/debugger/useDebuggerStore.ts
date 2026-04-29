/**
 * Hooks for reading the page-scoped Execution Debugger store.
 *
 * Lives in its own file so the provider component (`DebuggerStoreProvider`)
 * can stay export-only-components — Fast Refresh requires no non-component
 * exports in the same file as a component.
 */
import { useContext } from 'react';
import { useStore } from 'zustand';

import { DebuggerStoreContext } from './DebuggerStoreContext';
import type { ExecutionStore, ExecutionStoreState } from './executionStore';

/**
 * Read a slice of the nearest debugger store. Throws if no provider
 * is mounted — a developer bug worth surfacing loudly.
 */
export function useExecutionStore<T>(
  selector: (state: ExecutionStoreState) => T,
): T {
  const store = useContext(DebuggerStoreContext);
  if (!store) {
    throw new Error(
      'useExecutionStore must be used inside a <DebuggerStoreProvider>',
    );
  }
  return useStore(store, selector);
}

/**
 * Imperative handle on the store — useful for actions that need to
 * call `getState()` / `setState()` outside the React render path
 * (mostly the SSE wiring in Task 2 and the imperative actions in
 * Task 7).
 */
export function useDebuggerStoreHandle(): ExecutionStore {
  const store = useContext(DebuggerStoreContext);
  if (!store) {
    throw new Error(
      'useDebuggerStoreHandle must be used inside a <DebuggerStoreProvider>',
    );
  }
  return store;
}
