import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type Peer = components['schemas']['Peer'];
export type PeerStatus = components['schemas']['PeerStatus'];

export const peerKeys = {
  all: ['peers'] as const,
  list: () => [...peerKeys.all, 'list'] as const,
  detail: (id: string) => [...peerKeys.all, 'detail', id] as const,
};

export const listPeers = (signal?: AbortSignal) =>
  ApiService.get<Peer[]>('/peers', { signal });

export const getPeer = (id: string, signal?: AbortSignal) =>
  ApiService.get<Peer>(`/peers/${encodeURIComponent(id)}`, { signal });

export function usePeers() {
  return useQuery({
    queryKey: peerKeys.list(),
    queryFn: ({ signal }) => listPeers(signal),
  });
}

export function usePeer(id: string | undefined) {
  return useQuery({
    queryKey: id ? peerKeys.detail(id) : peerKeys.all,
    queryFn: ({ signal }) => getPeer(id!, signal),
    enabled: Boolean(id),
  });
}
