import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type Peer = components['schemas']['Peer'];
export type PeerStatus = components['schemas']['PeerStatus'];
export type PeerTransport = components['schemas']['PeerTransport'];
export type PeerInput = components['schemas']['PeerInput'];
export type PeerTestResult = components['schemas']['PeerTestResult'];

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

export const createPeer = (input: PeerInput) =>
  ApiService.post<Peer, PeerInput>('/peers', input);

export const updatePeer = (id: string, input: PeerInput) =>
  ApiService.put<Peer, PeerInput>(`/peers/${encodeURIComponent(id)}`, input);

export const deletePeer = (id: string) =>
  ApiService.delete<void>(`/peers/${encodeURIComponent(id)}`);

export const testPeer = (id: string) =>
  ApiService.post<PeerTestResult>(`/peers/${encodeURIComponent(id)}/test`);

/**
 * Create a new peer. On success, invalidates the list so the new row
 * appears and seeds the detail cache for instant navigation.
 */
export function useCreatePeer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PeerInput) => createPeer(input),
    onSuccess: (peer) => {
      qc.setQueryData(peerKeys.detail(peer.id), peer);
      void qc.invalidateQueries({ queryKey: peerKeys.list() });
    },
  });
}

/**
 * Update an existing peer. Writes the server response directly into the
 * detail cache and patches the list entry in place to avoid a refetch
 * round-trip — SSE `peer.updated` will reconcile anything we missed.
 */
export function useUpdatePeer(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PeerInput) => updatePeer(id, input),
    onSuccess: (peer) => {
      qc.setQueryData(peerKeys.detail(peer.id), peer);
      qc.setQueryData<Peer[]>(peerKeys.list(), (prev) =>
        prev ? prev.map((p) => (p.id === peer.id ? peer : p)) : prev,
      );
    },
  });
}

/**
 * Delete a peer. Drops the detail cache and removes the row from the
 * list cache synchronously — no refetch needed.
 */
/**
 * Dry-run CER/CEA probe. Does not mutate persistent state — result is
 * returned to the caller for UI display only.
 */
export function useTestPeer(id: string) {
  return useMutation({
    mutationFn: () => testPeer(id),
  });
}

export function useDeletePeer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deletePeer(id).then(() => id),
    onSuccess: (id) => {
      qc.removeQueries({ queryKey: peerKeys.detail(id) });
      qc.setQueryData<Peer[]>(peerKeys.list(), (prev) =>
        prev ? prev.filter((p) => p.id !== id) : prev,
      );
    },
  });
}
