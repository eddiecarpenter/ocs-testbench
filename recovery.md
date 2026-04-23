# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | 0421cfe                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T13:02:00Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted templates resource/mock/fake-API; removed nav, KPI tile, routes.
- **Files changed:** web/src/api/resources/templates.ts, web/src/mocks/data/templates.ts, web/src/mocks/fakeApi/templatesFakeApi.ts (deleted) + references updated across mocks, dashboard KPIs, AppShell, App, stories.

### #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types
- **Implemented:** Regenerated schema via `npm run gen:api`. ExecutionResult → ExecutionState, ExecutionStep → StepRecord, new PauseReason/ExecutionContextSnapshot aliases. Renamed ListExecutionsParams.status → state. All fixtures/call sites updated.
- **Files changed:** web/src/api/schema.d.ts, web/src/api/resources/executions.ts, web/src/mocks/data/executions.ts, web/src/mocks/data/executionDetails.ts, web/src/mocks/data/scenarios.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/mocks/fakeApi/executionsFakeApi.ts, web/src/mocks/sse.ts, web/src/pages/dashboard/RecentExecutionsCard.tsx

### #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2
- **Implemented:** Explicit return-type annotations on every fake-API reply body + shared ProblemBody alias. SseEventMap links event names to v0.2 payload schemas; dispatch/broadcast narrowed.
- **Files changed:** web/src/mocks/fakeApi/* (all 5) + web/src/mocks/sse.ts.

### #69 — Realign shell with current Figma
- **Implemented:** Nav label Execution → Executions with path /executions. Chrome borders and main bg switch via CSS light-dark(). Theme gained primaryShade for dark, explicit font/heading/line-height scales.
- **Files changed:** web/src/layout/AppShell.tsx, web/src/layout/AppShell.stories.tsx, web/src/App.tsx, web/src/pages/dashboard/kpis.ts, web/src/theme/theme.ts

### #70 — Realign Dashboard screen
- **Implemented:** KPI grid shrunk from 5 to 4 columns; "Active runs" → "Active executions" per Figma; skeleton count matches real tile count; KpiCard AllFive → AllFour story.
- **Files changed:** web/src/pages/dashboard/kpis.ts, web/src/pages/dashboard/DashboardPage.tsx, web/src/pages/dashboard/KpiCard.stories.tsx

### #71 — Realign Peers screen (list + edit drawer)
- **Implemented:** Row menu Start/Stop → Connect/Disconnect. Transport control changed from SegmentedControl to Select (dropdown) per Figma. Auto-connect label and subtitle match Figma. Cancel repositioned into the left footer slot when Delete is not available. Dark-mode-correct footer border via light-dark().
- **Files changed:** web/src/pages/peers/PeersPage.tsx, web/src/pages/peers/PeerForm.tsx

## Remaining Tasks

- [ ] #72 — Realign Subscribers screen (list + edit drawer) with current Figma ← current
- [ ] #73 — Realign Settings screen with current Figma
- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed
