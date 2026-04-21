import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type Subscriber = components['schemas']['Subscriber'];
export type SubscriberInput = components['schemas']['SubscriberInput'];
export type TacEntry = components['schemas']['TacEntry'];

export const subscriberKeys = {
  all: ['subscribers'] as const,
  list: () => [...subscriberKeys.all, 'list'] as const,
  detail: (id: string) => [...subscriberKeys.all, 'detail', id] as const,
};

export const tacKeys = {
  all: ['tac-catalog'] as const,
  list: () => [...tacKeys.all, 'list'] as const,
};

export const listSubscribers = (signal?: AbortSignal) =>
  ApiService.get<Subscriber[]>('/subscribers', { signal });

export const getSubscriber = (id: string, signal?: AbortSignal) =>
  ApiService.get<Subscriber>(
    `/subscribers/${encodeURIComponent(id)}`,
    { signal },
  );

export const createSubscriber = (input: SubscriberInput) =>
  ApiService.post<Subscriber, SubscriberInput>('/subscribers', input);

export const updateSubscriber = (id: string, input: SubscriberInput) =>
  ApiService.put<Subscriber, SubscriberInput>(
    `/subscribers/${encodeURIComponent(id)}`,
    input,
  );

export const deleteSubscriber = (id: string) =>
  ApiService.delete<void>(`/subscribers/${encodeURIComponent(id)}`);

export const listTacCatalog = (signal?: AbortSignal) =>
  ApiService.get<TacEntry[]>('/tac-catalog', { signal });

export function useSubscribers() {
  return useQuery({
    queryKey: subscriberKeys.list(),
    queryFn: ({ signal }) => listSubscribers(signal),
  });
}

export function useSubscriber(id: string | undefined) {
  return useQuery({
    queryKey: id ? subscriberKeys.detail(id) : subscriberKeys.all,
    queryFn: ({ signal }) => getSubscriber(id!, signal),
    enabled: Boolean(id),
  });
}

/**
 * Curated TAC catalogue used to populate Manufacturer → Model selects
 * and to derive IMEI prefixes. The data is server-immutable, so it is
 * marked `staleTime: Infinity` — one fetch per session is plenty.
 */
export function useTacCatalog() {
  return useQuery({
    queryKey: tacKeys.list(),
    queryFn: ({ signal }) => listTacCatalog(signal),
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

export function useCreateSubscriber() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: SubscriberInput) => createSubscriber(input),
    onSuccess: (sub) => {
      qc.setQueryData(subscriberKeys.detail(sub.id), sub);
      void qc.invalidateQueries({ queryKey: subscriberKeys.list() });
    },
  });
}

export function useUpdateSubscriber(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: SubscriberInput) => updateSubscriber(id, input),
    onSuccess: (sub) => {
      qc.setQueryData(subscriberKeys.detail(sub.id), sub);
      qc.setQueryData<Subscriber[]>(subscriberKeys.list(), (prev) =>
        prev ? prev.map((p) => (p.id === sub.id ? sub : p)) : prev,
      );
    },
  });
}

export function useDeleteSubscriber() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteSubscriber(id).then(() => id),
    onSuccess: (id) => {
      qc.removeQueries({ queryKey: subscriberKeys.detail(id) });
      qc.setQueryData<Subscriber[]>(subscriberKeys.list(), (prev) =>
        prev ? prev.filter((s) => s.id !== id) : prev,
      );
    },
  });
}
