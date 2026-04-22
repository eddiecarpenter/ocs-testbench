# Screens Manifest

Offline reference for every screen in the OCS Testbench Figma file. Lets downstream agents (designers, developers) work without live Figma / MCP access.

## Sources in this folder

| File | Purpose |
|---|---|
| `OCS Testbench.fig` | Canonical source — open in Figma desktop to edit |
| `screens/*.png` | Per-frame PNG exports at 2× (see screenshot column below) |
| `SCREENS.md` (this file) | Structural index — node IDs, grid positions, per-screen notes, screenshot links |

> **Regenerating the PNGs:** requires a Figma Personal Access Token in `$FIGMA_TOKEN`. Call `GET https://api.figma.com/v1/images/huYHD8RurHwsamK9qM5ljH?ids=<comma-separated-ids>&format=png&scale=2` with header `X-Figma-Token: $FIGMA_TOKEN` to get signed CDN URLs, then download each to `docs/design/screens/<NN>-<slug>.png`. Run this after any Figma change so the PNGs stay in sync.

## Figma file

- **Key:** `huYHD8RurHwsamK9qM5ljH`
- **URL pattern:** `https://www.figma.com/design/huYHD8RurHwsamK9qM5ljH/?node-id=<nodeId>`
- Replace `<nodeId>` with the IDs below (Figma accepts both `1:55` and `1-55` forms).

## Canvas layout

Three rows, stride **1540 × 1124** (frame 1440×1024 + 100px gutter).

```
Row 1 (y=0)       Shell surfaces — navigable app pages
Row 2 (y=1124)    Scenarios + Scenario Builder tab variants
Row 3 (y=2248)    Executions + Execution Debugger states
```

---

## Row 1 — Shell surfaces

| # | Name | Node ID | Grid (x, y) | Screenshot | Purpose |
|---|---|---|---|---|---|
| 1 | Dashboard | `1:55` | (0, 0) | [`screens/01-dashboard.png`](./screens/01-dashboard.png) | Landing page. 4 KPI tiles (Peers, Subscribers, Scenarios, Active executions) + Peer status card + Recent executions card + Response-time chart. |
| 2 | Peers | `30:2` | (1540, 0) | [`screens/02-peers.png`](./screens/02-peers.png) | Peer registry list with inline row actions (RowActions popover at `45:2`). |
| 3 | Peers — Edit | `50:2` | (3080, 0) | [`screens/03-peers-edit.png`](./screens/03-peers-edit.png) | Peers page with the Edit drawer (`52:2`) open. |
| 4 | Subscribers | `65:2` | (4620, 0) | [`screens/04-subscribers.png`](./screens/04-subscribers.png) | Subscriber registry list with row-actions popover (`69:2`). |
| 5 | Subscribers — Edit | `70:2` | (6160, 0) | [`screens/05-subscribers-edit.png`](./screens/05-subscribers-edit.png) | Subscribers page with Edit drawer (`71:3`) open. |
| 6 | Settings | `208:2` | (7700, 0) | [`screens/06-settings.png`](./screens/06-settings.png) | Global settings surface. |

---

## Row 2 — Scenarios + Scenario Builder tabs

| # | Name | Node ID | Grid (x, y) | Screenshot | Purpose |
|---|---|---|---|---|---|
| 7 | Scenarios | `123:2` | (0, 1124) | [`screens/07-scenarios.png`](./screens/07-scenarios.png) | Flat list of scenarios, **grouped by unit type** (OCTET / TIME / UNITS section headers). Search, peer filter, New-scenario CTA. |
| 8 | Scenario Builder — Steps | `252:2` | (1540, 1124) | [`screens/08-builder-steps.png`](./screens/08-builder-steps.png) | Step list + step editor. Root frame for the builder shell; other builder frames share the same shell and differ only in the right-pane tab. |
| 9 | Scenario Builder — Frame | `263:2` | (3080, 1124) | [`screens/09-builder-frame.png`](./screens/09-builder-frame.png) | Frame tab — preview of the CCR AVP tree. Shows `Multiple-Services-Indicator (455)` as engine-managed (dimmed `1 · managed` value). AVP tree is structurally frozen at runtime (invariant 1, §8). |
| 10 | Scenario Builder — Services (multi-mscc) | `270:2` | (4620, 1124) | [`screens/10-builder-services-multi-mscc.png`](./screens/10-builder-services-multi-mscc.png) | Services tab, `serviceModel = multi-mscc`. Segmented control: Root / Single MSCC / Multi MSCC. Hint: *MSI=1 · one MSCC per selected service*. Left pane "SERVICES" (was "MSCC CATALOGUE"), right pane Service editor (was MSCC editor). |
| 11 | Scenario Builder — Variables | `275:2` | (6160, 1124) | [`screens/11-builder-variables.png`](./screens/11-builder-variables.png) | Variables tab — per-step variable auto-provisioning. Services sub-tab shows per-service slots. Naming: **flat** in root / single-mscc, **prefixed `RG<rg>_`** in multi-mscc. |
| 12 | Scenario Builder — Services (root, TIME) | `285:2` | (7700, 1124) | [`screens/12-builder-services-root-time.png`](./screens/12-builder-services-root-time.png) | Services tab, `serviceModel = root`, `unitType = TIME` (voice-session happy path). One implicit root service. Hint: *No MSI · RSU/USU on root (no MSCC)*. Editor omits Identifiers section; RSU uses `CC-Time · {{RSU}} · default 300`, USU uses `CC-Time · elapsed({{WINDOW_MS}})`. |

