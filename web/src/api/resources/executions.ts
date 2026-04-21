import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type ExecutionSummary = components['schemas']['ExecutionSummary'];
export type ExecutionPage = components['schemas']['ExecutionPage'];
export type ExecutionMode = components['schemas']['ExecutionMode'];
export type ExecutionResult = components['schemas']['ExecutionResult'];

export interface ListExecutionsParams {
  status?: ExecutionResult;
  limit?: number;
  offset?: number;
}

export const executionKeys = {
  all: ['executions'] as const,
  list: (params: ListExecutionsParams = {}) =>
    [...executionKeys.all, 'list', params] as const,
};

export const listExecutions = (
  params: ListExecutionsParams = {},
  signal?: AbortSignal,
) =>
  ApiService.get<ExecutionPage>('/executions', {
    params,
    signal,
  });

export function useExecutions(params: ListExecutionsParams = {}) {
  return useQuery({
    queryKey: executionKeys.list(params),
    queryFn: ({ signal }) => listExecutions(params, signal),
  });
}
