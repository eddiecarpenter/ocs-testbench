# ARCHITECTURE.md — OCS Testbench

## Purpose

This document captures the evolving architectural design of the OCS Testbench
application. Sections are added incrementally as decisions are discussed and
locked in during architecture sessions.

**Status:** Scoping complete — entering design and implementation phase.

---

## 1. System Role

The OCS Testbench acts as a **Charging Trigger Function (CTF)**, sending
Diameter Credit-Control-Request (CCR) messages to an OCS via the **Gy interface**
(RFC 4006) and processing Credit-Control-Answer (CCA) responses.

It is a **traffic generator and verifier** — it does not implement OCS logic.

---

## 2. Charging Models

Two charging models are supported:

| Model | Request Types | Lifecycle |
|---|---|---|
| **Session-based** | CCR-Initial → CCR-Update(s) → CCR-Terminate | Long-lived session with multiple exchanges |
| **Event-based** | CCR-Event (single request) | One-shot, no session lifecycle |

---

## 3. Configuration

Diameter peer endpoint configuration (host, port, realm, peer identity, transport
parameters) is **runtime-configurable** — the user can add, modify, or remove
endpoints without restarting the application. Configuration changes take effect
on next connection attempt.

---

## 4. Execution Modes

### Interactive (step mode)

A manual step-through mode where the user controls every request:

1. User selects a subscriber, template, and peer
2. User can optionally **load a scenario** (pre-defined step list) to pre-populate
   request values, or start from scratch using template defaults
3. User reviews and **modifies** request values (units, overrides, etc.) before sending
4. User clicks **Send** → CCR sent, CCA displayed
5. User adjusts values for the next step (or accepts pre-populated defaults)
6. User clicks **Terminate** when done → CCR-T sent, session ends

For events: same flow but a single CCR-Event, no session lifecycle.

If a scenario is loaded, it provides defaults for each step — but the user can
always override before sending. This merges "run a test case" and "explore
ad-hoc" into one mode.

### Continuous

An automated loop for volume/endurance testing:

1. Testbench sends CCR, processes CCA, immediately sends next CCR
2. Loop terminates on any of:
   - **User intervention** (manual stop)
   - **Funds exhausted** (CCA result code indicates insufficient balance)
   - **Iteration limit reached** (pre-configured count)

---

## 5. AVP Template System

The testbench is **service-type agnostic**. Instead of hardcoding service types
(SMS, USSD, VOICE, DATA), traffic profiles are defined via **AVP templates**.

A template specifies:

- The **Service-Information (873)** grouped AVP tree and its nested sub-AVPs
  (e.g. PS-Information, IMS-Information, SMS-Information)
- One or more **Multiple-Services-Credit-Control (456)** blocks, each with its
  own Rating-Group (432), Service-Identifier (439), and unit configuration
- **Static values** for fixed AVPs
- **Placeholders** (`{{VARIABLE}}`) for values substituted at runtime

Standard templates (SMS, USSD, VOICE, DATA) may ship as defaults, but users can
create, modify, and add templates freely.

### Template Engine Architecture

The template system is split into two layers with clean separation of concerns:

**Template loader** — reads template definitions from the store, assembles the
value map from three sources (static, variable, generated), applies step-level
overrides, and produces a well-defined input struct for the engine. The loader
defines the contract.

**Template engine** — a pure, stateless processor that accepts the loader's
output struct (or an equivalent struct from any caller). It validates AVP names
against the dictionary, resolves placeholders, handles vendor ID inheritance,
encodes values into correct Diameter data types, and produces the final AVP
tree. No store or HTTP dependency.

This split enables a future inline-template API endpoint that bypasses the
loader and calls the engine directly.

### Placeholder resolution order

1. **Variable** (runtime — user input, scenario context) overrides static
2. **Static** (defined in template) provides defaults
3. **Generated** (Session-Id, Charging-Id, etc.) fills in if no explicit value

### Vendor ID inheritance

