import type { AxiosError } from 'axios';

/**
 * HTTP status codes that indicate the current credentials are rejected or
 * expired. Treated as "signed out" by the UI.
 *
 *  - 401 Unauthorized   — missing or invalid credentials
 *  - 419 Page Expired   — Laravel-style session timeout (harmless if unused)
 *  - 440 Login Timeout  — IIS session timeout (harmless if unused)
 */
const unauthorizedCodes = new Set([401, 419, 440]);

/**
 * Response error interceptor — runs on every non-2xx response.
 *
 * MVP: a no-op, because there's no auth and no store to clear.
 *
 * Post-Keycloak: clear the token + user from the auth store, and a route
 * listener elsewhere will redirect to /login. The Axios promise is still
 * rejected so the calling screen can render its own error UI if needed.
 */
export function axiosResponseErrorInterceptor(error: AxiosError): void {
  const status = error.response?.status;
  if (status !== undefined && unauthorizedCodes.has(status)) {
    // TODO(auth): clear access token from storage + reset auth store.
    //   import { useAuthStore } from '../../store/authStore';
    //   useAuthStore.getState().signOut();
  }
}
