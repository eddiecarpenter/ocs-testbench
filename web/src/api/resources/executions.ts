/**
 * TanStack Query hooks + REST helpers for the Executions resource.
 *
 * Types are sourced exclusively from `src/api/schema.d.ts` (regenerated
 * from OpenAPI v0.2 via `npm run gen:api`). No hand-rolled execution
 * shapes — every consumer should reach for these re-exports.
 */
import {
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type ExecutionSummary = components['schemas']['ExecutionSummary'];
export type ExecutionPage = components['schemas']['ExecutionPage'];
export type ExecutionMode = components['schemas']['ExecutionMode'];
export type ExecutionState = components['schemas']['ExecutionState'];
export type Execution = components['schemas']['Execution'];
export type StepRecord = components['schemas']['StepRecord'];
export type PauseReason = components['schemas']['PauseReason'];
export type ExecutionContextSnapshot =
  components['schemas']['ExecutionContextSnapshot'];
export type StartExecutionInput =
  components['schemas']['StartExecutionInput'];
export type StartExecutionResult =
  components['schemas']['StartExecutionResult'];

export interface ListExecutionsParams {
  state?: ExecutionState;
  scenarioId?: string;
  /**
   * Mock-layer convenience filter — the OpenAPI v0.2 spec does not
   * define a `peerId` query param on `GET /executions`, so this is
   * honoured only by `executionsFakeApi.ts` until the backend lands
   * an equivalent filter (parking-lot on the parent Requirement).
   */
  peerId?: string;
  batchId?: string;
  limit?: number;
  offset?: number;
}

export const executionKeys = {
  all: ['executions'] as const,
  list: (params: ListExecutionsParams = {}) =>
    [...executionKeys.all, 'list', params] as const,
  detail: (id: string) => [...executionKeys.all, 'detail', id] as const,
};

// ---------------------------------------------------------------------------
// REST primitives
// ---------------------------------------------------------------------------

export const listExecutions = (
  params: ListExecutionsParams = {},
  signal?: AbortSignal,
) =>
  ApiService.get<ExecutionPage>('/executions', {
    params,
    signal,
  });

export const getExecution = (id: string, signal?: AbortSignal) =>
  ApiService.get<Execution>(`/executions/${encodeURIComponent(id)}`, {
    signal,
  });

export const startExecution = (input: StartExecutionInput) =>
  ApiService.post<StartExecutionResult, StartExecutionInput>(
    '/executions',
    input,
  );

/**
 * Re-run an existing execution. Looks up the source on the server and
 * starts a new run with the same parameters.
 *
 * The `POST /executions/{id}/rerun` endpoint is a frontend-mock
 * convenience that the real backend has not yet landed; it returns the
 * same `StartExecutionResult` shape as `POST /executions` so call sites
 * are interchangeable. Backend-side implementation tracked on the
 * parent Requirement.
 */
export const rerunExecution = (id: string) =>
  ApiService.post<StartExecutionResult>(
    `/executions/${encodeURIComponent(id)}/rerun`,
  );

/**
 * Abort a running execution. Mirrors the OpenAPI `abortExecution`
 * operation — server transitions the run to `aborted` (with a
 * best-effort CCR-TERMINATE if the scenario is mid-session).
 */
export const abortExecution = (id: string) =>
  ApiService.post<void>(`/executions/${encodeURIComponent(id)}/abort`);

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

export function useExecutions(params: ListExecutionsParams = {}) {
  return useQuery({
    queryKey: executionKeys.list(params),
    queryFn: ({ signal }) => listExecutions(params, signal),
  });
}

export function useExecution(id: string) {
  return useQuery({
    queryKey: executionKeys.detail(id),
    queryFn: ({ signal }) => getExecution(id, signal),
    enabled: Boolean(id),
  });
}

/**
 * Create a new execution. On success, invalidates every list query so
 * the new row(s) appear, and seeds the detail cache for the first row
 * for instant navigation in the Interactive case.
 */
export function useCreateExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: StartExecutionInput) => startExecution(input),
    onSuccess: (result) => {
      void qc.invalidateQueries({ queryKey: executionKeys.all });
      const first = result.items[0];
      if (first) {
        // Seed the summary into the cache. The detail page (#95) will
        // hydrate the full Execution shape on demand.
        qc.setQueryData(executionKeys.detail(first.id), first);
      }
    },
  });
}

/**
 * Re-run an existing execution. Same cache effects as
 * `useCreateExecution`.
 */
export function useRerunExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => rerunExecution(id),
    onSuccess: (result) => {
      void qc.invalidateQueries({ queryKey: executionKeys.all });
      const first = result.items[0];
      if (first) {
        qc.setQueryData(executionKeys.detail(first.id), first);
      }
    },
  });
}

/**
 * Abort a running execution. Invalidates list + detail caches so the
 * row's status flips to `aborted` after the next refetch.
 */
export function useAbortExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => abortExecution(id),
    onSuccess: (_v, id) => {
      void qc.invalidateQueries({ queryKey: executionKeys.all });
      void qc.invalidateQueries({ queryKey: executionKeys.detail(id) });
    },
  });
}
