/**
 * Frontend runtime config.
 *
 * Values are sourced from Vite env vars (`VITE_*`) with sensible defaults for
 * local development. All policy decisions that might change between
 * environments belong here — not scattered through component files.
 */

export type TokenPersistStrategy = 'localStorage' | 'sessionStorage' | 'memory';

export interface AppConfig {
  /** Base URL for REST calls. Relative by default so the frontend works when
   *  embedded in the Go binary (go:embed) behind the same origin. */
  apiBaseUrl: string;
  /** Request timeout in milliseconds. */
  apiTimeoutMs: number;
  /** Where to persist the access token once auth lands (MVP: unused). */
  accessTokenPersistStrategy: TokenPersistStrategy;
  /** Storage key for the access token. */
  accessTokenStorageKey: string;
  /** When true, the axios-mock-adapter is installed and serves `fakeApi` handlers. */
  useMockApi: boolean;
  /** Enables verbose logging of API traffic (request-id, durations, errors). */
  debugApi: boolean;
}

const toBool = (v: string | undefined, fallback = false) =>
  v === undefined ? fallback : v === '1' || v.toLowerCase() === 'true';

export const appConfig: AppConfig = {
  apiBaseUrl: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
  apiTimeoutMs: Number(import.meta.env.VITE_API_TIMEOUT_MS ?? 30_000),
  accessTokenPersistStrategy:
    (import.meta.env.VITE_TOKEN_STORAGE as TokenPersistStrategy | undefined) ??
    'sessionStorage',
  accessTokenStorageKey: 'ocs-testbench.access-token',
  useMockApi: toBool(import.meta.env.VITE_USE_MOCK_API, import.meta.env.DEV),
  debugApi: toBool(import.meta.env.VITE_DEBUG_API, false),
};
