# ARCHITECTURE.md — OCS Testbench

## Purpose

This document captures the evolving architectural design of the OCS Testbench
application. Sections are added incrementally as decisions are discussed and
locked in during architecture sessions.

**Status:** In progress — architecture discovery phase.

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

**MVP:** Single peer. The architecture supports N independent peers for:
- Simulating different CTF identities (different Origin-Host values)
- Scaled load testing (multiple peers to the same OCS/DRA)

---

## 9. Application Architecture

### API-first design

The application is a **Go HTTP server** exposing a REST and WebSocket API:

- **REST** — configuration CRUD, template management, scenario control
- **WebSocket** — real-time streaming of CCA responses, session state changes,
  connection status events

### Frontend

**React/TypeScript SPA** built as static assets and embedded in the Go binary
via `go:embed`. The frontend consumes the same API as any other client.

### Single binary

The Go binary serves both the API and the embedded UI. No external dependencies,
no separate frontend server.

### Deployment modes

| Mode | How |
|---|---|
| **Local (desktop feel)** | Run binary, auto-opens browser to `localhost:<port>` |
| **Docker** | `docker run -p <port>:<port> ocs-testbench` |
| **Kubernetes** | Standard deployment + service, exposed via ingress |
| **Headless / CI** | Drive the REST/WS API directly, no browser needed |

### Core separation

The system is layered:

```
┌─────────────────────────────────────────────┐
│  React/TS UI  (embedded via go:embed)       │
├─────────────────────────────────────────────┤
│  REST + WebSocket API                       │
├─────────────────────────────────────────────┤
│  Core library                               │
│  ┌──────────┐ ┌──────────┐ ┌─────────────┐ │
│  │ Diameter  │ │ Session  │ │  Template   │ │
│  │  Stack    │ │ Manager  │ │  Engine     │ │
│  └──────────┘ └──────────┘ └─────────────┘ │
└─────────────────────────────────────────────┘
```

The core library (Diameter stack, session management, template engine) has **no
dependency on the HTTP layer**. This enables headless use, CLI tooling, and
alternative frontends.

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

**SQLite** for local persistence, accessed via **sqlc** (type-safe Go generated
from SQL queries).

Persisted data:
- Peer configurations
- AVP templates
- Scenario definitions
- Execution history/logs

Using sqlc means the application code depends on generated Go interfaces, not
a specific database driver. Migrating to PostgreSQL (e.g. for K8s with shared
state) requires changing the SQL dialect and driver, not the application logic.

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

## 13. CCA Response Handling

The execution engine processes CCA responses at two levels:

### Level 1: Protocol behavior (built-in, non-configurable)

The engine automatically handles protocol-mandated responses:

- **Final-Unit-Indication** with `TERMINATE` → engine sends CCR-T
- **Validity-Time** → engine tracks grant expiry, re-authorises before timeout
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

## 14. Expression Evaluator

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

## 15. AVP Dictionary

The testbench uses **go-diameter's built-in XML dictionary** for AVP metadata:

- **Base Protocol** (RFC 6733) — connection-level AVPs
- **Credit Control** (RFC 4006) — Rating-Group, MSCC, RSU/USU, result codes
- **3GPP Ro/Rf** (TS 32.299) — PS-Information, IMS-Information, SMS-Information,
  and all 3GPP charging AVPs

For vendor-specific or proprietary AVPs, additional XML dictionary files can be
loaded at runtime.

The **template engine** uses dictionary lookups to:
- Resolve AVP names to codes, vendor IDs, and data types
- Validate templates at load time (reject unknown AVP names)
- Encode placeholder values into the correct Diameter data type
  (Unsigned32, UTF8String, OctetString, Address, Grouped, etc.)

---

## 16. Subscriber Table

Subscribers are a first-class entity. Every scenario requires a subscriber identity.

| Field | Type | Required | Description |
|---|---|---|---|
| MSISDN | string | Yes | Mobile number — maps to `Subscription-Id` (END_USER_E164) |
| ICCID | string | Yes | SIM identifier |
| IMEI | string | No | Device identifier |

Subscribers are persisted in SQLite and managed via the UI/API. In interactive
mode, the user selects a subscriber before starting. In continuous mode, the
subscriber is specified in the scenario configuration.

---

## 17. Future Considerations (not in MVP)

- **Load testing** — a subscriber pool (bulk MSISDNs) combined with a load profile
  (target concurrency, ramp-up rate). The engine picks subscribers from the pool
  and starts concurrent sessions, scaling to the target. Metrics: response times,
  success/failure rates, throughput (TPS), result code distribution.
- **MCP server frontend** — expose the testbench API as MCP tools, enabling AI
  agents to compose and execute Diameter test scenarios conversationally
- **SCTP transport** — if required by specific OCS deployments
- **Multi-peer management UI** — dashboard for managing N concurrent peers

---

<!-- Further architectural decisions will be recorded below -->
