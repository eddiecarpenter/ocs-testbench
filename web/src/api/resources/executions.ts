import { useQuery } from '@tanstack/react-query';

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

export interface ListExecutionsParams {
  state?: ExecutionState;
  scenarioId?: string;
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
