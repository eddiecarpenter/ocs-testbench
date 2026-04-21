/**
 * Mock REST layer.
 *
 * Importing this module installs `axios-mock-adapter` onto the shared
 * Axios instance and registers every fake API handler below. Any route
 * not matched falls through to the real network (`passThrough`).
 *
 * Gated by `appConfig.useMockApi` — see `main.tsx` for the conditional
 * dynamic import. This file must be a pure side-effect module; it
 * intentionally has no exports.
 */

import './fakeApi/dashboardFakeApi';
import './fakeApi/peersFakeApi';
import './fakeApi/subscribersFakeApi';
import './fakeApi/templatesFakeApi';
import './fakeApi/scenariosFakeApi';
import './fakeApi/executionsFakeApi';
import './fakeApi/metricsFakeApi';

import { mock } from './MockAdapter';
import { installMockSse } from './sse';

// Anything that hasn't been explicitly mocked goes through to the real
// backend. Prevents surprises as the spec grows.
mock.onAny().passThrough();

// Install the mock SSE transport so `new EventSource(...)` in the app
// points at the in-memory emitter instead of the (non-existent) server.
installMockSse();
