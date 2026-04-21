import type { AxiosRequestConfig, AxiosResponse } from 'axios';

import AxiosBase from './axios/AxiosBase';
import { normalizeError } from './errors';

/**
 * Generic HTTP wrapper. Returns only `response.data` and throws `ApiError`
 * on failure, so call sites never touch Axios types directly.
 *
 * Supports:
 *   - `config.signal` for cancellation (TanStack Query uses this)
 *   - `config.skipAuth` — see AxiosRequestInterceptor
 *   - `config.skipRequestId` — see AxiosRequestInterceptor
 *   - every other AxiosRequestConfig option (params, headers, timeout, …)
 */
export const ApiService = {
  async request<Response = unknown, Request = Record<string, unknown>>(
    config: AxiosRequestConfig<Request>,
  ): Promise<Response> {
    try {
      const res = await AxiosBase.request<
        Response,
        AxiosResponse<Response>,
        Request
      >(config);
      return res.data;
    } catch (err) {
      return normalizeError(err);
    }
  },

  async get<Response = unknown>(
    url: string,
    config?: Omit<AxiosRequestConfig<never>, 'url' | 'method'>,
  ): Promise<Response> {
    return this.request<Response, never>({ ...(config ?? {}), url, method: 'get' });
  },

  async post<Response = unknown, Request = Record<string, unknown>>(
    url: string,
    data?: Request,
    config?: Omit<AxiosRequestConfig<Request>, 'url' | 'method' | 'data'>,
  ): Promise<Response> {
    return this.request<Response, Request>({
      ...(config ?? {}),
      url,
      method: 'post',
      data,
    });
  },

  async put<Response = unknown, Request = Record<string, unknown>>(
    url: string,
    data?: Request,
    config?: Omit<AxiosRequestConfig<Request>, 'url' | 'method' | 'data'>,
  ): Promise<Response> {
    return this.request<Response, Request>({
      ...(config ?? {}),
      url,
      method: 'put',
      data,
    });
  },

  async patch<Response = unknown, Request = Record<string, unknown>>(
    url: string,
    data?: Request,
    config?: Omit<AxiosRequestConfig<Request>, 'url' | 'method' | 'data'>,
  ): Promise<Response> {
    return this.request<Response, Request>({
      ...(config ?? {}),
      url,
      method: 'patch',
      data,
    });
  },

  async delete<Response = unknown>(
    url: string,
    config?: Omit<AxiosRequestConfig<never>, 'url' | 'method'>,
  ): Promise<Response> {
    return this.request<Response, never>({
      ...(config ?? {}),
      url,
      method: 'delete',
    });
  },
};

export default ApiService;
