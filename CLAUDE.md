# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@AGENTS.md

---

## Commands

```bash
# Build
make build          # go build ./...
make vet            # go vet ./...

# Test
make test           # unit tests only (excludes integration)
make test-integration  # PostgreSQL tests via testcontainers-go (requires Docker)
make test-all       # both passes

# Run a single test
go test ./internal/diameter/manager/... -run TestManagerConnect -v

# Code generation
make generate       # regenerate sqlc bindings (sqlc generate)

# Frontend
cd web && npm run build    # build SPA into web/dist (go:embed target)
cd web && npm run gen:api  # regenerate TypeScript types from OpenAPI spec
```

**First-time setup:** `git clone --recurse-submodules` (submodules required). If already cloned: `git submodule update --init --recursive`.

---

## Architecture

The OCS Testbench acts as a **Charging Trigger Function (CTF)** — it sends Diameter Gy Credit-Control-Request (CCR) messages and processes Credit-Control-Answer (CCA) responses. It is a traffic-generation and test tool, not an OCS implementation.

**Deployment model:** single binary with an embedded React SPA (`go:embed web/dist`). REST + SSE API; no HTTP dependency in the core library.

### Key dependency constraint

`internal/diameter` does **not** import `store` or any HTTP layer. This boundary is intentional and must be maintained.

### Package map

| Package | Responsibility |
|---|---|
| `cmd/ocs-testbench` | Entry point — wires all subsystems, runs lifecycle |
| `internal/baseconfig` | YAML config loading |
| `internal/logging` | Structured `slog` logging with Bootstrap/Configure lifecycle |
| `internal/appl` | Prometheus metrics, signal handling, graceful shutdown, browser auto-open |
| `internal/store` | `Store` interface + pgx production impl + in-memory test impl; wraps sqlc bindings |
| `internal/diameter/conn` | Per-peer TCP/TLS Diameter connections — CER/CEA, DWR/DWA, reconnect backoff |
| `internal/diameter/dictionary` | AVP dictionary loader — built-in RFC 6733/4006/3GPP + custom XML from store |
| `internal/diameter/manager` | Multi-peer registry; fans out state transitions; manages auto-connect lifecycle |
| `internal/diameter/messaging` | CCR/CCA Go-native types, encoder/decoder, `Sender` interface implementation |
| `internal/diameter/protocol` | Decorator over `Sender`; enforces protocol-mandated CCA behaviour (FUI-TERMINATE, Validity-Time, 5xxx result codes) |
| `internal/template` | AVP rendering engine — resolves `{{PLACEHOLDER}}` tokens, validates AVP names, encodes to Diameter types |
| `web/src` | React/TypeScript SPA — scenario authoring, interactive step-through, real-time SSE streaming |

### Core interfaces

```go
// Store — the single coupling point to persistence (internal/store/store.go)
// Two implementations: NewStore (pgx) and NewTestStore (in-memory, for unit tests)

// Sender — contract between execution engine and Diameter stack
Send(ctx context.Context, peerName string, ccr *CCR) (*CCA, error)

// PeerConnection — per-peer lifecycle
Connect(ctx context.Context) error
Disconnect()
State() ConnectionState
Subscribe() <-chan StateEvent
```

### Data model

PostgreSQL with JSONB columns for schema evolution. Five entities: `peer`, `subscriber`, `avp_template`, `scenario`, `custom_dictionary`. Migrations in `db/migrations/` (golang-migrate).

### Startup order

Config → logging → pgx pool → Store → dictionaries → Diameter Manager → protocol.Behaviour → HTTP router → metrics server → HTTP server → signal handler. Shutdown is the reverse.

---

## Testing

- **Unit tests** — no build tags; use `store.NewTestStore()` (goroutine-safe in-memory). Run with `make test`.
- **Integration tests** — `//+build integration` tag; spin up PostgreSQL via testcontainers-go. Run with `make test-integration` (requires Docker).
- Standard helpers: `seedPeer(t, s, name)`, `seedTemplate(t, s, name)`, `seedSubscriber(t, s, params)`, `fakePeerConnection` for Diameter lifecycle stubs.
- Assertions via `testify/assert`.

---

## Configuration

Default config: `cmd/ocs-testbench/config.yaml`. Override via `CONFIG_FILE` env var or `-config` flag.

Key fields: `database_url`, `server.addr`, `metrics.addr`, `logging.format` (`text`/`json`), `headless`, `peers[]`.

---

## Code generation

- **sqlc** — SQL queries live in `internal/store/queries/*.sql`; generated bindings in `internal/store/sqlc/`. Run `make generate` after editing SQL.
- **OpenAPI → TypeScript** — spec at `api/openapi.yaml`; run `cd web && npm run gen:api` after changing the API spec.