A grouped AVP's Vendor-Id propagates to all its children unless a child
explicitly overrides it.

---

## 6. Session Composition

Two composition patterns are supported:

### Multi-session (different templates, same subscriber)

Multiple **independent Diameter sessions** (separate `Session-Id` values) run
concurrently against the same `Subscription-Id`. Each session uses a different
template.

**Example:** A subscriber simultaneously consumes data (DATA template, session A)
and makes a voice call (VOICE template, session B).

### Multi-MSCC (one template, multiple rating groups)

A **single Diameter session** carries multiple `Multiple-Services-Credit-Control`
AVPs within the same CCR, each with a different Rating-Group.

**Example:** One data session with Rating-Group 100 (browsing) and
Rating-Group 200 (streaming) in the same CCR.

---

## 7. Transport

- **MVP:** TCP only
- Plaintext by default; TLS is a connection-level configuration option
- SCTP is out of scope (deprecated in practice)

---

## 8. Peer Model

A **peer** is an independent Diameter connection with its own:

- Origin-Host / Origin-Realm
- Remote endpoint (host, port)
- Connection state (CER/CEA exchange)
- Session space

**Scenarios are bound to a peer.** A peer can run multiple concurrent scenarios.
A scenario cannot span peers.

**MVP supports N independent concurrent peers** for:

- Simulating different CTF identities (different Origin-Host values)
- Testing against multiple OCS/DRA endpoints simultaneously
- Scaled load testing (multiple peers to the same OCS/DRA)

---

## 9. Application Architecture

### API-first design

The application is a **Go HTTP server** exposing a REST API with SSE streaming:

- **REST** — configuration CRUD, template management, scenario execution control
- **SSE (Server-Sent Events)** — real-time server-to-client streaming of CCA
  responses, session state changes, connection status events, and per-step metrics

SSE was chosen over WebSocket because all real-time updates flow server → client.
Client-to-server actions (start, stop, proceed, modify) are discrete REST calls.
SSE is simpler (standard HTTP, auto-reconnect, no protocol upgrade), works
through all proxies/load balancers, and keeps the application to a single
protocol layer.

### Frontend

**React/TypeScript SPA** built as static assets and embedded in the Go binary
via `go:embed`. The frontend consumes the same API as any other client.

Dark mode / theme switching is included in the MVP. Storybook is used for
component prototyping and visual verification during development.

#### CRUD UI conventions

Applies to all resource pages (peers, subscribers, templates, scenarios, …):

- **List view** uses a Mantine `Table` with a kebab (`⋯`) action menu per row.
- **Create and edit** open a right-hand **Drawer** (not a modal) with a
  sectioned form body and footer actions.
- **Delete** is **not** exposed as a row action. It lives inside the edit
  drawer footer (red subtle button, left-aligned) so the full identity of
  the resource is visible at the moment of confirmation. This makes it much
  harder to accidentally delete the wrong row from a dense table.
- Delete itself still goes through a small confirmation modal before the
  DELETE call fires.
- Field-level validation errors (RFC 7807 `422` with `errors` map keyed by
  JSON Pointer) are routed onto the form fields via `@mantine/form`'s
  `setErrors()`; non-field errors surface as toasts.

### Single binary

The Go binary serves both the API and the embedded UI. No external dependencies,
no separate frontend server.

### Deployment modes

| Mode | How |
|---|---|
| **Local (desktop feel)** | Run binary, auto-opens browser to `localhost:<port>` |
| **Docker** | `docker run -p <port>:<port> ocs-testbench` |
| **Kubernetes** | Standard deployment + service, exposed via ingress |
| **Headless / CI** | Drive the REST/SSE API directly, no browser needed |

### Core separation

The system is layered:

