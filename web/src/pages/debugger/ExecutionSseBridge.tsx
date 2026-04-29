/**
 * Mounts the per-execution mock SSE driver for the Debugger page.
 *
 * Lifts the scenario fetch + driver install out of `DebuggerPage` so
 * the page stays focused on layout. Handler is a no-op in Task 2 —
 * Task 7 (imperative controls) wires the events into the page-scoped
 * store via `ingestSse`.
 *
 * Renders nothing.
 */
import type { Execution } from '../../api/resources/executions';
import { useScenario } from '../../api/resources/scenarios';

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

  useExecutionSseDriver({
    executionId,
    execution,
    scenario: scenarioQuery.data,
    // Task 2: handler is a no-op. Task 7 wires this through to
    // `store.ingestSse`.
    onEvent: undefined,
  });

  return null;
}
