/**
 * React provider for the page-scoped Execution Debugger store.
 *
 * Creates one store per `executionId` it sees and disposes the
 * previous one when the id changes (or when the page unmounts).
 * Children read the store through `useExecutionStore` /
 * `useDebuggerStoreHandle` from `./useDebuggerStore`.
 *
 * Page-scoped is intentional — multiple debugger tabs (and multiple
 * `/executions/:id` routes mid-navigation) must not share state.
 */
import { useMemo, type ReactNode } from 'react';

import { DebuggerStoreContext } from './DebuggerStoreContext';
import { createExecutionStore } from './executionStore';

export interface DebuggerStoreProviderProps {
  executionId: string;
  children: ReactNode;
}

export function DebuggerStoreProvider({
  executionId,
  children,
}: DebuggerStoreProviderProps) {
  // `useMemo` keyed on `executionId` reuses the same store across
  // re-renders but rebuilds it when the route id changes (which would
  // otherwise leak state from one execution into the next).
  const store = useMemo(
    () => createExecutionStore(executionId),
    [executionId],
  );

  return (
    <DebuggerStoreContext.Provider value={store}>
      {children}
    </DebuggerStoreContext.Provider>
  );
}
