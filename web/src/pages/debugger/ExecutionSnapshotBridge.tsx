/**
 * Bridge from the `useExecution(id)` REST snapshot into the page-scoped
 * Zustand store.
 *
 * Sits inside `DebuggerStoreProvider` so it can call `ingestSnapshot`
 * when the snapshot first arrives and whenever it refetches. Renders
 * nothing — its job is purely the side-effect of seeding the store.
 *
 * The component is a separate file (rather than a hook in DebuggerPage)
 * because the store can only be reached *inside* the provider, and the
 * REST query runs *outside* it (the page-level data layer).
 */
import { useEffect } from 'react';

import type { Execution } from '../../api/resources/executions';

import { useDebuggerStoreHandle } from './useDebuggerStore';

interface ExecutionSnapshotBridgeProps {
  execution: Execution;
}

export function ExecutionSnapshotBridge({
  execution,
}: ExecutionSnapshotBridgeProps) {
  const store = useDebuggerStoreHandle();

  useEffect(() => {
    store.getState().ingestSnapshot(execution);
  }, [execution, store]);

  return null;
}
