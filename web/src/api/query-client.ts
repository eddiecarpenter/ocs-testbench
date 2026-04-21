import { QueryClient } from '@tanstack/react-query';

import { ApiError } from './errors';

/**
 * TanStack Query defaults tuned for an admin tool (vs. a consumer app):
 *
 *   - `staleTime: 30s`       — data is considered fresh for 30s; avoids
 *                              refetch on every mount while still feeling
 *                              live for status-y screens.
 *   - `refetchOnWindowFocus` — off. Engineers flip between tabs constantly;
 *                              the SSE layer (next PR) handles live updates.
 *   - `retry: 1`             — one retry for transient failures. Client
 *                              errors (4xx) are not retried because they
 *                              won't get better; mutations never retry.
 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        refetchOnWindowFocus: false,
        retry: (failureCount, error) => {
          if (error instanceof ApiError) {
            if (error.status && error.status >= 400 && error.status < 500) {
              return false;
            }
          }
          return failureCount < 1;
        },
      },
      mutations: {
        retry: false,
      },
    },
  });
}
