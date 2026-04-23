# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | 8dfe2cc                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T12:56:30Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted templates resource/mock/fake-API; removed nav, KPI tile, routes.
- **Files changed:** web/src/api/resources/templates.ts (deleted), web/src/mocks/data/templates.ts (deleted), web/src/mocks/fakeApi/templatesFakeApi.ts (deleted), plus references in mocks/index.ts, sse.ts, dashboardFakeApi.ts, kpis.ts, KpiCard.stories.tsx, AppShell.tsx, AppShell.stories.tsx, App.tsx.
- **Decisions:** Intentional intermediate build break on DashboardKpis; fixed by #67.

### #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types
- **Implemented:** Regenerated schema via `npm run gen:api`. ExecutionResult → ExecutionState, ExecutionStep → StepRecord, new PauseReason/ExecutionContextSnapshot aliases. Renamed ListExecutionsParams.status → state. All fixtures/call sites updated.
- **Files changed:** web/src/api/schema.d.ts, web/src/api/resources/executions.ts, web/src/mocks/data/executions.ts, web/src/mocks/data/executionDetails.ts, web/src/mocks/data/scenarios.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/mocks/fakeApi/executionsFakeApi.ts, web/src/mocks/sse.ts, web/src/pages/dashboard/RecentExecutionsCard.tsx
- **Decisions:** Scenario fixture aligned minimally here because it blocked the v0.2 build.

### #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2
- **Implemented:** Added explicit return-type annotations on every fake-API reply body (Peer/Subscriber/Scenario/Execution/ExecutionPage/TacEntry/DashboardKpis/ResponseTimeSeries/PeerTestResult) + a shared ProblemBody alias for error branches. SseEventMap links event names to v0.2 payload schemas; dispatch/broadcast narrowed via generics and overloads. Execution detail 404 now emits a full RFC 7807 Problem.
- **Files changed:** web/src/mocks/fakeApi/peersFakeApi.ts, subscribersFakeApi.ts, scenariosFakeApi.ts, executionsFakeApi.ts, metricsFakeApi.ts, web/src/mocks/sse.ts
- **Decisions:** ProblemBody alias duplicated locally in each fakeApi file (no shared wrapper module); SseEventMap is new because OpenAPI SseEventPayload is an unnamed union.

### #69 — Realign shell (nav, theme, layout chrome, dark mode) with current Figma
- **Implemented:** Nav label "Execution" → "Executions" (matches v2 Figma and /executions API path). Route path updated across App.tsx, AppShell.stories.tsx, and the dashboard KPI tile. Chrome borders and main background switch via CSS light-dark() so dark mode renders cleanly (previous gray-0 hex was light-only). Theme gained primaryShade for dark, explicit font/heading/line-height scales, and an Input radius default.
- **Files changed:** web/src/layout/AppShell.tsx, web/src/layout/AppShell.stories.tsx, web/src/App.tsx, web/src/pages/dashboard/kpis.ts, web/src/theme/theme.ts
- **Decisions:** Routes keep placeholders for /scenarios and /executions (shell links them; per-screen features are out of this feature's scope).

## Remaining Tasks

- [ ] #70 — Realign Dashboard screen with current Figma (01-dashboard.png) ← current
- [ ] #71 — Realign Peers screen (list + edit drawer) with current Figma
- [ ] #72 — Realign Subscribers screen (list + edit drawer) with current Figma
- [ ] #73 — Realign Settings screen with current Figma
- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed
