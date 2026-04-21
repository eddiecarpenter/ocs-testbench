import axios from 'axios';

import { appConfig } from '../../config/app';
import { axiosRequestInterceptor } from './AxiosRequestInterceptor';
import { axiosResponseErrorInterceptor } from './AxiosResponseInterceptor';

/**
 * Shared axios instance for the whole application.
 *
 * All REST calls — real or mocked — MUST go through this instance. Two
 * reasons:
 *   1. `axios-mock-adapter` binds to a specific instance, so the mock layer
 *      only works when traffic flows here.
 *   2. Interceptors (auth header, request-id, 401 handling) are attached
 *      once, here, and apply uniformly to every call.
 */
export const AxiosBase = axios.create({
  baseURL: appConfig.apiBaseUrl,
  timeout: appConfig.apiTimeoutMs,
  headers: {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  },
});

AxiosBase.interceptors.request.use(axiosRequestInterceptor, (err) =>
  Promise.reject(err),
);

AxiosBase.interceptors.response.use(
  (response) => response,
  (error) => {
    axiosResponseErrorInterceptor(error);
    return Promise.reject(error);
  },
);

export default AxiosBase;