```
┌─────────────────────────────────────────────────────┐
│  React/TS UI  (embedded via go:embed)               │
├─────────────────────────────────────────────────────┤
│  REST API + SSE Streaming                           │
├─────────────────────────────────────────────────────┤
│  Core library                                       │
│  ┌──────────┐ ┌───────────┐ ┌─────────────────────┐│
│  │ Diameter  │ │ Template  │ │    Execution        ││
│  │  Stack    │ │  Engine   │ │    Engine            ││
│  │          │ │           │ │ ┌─────────────────┐ ││
│  │ Connection│ │  Loader   │ │ │ Session Context │ ││
│  │ Manager  │ │     +     │ │ │ Step Executor   │ ││
│  │ Dictionary│ │  Engine   │ │ │ Orchestrator    │ ││
│  └──────────┘ └───────────┘ │ └─────────────────┘ ││
│                             └─────────────────────┘│
├─────────────────────────────────────────────────────┤
│  Store (PostgreSQL + sqlc)                          │
└─────────────────────────────────────────────────────┘
```

The core library has **no dependency on the HTTP layer**. This enables headless
use, CLI tooling, and alternative frontends.

### Variable sources for template placeholders

Placeholder values can come from multiple sources:

- **User input** — supplied via UI forms or API calls
- **Predefined** — static values defined in the template or scenario config
- **Generated** — auto-generated at runtime (e.g. Session-Id, Charging-Id)

A template can mix sources — some fields fixed, some user-supplied, some generated.

---

## 10. Diameter Library

The application uses **`fiorix/go-diameter`** for the Diameter base protocol
(connection management, CER/CEA, DWR/DWA, message encoding/decoding). The
Gy credit-control application layer (CCR/CCA, AVP construction) is built on
top of it.

---

## 11. Persistence

**PostgreSQL** for persistence, accessed via **sqlc** (type-safe Go generated
from SQL queries) with **pgx/v5** as the driver. Schema managed by
**golang-migrate**.

Persisted data:

- Peer configurations (JSONB body)
- AVP templates (JSONB body)
- Scenario definitions (JSONB step list)
- Subscribers (normalised columns)
- Custom dictionaries (XML content)
- Execution history/logs (shape TBD)

JSONB bodies store complex/variable structures; normalised columns store
queryable fields. PostgreSQL-specific query features (JSONB containment, array
types) are avoided to keep future SQLite migration feasible.

---

## 12. Scenario Model

A scenario is an **ordered list of steps** (step list). Each step specifies a
request type and its parameters. The execution engine processes steps sequentially.

```yaml
name: "Data session with decreasing grants"
template: DATA-session
peer: peer-01
execution: interactive   # or continuous
steps:
  - type: initial
    units: 1000
  - type: update
    units: 500
  - type: update
    units: 100
    overrides:
      Rating-Group: 200
  - type: terminate
```

### Key properties

- Each step can **override** template values (e.g. change Rating-Group mid-session)
- For **continuous** mode, the step list loops, with an optional iteration limit
- For **interactive** mode, the testbench pauses after each step for human review
- The step list is the **intermediate representation** — anything that can produce
  a valid step list can drive the execution engine

### Extensibility

The step list is structured data (JSON/YAML), making it trivially composable by:

- The UI (human builds steps via forms)
- The API (external tools submit step lists)
- AI agents (via MCP — generate step lists conversationally)

The execution engine only ever consumes step lists. What generates them is
a separate concern.

---

## 13. Execution Engine

The execution engine is structured as three layers:

### Session context

Created when a scenario execution starts, bound to a specific peer. Holds:

- The peer binding (which connection to send on)
- The context variable map (extractions, derived values, carried across steps)
- Session-level state (Session-Id, CC-Request-Number, execution mode)
- A `Sender` interface for dispatching CCR messages and receiving CCA responses
- A `MeasuredSender` decorator wrapping the `Sender` for metrics capture
- A metrics accumulator storing per-step metrics with a query API

### Step executor

A stateless processor called with: session context, previous `SendResult`
(nil for first step), and current step definition. It:

1. Evaluates the guard expression → skip if false
2. Runs extractions from previous CCA into context variables
3. Resolves derived values and template placeholders
4. Calls the template engine to construct the AVP tree
5. Calls `MeasuredSender.Send(CCR)` → receives `SendResult{CCA, Metrics}`
6. Runs assertions against the CCA
7. Evaluates result code handlers
8. Returns the step result

