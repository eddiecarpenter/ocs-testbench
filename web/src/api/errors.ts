import type { AxiosError } from 'axios';

/**
 * Normalised error shape thrown by `ApiService`. Callers can pattern-match
 * on `status` / `code` without importing Axios types.
 *
 * The OCS Testbench backend returns RFC 7807 `application/problem+json` on
 * all non-2xx responses:
 *
 *   { "type": "about:blank", "title": "Peer not found", "status": 404, ... }
 *
 * We prefer those fields, but fall back to plain `{message}` / `{error}`
 * payloads so the client is robust against off-contract responses (dev
 * tooling, proxies, third-party errors).
 */
export class ApiError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly detail?: string;
  readonly data?: unknown;

  constructor(
    message: string,
    opts: {
      status?: number;
      code?: string;
      detail?: string;
      data?: unknown;
    } = {},
  ) {
    super(message);
    this.name = 'ApiError';
    // ensure instanceof works across bundle / transform boundaries
    Object.setPrototypeOf(this, new.target.prototype);
    this.status = opts.status;
    this.code = opts.code;
    this.detail = opts.detail;
    this.data = opts.data;
  }
}

type ProblemShape = {
  type?: string;
  title?: string;
  status?: number;
  detail?: string;
  instance?: string;
};

/**
 * Convert any thrown value into an `ApiError`. Never resolves — always
 * throws. Typed as `never` so TypeScript narrows correctly after a call.
 */
export function normalizeError(err: unknown): never {
  const axErr = err as AxiosError<unknown> | undefined;

  if (axErr?.isAxiosError) {
    const status = axErr.response?.status;
    const payload = (axErr.response?.data ?? {}) as Record<string, unknown> &
      ProblemShape;

    // Prefer RFC 7807 (title + detail). Fall back to common legacy shapes.
    const title =
      payload.title ??
      (payload['message'] as string | undefined) ??
      (payload['error'] as string | undefined) ??
      axErr.message ??
      'Request failed';

    const detail =
      payload.detail ??
      (payload['errorDescription'] as string | undefined);

    const code =
      (payload['code'] as string | undefined) ??
      (payload['errorCode'] as string | undefined) ??
      axErr.code;

    throw new ApiError(title, {
      status,
      code,
      detail,
      data: axErr.response?.data,
    });
  }

  // Non-Axios unexpected error
  const message = err instanceof Error ? err.message : 'Unexpected error';
  throw new ApiError(message);
}
