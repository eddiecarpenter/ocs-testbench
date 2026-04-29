/**
 * Mounts the per-execution mock SSE driver for the Debugger page.
 *
 * Lifts the scenario fetch + driver install out of `DebuggerPage` so
 * the page stays focused on layout. Driver events are routed into the
 * page-scoped store via `ingestSse`; the store's reducer maps each
 * event onto a state-machine transition + step append.
 *
 * Also coordinates the driver's pause / resume with the store's
 * lifecycle: when the user pauses (state goes `paused`), the driver
 * is paused; when the user resumes (state goes `running`), the
 * driver picks up where it left off.
 *
 * Renders nothing.
 */
import { useCallback, useEffect } from 'react';

import type { Execution } from '../../api/resources/executions';
import { useScenario } from '../../api/resources/scenarios';
import type { DriverEvent } from '../../mocks/sse/installExecutionSse';

import type { SseEventPayload } from './executionStore';
import { useDebuggerStoreHandle } from './useDebuggerStore';
import { useExecutionStore } from './useDebuggerStore';
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
  const runState = useExecutionStore((s) => s.state);

  const onEvent = useCallback(
    (event: DriverEvent) => {
      // `execution.batchStep` is a continuous-mode-only event the
      // store doesn't yet consume — drop it. The store's
      // `SseEventPayload` union pins the events it understands; the
      // narrow below filters everything else out at type-check time.
      if (event.type === 'execution.batchStep') return;
      store.getState().ingestSse(event as SseEventPayload);
    },
    [store],
  );

  const driver = useExecutionSseDriver({
    executionId,
    execution,
    scenario: scenarioQuery.data,
    onEvent,
  });

  // Coordinate the driver's pause/resume with the store's state. When
  // the user clicks Pause, the imperative action flips the store
  // synchronously; this effect catches that change and stops the
  // driver's auto-paced events. Same path in reverse for Resume.
  useEffect(() => {
    if (runState === 'paused') driver.pause();
    else if (runState === 'running') driver.resume();
  }, [driver, runState]);

  return null;
}
