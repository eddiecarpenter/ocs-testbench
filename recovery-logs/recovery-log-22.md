# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #22                                |
| Branch              | feature/22-frontend-v2-realign     |
| Last commit         | 7f678de                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-23T13:10:00Z               |

## Completed Tasks

### #66 — Remove AVP Templates from frontend
- **Implemented:** Deleted templates resource/mock/fake-API; removed nav, KPI tile, routes.

### #67 — Regenerate TypeScript types from OpenAPI v0.2 and align Execution types
- **Implemented:** Regenerated schema via `npm run gen:api`. ExecutionResult → ExecutionState, ExecutionStep → StepRecord, new PauseReason/ExecutionContextSnapshot aliases. Renamed ListExecutionsParams.status → state. All fixtures/call sites updated.

### #68 — Align MSW mock handlers and fixtures with OpenAPI v0.2
- **Implemented:** Explicit return-type annotations on every fake-API reply + shared ProblemBody alias. SseEventMap links event names to v0.2 payload schemas; dispatch/broadcast narrowed.

### #69 — Realign shell with current Figma
- **Implemented:** Nav label Execution → Executions with path /executions. Chrome borders and main bg switch via CSS light-dark(). Theme gained primaryShade for dark, explicit font/heading/line-height scales.

### #70 — Realign Dashboard screen
- **Implemented:** KPI grid shrunk from 5 to 4 columns; "Active runs" → "Active executions" per Figma; skeleton count matches real tile count; KpiCard AllFive → AllFour story.

### #71 — Realign Peers screen
- **Implemented:** Row menu Start/Stop → Connect/Disconnect. Transport SegmentedControl → Select. Auto-connect wording per Figma. Dark-mode-correct footer border.

### #72 — Realign Subscribers screen
- **Implemented:** Dropped inline footer Cancel; chrome via light-dark(). Documented the `name`-column gap vs v0.2 contract.

### #73 — Realign Settings screen
- **Implemented:** Three sections per Figma — General (Theme/Auto-open/Log level), Diameter defaults (Origin-Host suffix, Origin-Realm, Watchdog interval, Default transport), SIM provisioning (existing MCCMNC), AVP Dictionaries (static fixture list; upload/remove disabled with explanatory tooltips because v0.2 has no /dictionaries endpoint). Settings module extended with new fields. Theme control reuses Mantine's useMantineColorScheme.
- **Files changed:** web/src/settings/settings.ts, web/src/pages/settings/SettingsPage.tsx
- **Decisions:** AVP Dictionaries upload/remove buttons disabled (no endpoint). Nokia/Ericsson custom entries rendered as fixture data for visual parity — to be replaced with a `useDictionaries()` query when the backend lands.

## Remaining Tasks

- [ ] #74 — Verification — build, lint, tests, dev-server proxy, go:embed ← current
