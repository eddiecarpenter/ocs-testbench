/**
 * Mounts the per-execution SSE driver for the Debugger page.
 *
 * Routes server-sent events into the page-scoped store via `ingestSse`.
 * Renders nothing.
 */
import { useCallback } from 'react';

import { useQueryClient } from '@tanstack/react-query';

import { executionKeys } from '../../api/resources/executions';
import type { Execution } from '../../api/resources/executions';
import { useScenario } from '../../api/resources/scenarios';

import type { SseEventPayload } from './executionStore';
import { useDebuggerStoreHandle } from './useDebuggerStore';
import { useExecutionSseDriver } from './useExecutionSseDriver';

const TERMINAL_EVENT_TYPES = new Set([
  'execution.completed',
  'execution.failed',
  'execution.aborted',
]);

// Events that require a fresh snapshot so the history pane shows the correct
// pending step. `execution.paused` fires when the engine is interrupted mid-run;
// the SSE payload doesn't carry step detail, so we refetch to get it.
const REFETCH_EVENT_TYPES = new Set([
  ...TERMINAL_EVENT_TYPES,
  'execution.paused',
]);

interface ExecutionSseBridgeProps {
  executionId: string;
  execution: Execution;
}

export function ExecutionSseBridge({
  executionId,
  execution,
}: ExecutionSseBridgeProps) {
  const scenarioQuery = useScenario(execution.scenarioId);
  const store = useDebuggerStoreHandle();
  const queryClient = useQueryClient();

  const onEvent = useCallback(
    (event: SseEventPayload) => {
      store.getState().ingestSse(event);
      // On terminal events, fetch the authoritative final snapshot so
      // ExecutionSnapshotBridge can populate the complete step list.
      // The SSE payload carries stale execution data from mount time;
      // the REST refetch gives us the real finished steps.
      if (REFETCH_EVENT_TYPES.has(event.type)) {
        void queryClient.invalidateQueries({
          queryKey: executionKeys.detail(executionId),
        });
      }
    },
    [store, queryClient, executionId],
  );

  useExecutionSseDriver({
    executionId,
    execution,
    scenario: scenarioQuery.data,
    onEvent,
  });

  return null;
}
