# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | de6ee99                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T12:49:00Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted the templates resource module, mock fixture, fake-API handler, KPI tile, nav entry, and all placeholder routes.
- **Files changed:** web/src/api/resources/templates.ts (deleted), web/src/mocks/data/templates.ts (deleted), web/src/mocks/fakeApi/templatesFakeApi.ts (deleted), plus references in mocks/index.ts, sse.ts, dashboardFakeApi.ts, kpis.ts, KpiCard.stories.tsx, AppShell.tsx, AppShell.stories.tsx, App.tsx.
- **Decisions:** Build stayed broken on DashboardKpis call sites until #67 regenerated the schema; lint remained clean throughout.

### #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types
- **Implemented:** Regenerated schema.d.ts via `npm run gen:api`. Replaced ExecutionResult with ExecutionState, dropped ExecutionStep for StepRecord, added PauseReason and ExecutionContextSnapshot aliases. Renamed ListExecutionsParams.status → state. Fixtures and call sites updated to the new field names. Scenario fixtures gained the now-required unitType/sessionMode/serviceModel/origin fields.
- **Files changed:** web/src/api/schema.d.ts, web/src/api/resources/executions.ts, web/src/mocks/data/executions.ts, web/src/mocks/data/executionDetails.ts, web/src/mocks/data/scenarios.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/mocks/fakeApi/executionsFakeApi.ts, web/src/mocks/sse.ts, web/src/pages/dashboard/RecentExecutionsCard.tsx
- **Decisions:** Scenario fixture was minimally aligned here because it blocked the v0.2 build.

### #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2
- **Implemented:** Added explicit return-type annotations on every fake-API reply body (Peer / Subscriber / Scenario / Execution / ExecutionPage / TacEntry / DashboardKpis / PeerTestResult / ResponseTimeSeries) plus a shared ProblemBody alias for error branches. Added SseEventMap that links each SSE event name to its v0.2 payload schema; dispatch and broadcast are now type-safe via generics and overloads. Execution detail 404 now emits a full RFC 7807 Problem.
- **Files changed:** web/src/mocks/fakeApi/peersFakeApi.ts, web/src/mocks/fakeApi/subscribersFakeApi.ts, web/src/mocks/fakeApi/scenariosFakeApi.ts, web/src/mocks/fakeApi/executionsFakeApi.ts, web/src/mocks/fakeApi/metricsFakeApi.ts, web/src/mocks/sse.ts
- **Decisions:** ProblemBody is a local alias in each mock file (kept inline to avoid module sprawl); SseEventMap is a new type because the OpenAPI SseEventPayload union does not bind names to payloads.

## Remaining Tasks

- [ ] #69 — Realign shell (nav, theme, layout chrome, dark mode) with current Figma ← current
- [ ] #70 — Realign Dashboard screen with current Figma (01-dashboard.png)
- [ ] #71 — Realign Peers screen (list + edit drawer) with current Figma
- [ ] #72 — Realign Subscribers screen (list + edit drawer) with current Figma
- [ ] #73 — Realign Settings screen with current Figma
- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed
