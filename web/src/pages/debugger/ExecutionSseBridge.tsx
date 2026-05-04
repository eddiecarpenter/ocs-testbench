/**
 * Mounts the per-execution SSE driver for the Debugger page.
 *
 * Routes server-sent events into the page-scoped store via `ingestSse`.
 * Renders nothing.
 */
import { useCallback } from 'react';

import type { Execution } from '../../api/resources/executions';
import { useScenario } from '../../api/resources/scenarios';

import type { SseEventPayload } from './executionStore';
import { useDebuggerStoreHandle } from './useDebuggerStore';
import { useExecutionSseDriver } from './useExecutionSseDriver';

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

  const onEvent = useCallback(
    (event: SseEventPayload) => {
      store.getState().ingestSse(event);
    },
    [store],
  );

  useExecutionSseDriver({
    executionId,
    execution,
    scenario: scenarioQuery.data,
    onEvent,
  });

  return null;
}
