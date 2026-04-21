import MockAdapter from 'axios-mock-adapter';

import AxiosBase from '../api/axios/AxiosBase';

/**
 * Single `axios-mock-adapter` instance bound to the shared Axios base.
 * Handlers in `fakeApi/*.ts` register onto this. Anything not matched
 * falls through to the real network (see `index.ts`).
 */
export const mock = new MockAdapter(AxiosBase, {
  // Do not block real requests by default — `.onAny().passThrough()` in
  // `index.ts` handles the fall-through policy.
  delayResponse: 0,
});
