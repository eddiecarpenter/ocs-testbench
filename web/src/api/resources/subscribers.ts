import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type Subscriber = components['schemas']['Subscriber'];
export type SubscriberPage = components['schemas']['SubscriberPage'];

export interface ListSubscribersParams {
  limit?: number;
  offset?: number;
}

export const subscriberKeys = {
  all: ['subscribers'] as const,
  list: (params: ListSubscribersParams = {}) =>
    [...subscriberKeys.all, 'list', params] as const,
};

export const listSubscribers = (
  params: ListSubscribersParams = {},
  signal?: AbortSignal,
) =>
  ApiService.get<SubscriberPage>('/subscribers', {
    params,
    signal,
  });

export function useSubscribers(params: ListSubscribersParams = {}) {
  return useQuery({
    queryKey: subscriberKeys.list(params),
    queryFn: ({ signal }) => listSubscribers(params, signal),
  });
}
