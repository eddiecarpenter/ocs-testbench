# Project Brief — OCS Testbench

## What this is

The OCS Testbench is a Diameter Gy traffic generator and verifier for testing
Online Charging Systems (OCS). It acts as a Charging Trigger Function (CTF),
sending Credit-Control-Request (CCR) messages to an OCS endpoint and processing
Credit-Control-Answer (CCA) responses.

It is a testing tool — it does not implement OCS or charging logic.

## Problem statement

Testing an OCS requires generating realistic Diameter Gy traffic: session-based
charging (Initial → Update → Terminate), event-based charging, multiple service
types, multiple rating groups, and varied subscriber scenarios. Existing tools
are either proprietary, inflexible, or require complex scripting to drive.

The OCS Testbench provides a lightweight, configurable, API-first application
that enables both manual step-through testing and automated continuous runs
against any Diameter Gy endpoint.

## Who it is for

- **OCS developers and testers** — verifying charging logic, grant behaviour,
  and result code handling
- **Integration engineers** — validating Diameter connectivity and AVP compliance
- **Operations teams** — smoke-testing OCS deployments and configuration changes

## Key capabilities

### Traffic generation

- Session-based charging (CCR-I / CCR-U / CCR-T)
- Event-based charging (CCR-E)
- Service-type agnostic via AVP templates (SMS, USSD, VOICE, DATA, custom)
- Multiple rating groups and service identifiers per session (multi-MSCC)
- Multiple concurrent sessions per subscriber (multi-session)

### Execution modes

- **Interactive (step mode)** — manual control of every request with ability to
  modify values between steps; optionally pre-populate from a saved scenario
- **Continuous** — automated loop with configurable stop conditions (user stop,
  funds exhausted, iteration limit)

### Response handling

- Protocol-mandated behaviour built-in (Final-Unit-Indication, Validity-Time,
  permanent failures)
- Configurable result code handlers (retry, terminate, pause, continue)
- Value extraction from CCA responses into scenario context variables
- Guards and assertions evaluated against responses using expression evaluator
- Derived values fed back into subsequent requests via template placeholders

### Configuration

- Runtime-configurable Diameter peer endpoints (no restart required)
- Multiple concurrent peer connections with independent identities
- AVP templates with placeholder substitution (user input, predefined, generated)
- Subscriber management (MSISDN, ICCID, optional IMEI)
- Scenario definitions as ordered step lists (the intermediate representation)

## Technology stack

| Component | Technology |
|---|---|
| Backend | Go |
| Diameter | `fiorix/go-diameter` (Gy/credit-control layer built on top) |
| Expression evaluator | `github.com/eddiecarpenter/ruleevaluator` |
| Frontend | React / TypeScript SPA |
| Persistence | PostgreSQL + sqlc |
| Real-time streaming | Server-Sent Events (SSE) |
| Packaging | Single binary (Go backend + embedded UI via `go:embed`) |

## Architecture

The application follows an API-first design:

- **REST API** — configuration CRUD, template management, scenario control
- **SSE** — real-time streaming of responses, session state, connection status
- **Core library** — Diameter stack, execution engine, template engine (no HTTP dependency)

See `docs/ARCHITECTURE.md` for the full architectural design.

## Deployment

| Mode | Description |
|---|---|
| Local | Single binary, auto-opens browser |
| Docker | Container image |
| Kubernetes | Standard deployment + service |
| Headless | REST/SSE API only, no browser |

## MVP scope

- Multiple concurrent Diameter peer connections (N independent peers)
- TCP transport, plaintext default (TLS configurable)
- Interactive step mode and continuous mode
- AVP template system with placeholder substitution
- Scenario step lists with extraction, guards, assertions
- Subscriber table (MSISDN, ICCID, IMEI)
- PostgreSQL persistence
- React/TS web UI with dark mode
- REST + SSE API
- Per-step response metrics (round-trip time, message sizes, result codes)

## Future considerations

- **Load testing** — subscriber pool, concurrent session scaling, metrics
- **MCP server** — expose API as MCP tools for AI-driven test composition
- **SCTP transport** — if required by specific OCS deployments

## Repository

- **GitHub:** https://github.com/eddiecarpenter/ocs-testbench
- **Stack:** Go + React/TypeScript
- **Topology:** Single (embedded)
