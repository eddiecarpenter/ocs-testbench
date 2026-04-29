/**
 * TanStack Query hooks + REST helpers for the Scenarios resource.
 *
 * The hooks own the server-state cache; the Builder's editing state
 * lives in `scenarioDraftStore.ts`. Shape of the keys mirrors the
 * `peerKeys` / `subscriberKeys` pattern (Reuse: refactor — same list
 * + detail key shape; the existing `api/resources/scenarios.ts` only
 * provided `useScenarios`, which this module supersedes).
 */
import {
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import ApiService from '../ApiService';
import type {
  Scenario,
  ScenarioDuplicateInput,
  ScenarioInput,
  ScenarioSummary,
} from '../../pages/scenarios/types';
import type {
  StartExecutionInput,
  StartExecutionResult,
} from './executions';

export const scenarioKeys = {
  all: ['scenarios'] as const,
  list: () => [...scenarioKeys.all, 'list'] as const,
  detail: (id: string) => [...scenarioKeys.all, 'detail', id] as const,
};

// ---------------------------------------------------------------------------
// REST primitives
// ---------------------------------------------------------------------------

export const listScenarios = (signal?: AbortSignal) =>
  ApiService.get<ScenarioSummary[]>('/scenarios', { signal });

export const getScenario = (id: string, signal?: AbortSignal) =>
  ApiService.get<Scenario>(`/scenarios/${encodeURIComponent(id)}`, {
    signal,
  });

export const createScenario = (input: ScenarioInput) =>
  ApiService.post<Scenario, ScenarioInput>('/scenarios', input);

export const updateScenario = (id: string, input: ScenarioInput) =>
  ApiService.put<Scenario, ScenarioInput>(
    `/scenarios/${encodeURIComponent(id)}`,
    input,
  );

export const deleteScenario = (id: string) =>
  ApiService.delete<void>(`/scenarios/${encodeURIComponent(id)}`);

export const duplicateScenario = (
  id: string,
  body: ScenarioDuplicateInput = {},
) =>
  ApiService.post<Scenario, ScenarioDuplicateInput>(
    `/scenarios/${encodeURIComponent(id)}/duplicate`,
    body,
  );

/**
 * Start a one-off `continuous`-mode execution for a scenario from the
 * Scenarios list "Run" affordance. The body matches OpenAPI v0.2's
 * `StartExecutionInput` exactly — concurrency and repeats default to 1
 * because this entry point doesn't expose batched-run controls (those
 * live on the Executions Start-Run dialog, Feature #94).
 */
export const runScenario = (
  scenarioId: string,
): Promise<StartExecutionResult> =>
  ApiService.post<StartExecutionResult, StartExecutionInput>(
    '/executions',
    {
      scenarioId,
      mode: 'continuous',
      concurrency: 1,
      repeats: 1,
    },
  );

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

export function useScenarios() {
  return useQuery({
    queryKey: scenarioKeys.list(),
    queryFn: ({ signal }) => listScenarios(signal),
  });
}

export function useScenario(id: string | undefined) {
  return useQuery({
    queryKey: id ? scenarioKeys.detail(id) : scenarioKeys.all,
    queryFn: ({ signal }) => getScenario(id!, signal),
    enabled: Boolean(id),
  });
}

export function useCreateScenario() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ScenarioInput) => createScenario(input),
    onSuccess: (scn) => {
      qc.setQueryData(scenarioKeys.detail(scn.id), scn);
      void qc.invalidateQueries({ queryKey: scenarioKeys.list() });
    },
  });
}

export function useUpdateScenario(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ScenarioInput) => updateScenario(id, input),
    onSuccess: (scn) => {
      qc.setQueryData(scenarioKeys.detail(scn.id), scn);
      void qc.invalidateQueries({ queryKey: scenarioKeys.list() });
    },
  });
}

export function useDeleteScenario() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteScenario(id),
    onSuccess: (_v, id) => {
      qc.removeQueries({ queryKey: scenarioKeys.detail(id) });
      void qc.invalidateQueries({ queryKey: scenarioKeys.list() });
    },
  });
}

export function useDuplicateScenario() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body?: ScenarioDuplicateInput }) =>
      duplicateScenario(id, body ?? {}),
    onSuccess: (scn) => {
      qc.setQueryData(scenarioKeys.detail(scn.id), scn);
      void qc.invalidateQueries({ queryKey: scenarioKeys.list() });
    },
  });
}

export function useRunScenario() {
  return useMutation({
    mutationFn: (scenarioId: string) => runScenario(scenarioId),
  });
}
