# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | 5b3f129                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T12:40:30Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted the templates resource module, mock fixture, and fake-API handler; removed the nav entry, the KPI tile, all placeholder routes, and every import of the deleted files. Schema.d.ts retains stale TemplateSummary symbols that task #67 will clear when it regenerates types.
- **Files changed:** web/src/api/resources/templates.ts (deleted), web/src/mocks/data/templates.ts (deleted), web/src/mocks/fakeApi/templatesFakeApi.ts (deleted), web/src/mocks/index.ts, web/src/mocks/sse.ts, web/src/mocks/fakeApi/dashboardFakeApi.ts, web/src/pages/dashboard/kpis.ts, web/src/pages/dashboard/KpiCard.stories.tsx, web/src/layout/AppShell.tsx, web/src/layout/AppShell.stories.tsx, web/src/App.tsx
- **Decisions:** Build intentionally errors on two DashboardKpis call sites because the stale schema.d.ts still requires a `templates` field. Task #67 regenerates the schema and unblocks the build. Lint remains clean.

## Remaining Tasks

- [ ] #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types ← current
- [ ] #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2
- [ ] #69 — Realign shell (nav, theme, layout chrome, dark mode) with current Figma
- [ ] #70 — Realign Dashboard screen with current Figma (01-dashboard.png)
- [ ] #71 — Realign Peers screen (list + edit drawer) with current Figma (02-peers.png, 03-peers-edit.png)
- [ ] #72 — Realign Subscribers screen (list + edit drawer) with current Figma (04-subscribers.png, 05-subscribers-edit.png)
- [ ] #73 — Realign Settings screen with current Figma (06-settings.png)
- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed
