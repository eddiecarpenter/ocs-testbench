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

export const testPeerConfig = (input: PeerInput) =>
  ApiService.post<PeerTestResult, PeerInput>('/peers/test', input);

export const connectPeer = (id: string) =>
  ApiService.post<Peer>(`/peers/${encodeURIComponent(id)}/connect`);

export const disconnectPeer = (id: string) =>
  ApiService.post<Peer>(`/peers/${encodeURIComponent(id)}/disconnect`);

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
 * Dry-run CER/CEA probe against a candidate `PeerInput`. Stateless —
 * does not read from or write to any persisted peer. Used by both the
 * Create and Edit drawers to validate unsaved configuration.
 */
export function useTestPeerConfig() {
  return useMutation({
    mutationFn: (input: PeerInput) => testPeerConfig(input),
  });
}

/**
 * Shared cache patcher for connect/disconnect — writes the returned peer into
 * the detail cache and patches the list row in place so the UI reflects the
 * new status without a refetch. SSE reconciles anything we missed.
 */
function writePeerToCaches(qc: ReturnType<typeof useQueryClient>, peer: Peer) {
  qc.setQueryData(peerKeys.detail(peer.id), peer);
  qc.setQueryData<Peer[]>(peerKeys.list(), (prev) =>
    prev ? prev.map((p) => (p.id === peer.id ? peer : p)) : prev,
  );
}

/**
 * Optimistically patch a peer's status so the UI reflects the in-flight
 * action (`connecting` / `disconnecting`) immediately. Returns a snapshot
 * of the previous peer so the caller can roll back on error.
 */
function optimisticStatusPatch(
  qc: ReturnType<typeof useQueryClient>,
  id: string,
  nextStatus: PeerStatus,
  nextDetail?: string,
): Peer | undefined {
  const prev = qc.getQueryData<Peer>(peerKeys.detail(id))
    ?? qc.getQueryData<Peer[]>(peerKeys.list())?.find((p) => p.id === id);
  if (!prev) return undefined;
  const patched: Peer = {
    ...prev,
    status: nextStatus,
    statusDetail: nextDetail,
    lastChangeAt: new Date().toISOString(),
  };
  writePeerToCaches(qc, patched);
  return prev;
}

/**
 * Explicitly connect a peer. Independent of `autoConnect`, which governs
 * server-startup behaviour only. Optimistically flips status to
 * `connecting` so the UI shows immediate feedback; rolls back on error.
 */
export function useConnectPeer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => connectPeer(id),
    onMutate: (id) => ({
      prev: optimisticStatusPatch(qc, id, 'connecting'),
      id,
    }),
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) writePeerToCaches(qc, ctx.prev);
    },
    onSuccess: (peer) => writePeerToCaches(qc, peer),
  });
}

/**
 * Explicitly disconnect a peer. Supervision does not auto-reconnect.
 * Optimistically flips status to `disconnected` with a "Disconnecting…"
 * detail so the transition is visible while the request is in flight.
 */
export function useDisconnectPeer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => disconnectPeer(id),
    onMutate: (id) => ({
      prev: optimisticStatusPatch(qc, id, 'disconnecting'),
      id,
    }),
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) writePeerToCaches(qc, ctx.prev);
    },
    onSuccess: (peer) => writePeerToCaches(qc, peer),
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