### Scenario orchestrator

Iterates the step list, feeding each step into the executor:

- **Interactive mode** — yields control to the caller after each step with the
  result and next step's pre-populated values. Caller can modify and signal proceed.
- **Continuous mode** — loops automatically, checking stop conditions.

### Sender interface and MeasuredSender

The `Sender` interface is the key abstraction — a simple contract: takes a CCR,
returns a CCA. The real implementation wraps the Diameter stack peer connection.
Test implementations return canned CCA responses.

The `MeasuredSender` is a decorator that wraps any `Sender` to capture per-request
metrics (round-trip time, send/receive timestamps, message sizes, result code,
transport errors) without modifying the Diameter stack's send method. It returns
a `SendResult{CCA, Metrics}`.

The session context accumulates all per-step metrics from the `MeasuredSender`
and exposes a query API (all metrics, by step index, summary aggregates).

---

## 14. CCA Response Handling

The execution engine processes CCA responses at two levels:

### Level 1: Protocol behavior (built-in, non-configurable)

The Diameter stack automatically handles protocol-mandated responses:

- **Final-Unit-Indication** with `TERMINATE` → sends CCR-T
- **Validity-Time** → tracks grant expiry, re-authorises before timeout
- **Result code 5xxx** (permanent failures) → session terminates
- **FUI with REDIRECT / RESTRICT_ACCESS** → handled per specification

These are not exposed to the scenario definition — they are Diameter protocol
compliance, not testing choices.

### Level 2: Scenario-driven response evaluation

Each step can define **extractions**, **guards**, and **assertions** evaluated
against the CCA response using an expression evaluator.

#### Extraction — bind CCA values to context variables

```yaml
  - type: update
    extract:
      GRANTED: "Response.MSCC[0].GrantedServiceUnit.CCTotalOctets"
      FUI_ACTION: "Response.MSCC[0].FinalUnitIndication.FinalUnitAction"
```

Extracted variables persist in the scenario context and are available as
template placeholders (`{{GRANTED}}`) in subsequent steps.

#### Guards — conditional step execution

```yaml
  - type: update
    when: "{{GRANTED}} >= 100"
    units: "percentage({{GRANTED}}, 80)"
```

If the guard evaluates to `false`, the step is skipped. Guards use the same
expression syntax as extraction.

#### Assertions — verify expected CCA values

```yaml
  - type: initial
    assert:
      - expr: "Response.ResultCode == 2001"
        message: "Expected SUCCESS"
      - expr: "Response.MSCC[0].GrantedServiceUnit.CCTotalOctets > 0"
        message: "Expected non-zero grant"
```

In **interactive** mode, failed assertions are highlighted for human review.
In **continuous** mode, failed assertions are logged and optionally stop the run.

#### Derived values — computed values for subsequent requests

Template placeholders can reference extracted variables and custom functions:

```yaml
  - type: update
    units: "percentage({{GRANTED}}, 80)"
```

### Result code handlers

For result codes where the action is a **testing choice** (not protocol-mandated),
the scenario defines handlers:

```yaml
result_handlers:
  4012:
    action: retry
    delay: from_validity_time
    max_retries: 3
  default:
    action: pause
```

Available actions:

| Action | Behavior |
|---|---|
| `continue` | Ignore the code, proceed to next step |
| `terminate` | Send CCR-T, end scenario |
| `retry` | Wait (fixed duration or from Validity-Time AVP), resend step |
| `pause` | Stop and wait for human decision |
| `stop` | End scenario without CCR-T (hard stop) |

---

## 15. Expression Evaluator

CCA response evaluation uses **`github.com/eddiecarpenter/ruleevaluator`** —
a general-purpose Go expression evaluator.

Capabilities used by the testbench:

