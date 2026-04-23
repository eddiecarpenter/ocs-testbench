# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | ac68760                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T12:44:00Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted the templates resource module, mock fixture, and fake-API handler; removed the nav entry, the KPI tile, all placeholder routes, and every import of the deleted files.
- **Files changed:** web/src/api/resources/templates.ts (deleted), web/src/mocks/data/templates.ts (deleted), web/src/mocks/fakeApi/templatesFakeApi.ts (deleted), web/src/mocks/index.ts, web/src/mocks/sse.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/pages/dashboard/kpis.ts, web/src/pages/dashboard/KpiCard.stories.tsx, web/src/layout/AppShell.tsx, web/src/layout/AppShell.stories.tsx, web/src/App.tsx
- **Decisions:** Build stayed broken on DashboardKpis call sites until #67 regenerated the schema; lint remained clean throughout.

### #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types
- **Implemented:** Regenerated src/api/schema.d.ts via `npm run gen:api` against the v0.2 OpenAPI spec. Replaced ExecutionResult with ExecutionState, dropped ExecutionStep for StepRecord, added PauseReason and ExecutionContextSnapshot aliases, renamed ListExecutionsParams.status → state, and updated every fixture and call site (executions.ts, executionDetails.ts, dashboardFakeApi.ts, executionsFakeApi.ts, sse.ts, RecentExecutionsCard.tsx) to the new field names. Added the minimum required fields to scenario fixtures so the build passes.
- **Files changed:** web/src/api/schema.d.ts, web/src/api/resources/executions.ts, web/src/mocks/data/executions.ts, web/src/mocks/data/executionDetails.ts, web/src/mocks/data/scenarios.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/mocks/fakeApi/executionsFakeApi.ts, web/src/mocks/sse.ts, web/src/pages/dashboard/RecentExecutionsCard.tsx
- **Decisions:** Scenario fixture alignment (a #68 concern) was partially done here because it was necessary to pass the build-clean acceptance criterion of #67. Task #68 can still extend the fixture audit across mocks/data and every handler.

## Remaining Tasks

- [ ] #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2 ← current
- [ ] #69 — Realign shell (nav, theme, layout chrome, dark mode) with current Figma
- [ ] #70 — Realign Dashboard screen with current Figma (01-dashboard.png)
- [ ] #71 — Realign Peers screen (list + edit drawer) with current Figma (02-peers.png, 03-peers-edit.png)
- [ ] #72 — Realign Subscribers screen (list + edit drawer) with current Figma (04-subscribers.png, 05-subscribers-edit.png)
- [ ] #73 — Realign Settings screen with current Figma (06-settings.png)
- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed
