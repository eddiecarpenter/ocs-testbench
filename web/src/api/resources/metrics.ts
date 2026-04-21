import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type ResponseTimeSeries = components['schemas']['ResponseTimeSeries'];
export type ResponseTimePoint = components['schemas']['ResponseTimePoint'];

export interface ResponseTimeParams {
  /** ISO 8601 duration, e.g. `PT1H`, `PT15M`, `PT24H`. Defaults to `PT1H`. */
  window?: string;
}

export const metricsKeys = {
  all: ['metrics'] as const,
  responseTime: (params: ResponseTimeParams = {}) =>
    [...metricsKeys.all, 'response-time', params] as const,
};

export const getResponseTimeSeries = (
  params: ResponseTimeParams = {},
  signal?: AbortSignal,
) =>
  ApiService.get<ResponseTimeSeries>('/metrics/response-time', {
    params,
    signal,
  });

export function useResponseTimeSeries(params: ResponseTimeParams = {}) {
  return useQuery({
    queryKey: metricsKeys.responseTime(params),
    queryFn: ({ signal }) => getResponseTimeSeries(params, signal),
  });
}