- **Dot-path field resolution** — navigate CCA response structs (`Response.MSCC[0].GrantedServiceUnit.CCTotalOctets`)
- **Comparisons** — `==`, `!=`, `<`, `>`, `>=`, `<=`
- **Logical operators** — `&&`, `||`, `!`
- **Ternary expressions** — `condition ? valueA : valueB`
- **Variables** — `$var` substitution from scenario context
- **Custom functions** — domain-specific functions registered at startup (e.g. `percentage()`, `avpPresent()`)

The evaluator is fed the CCA response struct as its data context. Expressions
resolve fields directly against the response, with scenario variables available
for cross-step references.

---

## 16. AVP Dictionary

The testbench uses **go-diameter's built-in XML dictionary** for AVP metadata:

- **Base Protocol** (RFC 6733) — connection-level AVPs
- **Credit Control** (RFC 4006) — Rating-Group, MSCC, RSU/USU, result codes
- **3GPP Ro/Rf** (TS 32.299) — PS-Information, IMS-Information, SMS-Information,
  and all 3GPP charging AVPs

For vendor-specific or proprietary AVPs, additional XML dictionary files can be
loaded at runtime from the `custom_dictionary` table.

The **template engine** uses dictionary lookups to:

- Resolve AVP names to codes, vendor IDs, and data types
- Validate templates at load time (reject unknown AVP names)
- Encode placeholder values into the correct Diameter data type
  (Unsigned32, UTF8String, OctetString, Address, Grouped, etc.)

---

## 17. Subscriber Table

Subscribers are a first-class entity. Every scenario requires a subscriber identity.

| Field | Type | Required | Description |
|---|---|---|---|
| MSISDN | string | Yes | Mobile number — maps to `Subscription-Id` (END_USER_E164) |
| ICCID | string | Yes | SIM identifier |
| IMEI | string | No | Device identifier |

Subscribers are persisted in PostgreSQL and managed via the UI/API. In interactive
mode, the user selects a subscriber before starting. In continuous mode, the
subscriber is specified in the scenario configuration.

---

## 18. Monorepo Layout

```
ocs-testbench/
├── cmd/
│   └── ocs-testbench/          # Application entry point + config
├── internal/                    # Go backend
│   ├── appl/                   # App lifecycle (metrics, signals)
│   ├── baseconfig/             # Config loading, base structs
│   ├── logging/                # Structured logging (slog)
│   ├── store/                  # Database layer (sqlc)
│   ├── diameter/               # Diameter stack
│   ├── template/               # Template loader + engine
│   ├── engine/                 # Execution engine
│   └── api/                    # REST API + SSE handlers
├── db/
│   └── migrations/             # golang-migrate files
├── web/                        # React/TypeScript frontend
│   ├── src/                    # Source code
│   ├── .storybook/             # Storybook config
│   └── dist/                   # Build output (go:embed target, gitignored)
├── docs/                       # Project documentation
├── go.mod
├── sqlc.yaml
└── .gitignore
```

---

## 19. Bootstrap and Infrastructure

Application bootstrap adapts proven patterns from the charging-domain project:

- **`internal/baseconfig`** — YAML config loading, base config struct
- **`internal/logging`** — slog-based structured logging, Bootstrap/Configure lifecycle
- **`internal/appl`** — Prometheus metrics server, signal handling, graceful shutdown

The `cmd/ocs-testbench/main.go` entry point wires all components: config → logging
→ store → Diameter stack → API router → embedded frontend → HTTP server → signal
handler → graceful shutdown.

---

## 20. Future Considerations (not in MVP)

- **Load testing** — a subscriber pool (bulk MSISDNs) combined with a load profile
  (target concurrency, ramp-up rate). The engine picks subscribers from the pool
  and starts concurrent sessions, scaling to the target. Metrics: response times,
  success/failure rates, throughput (TPS), result code distribution.
- **MCP server frontend** — expose the testbench API as MCP tools, enabling AI
  agents to compose and execute Diameter test scenarios conversationally
- **SCTP transport** — if required by specific OCS deployments