### serviceModel compatibility matrix (see ARCHITECTURE §4)

| unitType | root | single-mscc | multi-mscc |
|---|---|---|---|
| OCTET | — | ✓ | ✓ |
| TIME  | ✓ | ✓ | — |
| UNITS | ✓ | ✓ | — |

---

## Row 3 — Executions + Execution Debugger

| # | Name | Node ID | Grid (x, y) | Screenshot | Purpose |
|---|---|---|---|---|---|
| 13 | Executions | `153:2` | (0, 2248) | [`screens/13-executions.png`](./screens/13-executions.png) | Run history list — filterable, re-runnable. |
| 14 | Executions — Start Run Dialog | `164:2` | (1540, 2248) | [`screens/14-start-run-dialog.png`](./screens/14-start-run-dialog.png) | Executions page with the Start-Run dialog (`165:2`) open. Dialog fields: Scenario (required), Peer override (optional), Subscriber override (optional), Execution mode (Interactive / Continuous), Concurrency, Repeats. Concurrency/Repeats apply to Continuous mode only. |
| 15 | Execution Debugger — Paused | `290:2` | (3080, 2248) | [`screens/15-debugger-paused.png`](./screens/15-debugger-paused.png) | Interactive execution paused on a `pause` step. State chip: *Paused · pause step*. Service selection shows `RG 100` + `RG 200` active; context variables shown as `RG100_GRANTED`, `RG100_UNITS`, `RG100_FUI_ACTION` (multi-mscc prefixed naming). |
| 16 | Execution Debugger — Running | `298:2` | (4620, 2248) | [`screens/16-debugger-running.png`](./screens/16-debugger-running.png) | Execution mid-flight. Chip: *Running · sending step 2*. Footer CTAs: Pause / Abort. Body CTAs: Read-only, Auto-advancing, In flight. |
| 17 | Execution Debugger — Completed | `298:293` | (6160, 2248) | [`screens/17-debugger-completed.png`](./screens/17-debugger-completed.png) | Execution finished. Chip: *Completed · success*. Header CTAs: Export / View scenario / Re-run. CCR-Terminate (request type 3) shown. Total duration `3m 42s`. |

### Execution state transitions

`Paused ⇄ Running → Completed`

- **Paused** and **Running** are live interactive states.
- **Completed** is terminal (success or fail); no further transitions.

---

## Global fixtures visible across screens

- **Peers:** `peer-01` (connected), `peer-02` (disconnected), `peer-03` (flapping — CER/CEA timeout).
- **Subscribers:** `Alice Test` appears as the default in the Start-Run dialog.
- **Execution row fixtures** appear in Dashboard > Recent Executions, Scenarios list, and Executions page. Names are consistent across screens.

---

## Screen-level design decisions (from ARCHITECTURE.md)

| Decision | Screens affected | Notes |
|---|---|---|
| Scenario is the sole authoring unit | All shell screens | No separate authoring resource; users duplicate starters |
| Scenarios grouped by unit type | Scenarios list (`123:2`) | Section headers: OCTET / TIME / UNITS |
| serviceModel is a per-scenario choice | Builder Services tab (`270:2`, `285:2`) | Drives MSI, variable naming, AVP tree shape |
| MSI is engine-managed, not authored | Builder Frame tab (`263:2`) | Shown as dimmed read-only row in AVP tree |
| AVP tree frozen at runtime | All debugger states | Structure cannot change mid-run; only values can |
| Peer / Subscriber are per-run overrides | Start-Run dialog (`164:2`) | Optional; scenario carries defaults |

---

## Node-ID lookup (alphabetical)

| Node ID | Screen |
|---|---|
| `1:55` | Dashboard |
| `30:2` | Peers |
| `50:2` | Peers — Edit |
| `65:2` | Subscribers |
| `70:2` | Subscribers — Edit |
| `123:2` | Scenarios |
| `153:2` | Executions |
| `164:2` | Executions — Start Run Dialog |
| `208:2` | Settings |
| `252:2` | Scenario Builder — Steps |
| `263:2` | Scenario Builder — Frame |
| `270:2` | Scenario Builder — Services (multi-mscc) |
| `275:2` | Scenario Builder — Variables |
| `285:2` | Scenario Builder — Services (root, TIME) |
| `290:2` | Execution Debugger — Paused |
| `298:2` | Execution Debugger — Running |
| `298:293` | Execution Debugger — Completed |

---

## When screens change

1. Edit in Figma.
2. Re-export the affected PNG(s) via the REST API (see top of this file).
3. Update the affected row(s) in this manifest.
4. If a new screen is added, add it to the appropriate row table, the lookup, and bump the canvas-layout description.
5. Commit all three — `.fig`, the updated PNG(s) under `screens/`, and `SCREENS.md` — in the same commit so the repo stays consistent.
