# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | e4f6051                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T12:58:30Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted templates resource/mock/fake-API; removed nav, KPI tile, routes.
- **Files changed:** web/src/api/resources/templates.ts, web/src/mocks/data/templates.ts, web/src/mocks/fakeApi/templatesFakeApi.ts (all deleted) + references updated across mocks, dashboard KPIs, AppShell, App, stories.
- **Decisions:** Intentional intermediate build break on DashboardKpis; fixed by #67.

### #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types
- **Implemented:** Regenerated schema via `npm run gen:api`. ExecutionResult → ExecutionState, ExecutionStep → StepRecord, new PauseReason/ExecutionContextSnapshot aliases. Renamed ListExecutionsParams.status → state. All fixtures/call sites updated.
- **Files changed:** web/src/api/schema.d.ts, web/src/api/resources/executions.ts, web/src/mocks/data/executions.ts, web/src/mocks/data/executionDetails.ts, web/src/mocks/data/scenarios.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/mocks/fakeApi/executionsFakeApi.ts, web/src/mocks/sse.ts, web/src/pages/dashboard/RecentExecutionsCard.tsx
- **Decisions:** Scenario fixture aligned minimally here because it blocked the v0.2 build.

### #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2
- **Implemented:** Added explicit return-type annotations on every fake-API reply body + a shared ProblemBody alias for error branches. SseEventMap links event names to v0.2 payload schemas; dispatch/broadcast narrowed via generics and overloads. Execution detail 404 now emits a full RFC 7807 Problem.
- **Files changed:** web/src/mocks/fakeApi/peersFakeApi.ts, subscribersFakeApi.ts, scenariosFakeApi.ts, executionsFakeApi.ts, metricsFakeApi.ts, web/src/mocks/sse.ts
- **Decisions:** ProblemBody duplicated locally; SseEventMap is new because OpenAPI SseEventPayload is unnamed.

### #69 — Realign shell (nav, theme, layout chrome, dark mode) with current Figma
- **Implemented:** Nav label "Execution" → "Executions" with path /executions. Chrome borders and main bg switch via CSS light-dark() for dark-mode. Theme gained primaryShade for dark, explicit font/heading/line-height scales, and Input radius default.
- **Files changed:** web/src/layout/AppShell.tsx, web/src/layout/AppShell.stories.tsx, web/src/App.tsx, web/src/pages/dashboard/kpis.ts, web/src/theme/theme.ts

### #70 — Realign Dashboard screen with current Figma (01-dashboard.png)
- **Implemented:** KPI grid shrunk from 5 columns to 4 to match the Figma (Peers, Subscribers, Scenarios, Active executions). Renamed the fourth tile "Active runs" → "Active executions" to match the Figma. Skeleton count now tracks the real tile count. KpiCard story AllFive → AllFour.
- **Files changed:** web/src/pages/dashboard/kpis.ts, web/src/pages/dashboard/DashboardPage.tsx, web/src/pages/dashboard/KpiCard.stories.tsx

## Remaining Tasks

- [ ] #71 — Realign Peers screen (list + edit drawer) with current Figma ← current
- [ ] #72 — Realign Subscribers screen (list + edit drawer) with current Figma
- [ ] #73 — Realign Settings screen with current Figma
- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed
