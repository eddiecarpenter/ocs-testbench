import type { InternalAxiosRequestConfig } from 'axios';

import { appConfig } from '../../config/app';

const REQUEST_ID_HEADER = 'X-Request-Id';
const AUTH_HEADER = 'Authorization';
const TOKEN_TYPE_PREFIX = 'Bearer ';

/** Augment AxiosRequestConfig with our own per-request hints.
 *
 *  - `skipAuth`      — don't attach the bearer token (public endpoints)
 *  - `skipRequestId` — don't generate an X-Request-Id header for this call
 */
declare module 'axios' {
  interface AxiosRequestConfig {
    skipAuth?: boolean;
    skipRequestId?: boolean;
  }
  interface InternalAxiosRequestConfig {
    skipAuth?: boolean;
    skipRequestId?: boolean;
  }
}

function generateRequestId(): string {
  try {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
  } catch {
    // fall through
  }
  return '';
}

/**
 * Read the access token from the configured storage strategy. Returns `''`
 * when no token exists or the browser blocks storage access.
 *
 * MVP: no auth is issued, so this almost always returns ''. The token-storage
 * plumbing is reserved so that post-Keycloak we only need to populate the
 * storage on sign-in; no changes here.
 */
function readAccessToken(): string {
  const { accessTokenPersistStrategy, accessTokenStorageKey } = appConfig;
  try {
    switch (accessTokenPersistStrategy) {
      case 'localStorage':
        return localStorage.getItem(accessTokenStorageKey) ?? '';
      case 'sessionStorage':
        return sessionStorage.getItem(accessTokenStorageKey) ?? '';
      case 'memory':
        // Future: read from an in-memory auth store (Zustand).
        return '';
    }
  } catch {
    return '';
  }
}

export function axiosRequestInterceptor(
  config: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig {
  // Authorization header — respects per-request skip flag
  if (!config.skipAuth) {
    const token = readAccessToken();
    if (token) {
      config.headers.set(AUTH_HEADER, `${TOKEN_TYPE_PREFIX}${token}`);
    }
  }

  // Correlation id — respects per-request skip flag
  if (!config.skipRequestId && !config.headers.has(REQUEST_ID_HEADER)) {
    const id = generateRequestId();
    if (id) config.headers.set(REQUEST_ID_HEADER, id);
  }

  return config;
}
