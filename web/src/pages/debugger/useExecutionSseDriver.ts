/**
 * Per-execution SSE driver — connects to the real server endpoint
 * GET /api/v1/events/executions/{id} and maps server-sent events into
 * the debugger store via `ingestSse`.
 *
 * The server emits `execution.progress` events with:
 *   { type, sessionId, state, step, metrics }
 *
 * We map these onto the store's SseEventPayload discriminated union and
 * call ingestSse so the store's state machine transitions correctly.
 *
 * The controller returned by this hook exposes `pause()` / `resume()`
 * as no-ops — interactive mode is driven by the user clicking
 * "Send CCR" (which calls POST /step synchronously), not by the SSE
 * stream.  The interface is preserved so ExecutionSseBridge compiles
 * unchanged.
 */
import { useEffect, useMemo, useRef } from 'react';

import { appConfig } from '../../config/app';
import type { Execution } from '../../api/resources/executions';
import type { Scenario } from '../scenarios/types';
import type { SseEventPayload } from './executionStore';

export type ExecutionSseHandler = (event: SseEventPayload) => void;

export interface ExecutionSseController {
  pause(): void;
  resume(): void;
}

interface UseExecutionSseDriverOptions {
  executionId: string;
  execution: Execution | undefined;
  scenario: Scenario | undefined;
  onEvent?: ExecutionSseHandler;
  enabled?: boolean;
}

const TERMINAL_STATES = new Set(['success', 'failure', 'aborted', 'error']);

export function useExecutionSseDriver({
  executionId,
  execution,
  onEvent,
  enabled = true,
}: UseExecutionSseDriverOptions): ExecutionSseController {
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    if (!enabled || !execution) return;
    if (TERMINAL_STATES.has(execution.state)) return;

    const url = `${appConfig.apiBaseUrl}/events/executions/${encodeURIComponent(executionId)}`;
    const es = new EventSource(url);

    es.addEventListener('execution.progress', (ev: MessageEvent) => {
      try {
        const raw = JSON.parse(ev.data as string) as {
          type: string;
          sessionId: string;
          state: string;
          step: number;
          metrics: unknown;
        };
        const handler = onEventRef.current;
        if (!handler) return;

        const state = raw.state;
        if (state === 'paused') {
          handler({
            type: 'execution.paused',
            data: { executionId: raw.sessionId, atStepIndex: raw.step + 1 },
          });
        } else if (state === 'completed' || state === 'success') {
          // Use the full snapshot shape the store expects — we don't have
          // step records here so pass what we have; the snapshot bridge
          // will fill in the rest on the next REST poll.
          handler({
            type: 'execution.completed',
            data: execution as Execution,
          });
        } else if (state === 'error' || state === 'failure') {
          handler({
            type: 'execution.failed',
            data: { ...(execution as Execution), failureReason: 'Step failed' },
          });
        } else if (state === 'aborted') {
          handler({
            type: 'execution.aborted',
            data: execution as Execution,
          });
        }
      } catch {
        // malformed event — drop
      }
    });

    es.addEventListener('error', () => {
      // EventSource auto-reconnects on transient errors; nothing to do here.
      // If the connection is terminal (readyState === 2) the execution is done.
    });

    return () => {
      es.close();
    };
  }, [executionId, execution, enabled]);

  return useMemo<ExecutionSseController>(
    () => ({
      pause() {},
      resume() {},
    }),
    [],
  );
}
