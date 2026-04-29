/**
 * Wire `installExecutionSse` into the Debugger page.
 *
 * Lifecycle:
 *   - Subscribe on mount, dispose on unmount.
 *   - Reinstall when the execution id or scenario changes.
 *
 * Returns a stable controller object whose `pause()` / `resume()` call
 * through to the live driver's handle (or are no-ops when no driver
 * is installed). The controller is intentionally not React state — it
 * doesn't need to trigger re-renders. Components that want to react
 * to state should subscribe to the store directly.
 */
import { useEffect, useMemo, useRef } from 'react';

import type {
  Execution,
  ExecutionMode,
} from '../../api/resources/executions';
import {
  installExecutionSse,
  type DriverEvent,
  type InstallExecutionSseHandle,
} from '../../mocks/sse/installExecutionSse';
import type { Scenario } from '../scenarios/types';

export type ExecutionSseHandler = (event: DriverEvent) => void;

interface UseExecutionSseDriverOptions {
  executionId: string;
  execution: Execution | undefined;
  scenario: Scenario | undefined;
  /** Subscriber. Defaults to a no-op. */
  onEvent?: ExecutionSseHandler;
  /** When false, do not install the driver (e.g. terminal executions). */
  enabled?: boolean;
}

export interface ExecutionSseController {
  pause(): void;
  resume(): void;
}

export function useExecutionSseDriver({
  executionId,
  execution,
  scenario,
  onEvent,
  enabled = true,
}: UseExecutionSseDriverOptions): ExecutionSseController {
  const handleRef = useRef<InstallExecutionSseHandle | null>(null);

  useEffect(() => {
    if (!enabled) {
      handleRef.current = null;
      return undefined;
    }
    if (!execution || !scenario) {
      handleRef.current = null;
      return undefined;
    }
    const isTerminal =
      execution.state === 'success' ||
      execution.state === 'failure' ||
      execution.state === 'aborted' ||
      execution.state === 'error';
    if (isTerminal) {
      handleRef.current = null;
      return undefined;
    }

    const mode: ExecutionMode = execution.mode;
    const driver = installExecutionSse(executionId, scenario, mode, {
      startedAtMs: Date.parse(execution.startedAt) || Date.now(),
    });
    handleRef.current = driver;
    const unsub = driver.subscribe((event) => {
      if (onEvent) onEvent(event);
    });
    return () => {
      unsub();
      driver.dispose();
      handleRef.current = null;
    };
  }, [executionId, execution, scenario, onEvent, enabled]);

  // Stable controller — `pause` / `resume` always reflect the current
  // driver via the ref.
  return useMemo<ExecutionSseController>(
    () => ({
      pause() {
        handleRef.current?.pause();
      },
      resume() {
        handleRef.current?.resume();
      },
    }),
    [],
  );
}
