import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState, type ReactNode } from 'react';

import { appConfig } from '../../config/app';
import { applyEventToCache } from './applyEventToCache';
import { SseContext, type SseContextValue } from './SseContext';
import { SseClient, type SseStatus } from './SseClient';

interface SseProviderProps {
  children: ReactNode;
  /** Override the SSE endpoint. Defaults to `${apiBaseUrl}/events`. */
  url?: string;
}

/**
 * Owns a single SSE connection for the app and fans events out to the
 * TanStack Query cache.
 *
 * Integration model (per ARCHITECTURE.md §9):
 * - Every event payload is the **full current state** of its resource.
 * - We `setQueryData` for detail/KPI caches so the UI updates instantly.
 * - We patch list caches where we know the shape; otherwise invalidate.
 * - On transient reconnection, we invalidate everything so the next paint
 *   reads truth from REST and converges on any missed state.
 */
export function SseProvider({ children, url }: SseProviderProps) {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<SseStatus>('idle');

  const resolvedUrl = url ?? `${appConfig.apiBaseUrl}/events`;

  useEffect(() => {
    const client = new SseClient({
      url: resolvedUrl,
      onStatus: setStatus,
      onReconnectHint: () => {
        // Browser reconnects automatically; nudge the cache so the first
        // post-reconnect paint is guaranteed fresh.
        queryClient.invalidateQueries();
      },
      onEvent: (event) => applyEventToCache(queryClient, event),
    });
    client.open();
    return () => client.close();
    // resolvedUrl and queryClient are stable for the app's lifetime
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const value = useMemo<SseContextValue>(() => ({ status }), [status]);
  return <SseContext.Provider value={value}>{children}</SseContext.Provider>;
}
