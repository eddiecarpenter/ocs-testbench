import { useQueryClient } from '@tanstack/react-query';
import { notifications } from '@mantine/notifications';
import { useEffect, useRef } from 'react';

import { STATUS_LABEL } from '../../components/peer/peerStatus';
import { peerKeys, type Peer, type PeerStatus } from './peers';

/**
 * Watch the peers list cache and surface a toast for every status change
 * — regardless of what caused it (user mutation, SSE event, refetch).
 *
 * Mount once at the application root so toasts fire on every page.
 *
 * Transient states (`connecting`, `disconnecting`, `restarting`) are
 * suppressed: they're already visible in the row badge and doubling up
 * would produce three toasts for a single restart. We only toast on
 * settled outcomes (`connected`, `disconnected`, `stopped`, `error`).
 */
export function usePeerStatusToasts() {
  const qc = useQueryClient();
  // Track the last observed status per peer id so we can diff on each
  // cache update. Ref, not state — we don't want to re-render.
  const lastStatus = useRef<Map<string, PeerStatus>>(new Map());

  useEffect(() => {
    // Seed from whatever the cache already has so we don't spam toasts on
    // first mount for peers whose status we've seen since page load.
    const seed = qc.getQueryData<Peer[]>(peerKeys.list());
    if (seed) {
      for (const p of seed) lastStatus.current.set(p.id, p.status);
    }

    const cache = qc.getQueryCache();
    const unsubscribe = cache.subscribe((event) => {
      // Only react to list-cache updates. Detail-cache writes are already
      // mirrored into the list by our mutations and by applyEventToCache,
      // so listening to both would double-fire.
      if (event.type !== 'updated') return;
      const key = event.query.queryKey;
      if (!Array.isArray(key) || key[0] !== 'peers' || key[1] !== 'list') return;

      const peers = event.query.state.data as Peer[] | undefined;
      if (!peers) return;

      for (const peer of peers) {
        const prev = lastStatus.current.get(peer.id);
        if (prev === peer.status) continue;
        lastStatus.current.set(peer.id, peer.status);

        // Skip the first observation of a peer — no meaningful transition.
        if (prev === undefined) continue;

        // Skip transients; we only toast settled outcomes. The row badge
        // communicates the in-flight state already.
        if (
          peer.status === 'connecting' ||
          peer.status === 'disconnecting' ||
          peer.status === 'restarting'
        ) {
          continue;
        }

        // Peer-status toasts are *informational*, even when the new
        // status is `error` — the user does not need to act, just take
        // note. So they all auto-close (Mantine default), unlike the
        // sticky `notifyError` toasts used for failed user actions.
        // The colour still distinguishes the state at a glance.
        const color =
          peer.status === 'connected'
            ? 'teal'
            : peer.status === 'error'
              ? 'red'
              : 'gray';

        notifications.show({
          color,
          title: `${peer.name}: ${STATUS_LABEL[peer.status]}`,
          message:
            peer.statusDetail ??
            `Status changed from ${STATUS_LABEL[prev]} to ${STATUS_LABEL[peer.status]}.`,
        });
      }

      // Prune ids that are no longer in the list (peer deleted).
      const live = new Set(peers.map((p) => p.id));
      for (const id of lastStatus.current.keys()) {
        if (!live.has(id)) lastStatus.current.delete(id);
      }
    });

    return () => unsubscribe();
  }, [qc]);
}
