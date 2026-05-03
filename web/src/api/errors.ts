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
/**
 * Field-level validation errors. Keys are JSON Pointer refs into the
 * request body (e.g. `/name`), values are arrays of messages for that
 * field. Present on 422 responses.
 */
export type FieldErrors = Record<string, string[]>;

export class ApiError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly detail?: string;
  readonly data?: unknown;
  readonly errors?: FieldErrors;

  constructor(
    message: string,
    opts: {
      status?: number;
      code?: string;
      detail?: string;
      data?: unknown;
      errors?: FieldErrors;
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
    this.errors = opts.errors;
  }

  /**
   * Map `errors` from JSON-Pointer keys (`/name`) onto plain field names
   * (`name`) for plugging directly into a form library. Returns an empty
   * object when no field errors are present.
   */
  fieldErrors(): Record<string, string> {
    if (!this.errors) return {};
    const out: Record<string, string> = {};
    for (const [key, msgs] of Object.entries(this.errors)) {
      const field = key.startsWith('/') ? key.slice(1) : key;
      if (msgs && msgs.length > 0) out[field] = msgs[0];
    }
    return out;
  }
}

type ProblemShape = {
  type?: string;
  title?: string;
  status?: number;
  detail?: string;
  instance?: string;
  errors?: FieldErrors;
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

    // Prefer RFC 7807 (title + detail). Fall back to the backend's
    // {"error": {"code": "...", "message": "..."}} envelope, then
    // common legacy shapes.
    const errorBody = payload['error'];
    const errorMessage =
      typeof errorBody === 'object' && errorBody !== null
        ? ((errorBody as Record<string, unknown>)['message'] as string | undefined)
        : (errorBody as string | undefined);

    const title =
      payload.title ??
      (payload['message'] as string | undefined) ??
      errorMessage ??
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
      errors: payload.errors,
    });
  }

  // Non-Axios unexpected error
  const message = err instanceof Error ? err.message : 'Unexpected error';
  throw new ApiError(message);
}
