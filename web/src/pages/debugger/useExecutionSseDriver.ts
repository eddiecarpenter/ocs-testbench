/**
 * Wire `installExecutionSse` into the Debugger page.
 *
 * Lifecycle:
 *   - Subscribe on mount, dispose on unmount.
 *   - Reinstall when the execution id or scenario changes.
 *
 * In Task 2 the subscriber is intentionally a no-op — Task 7 wires
 * the driver's events into the page-scoped store so the imperative
 * controls drive real state transitions. Until then, the driver
 * exists so the rest of the app can prove the contract is wired.
 */
import { useEffect } from 'react';

import type {
  Execution,
  ExecutionMode,
} from '../../api/resources/executions';
import {
  installExecutionSse,
  type DriverEvent,
} from '../../mocks/sse/installExecutionSse';
import type { Scenario } from '../scenarios/types';

export type ExecutionSseHandler = (event: DriverEvent) => void;

interface UseExecutionSseDriverOptions {
  executionId: string;
  execution: Execution | undefined;
  scenario: Scenario | undefined;
  /** Subscriber. Defaults to a no-op (Task 2 behaviour). */
  onEvent?: ExecutionSseHandler;
  /** When false, do not install the driver (e.g. terminal executions). */
  enabled?: boolean;
}

export function useExecutionSseDriver({
  executionId,
  execution,
  scenario,
  onEvent,
  enabled = true,
}: UseExecutionSseDriverOptions): void {
  useEffect(() => {
    if (!enabled) return undefined;
    if (!execution || !scenario) return undefined;
    // Don't install for terminal runs — there's no live event stream
    // for an already-finished execution.
    const isTerminal =
      execution.state === 'success' ||
      execution.state === 'failure' ||
      execution.state === 'aborted' ||
      execution.state === 'error';
    if (isTerminal) return undefined;

    const mode: ExecutionMode = execution.mode;
    const handle = installExecutionSse(executionId, scenario, mode, {
      startedAtMs: Date.parse(execution.startedAt) || Date.now(),
    });
    const unsub = handle.subscribe((event) => {
      if (onEvent) onEvent(event);
    });
    return () => {
      unsub();
      handle.dispose();
    };
  }, [executionId, execution, scenario, onEvent, enabled]);
}
