# LOCALRULES.md — Local Overrides

This file contains project-specific rules and overrides that extend or
supersede the global protocol defined in `.ai/RULEBOOK.md`.

This file is never overwritten by a template sync.

---

## Template Source

Template: eddiecarpenter/ai-native-delivery

## Project

- **Name:** ocs-testbench
- **Topology:** Single
- **Stack:** Go, TypeScript, React
- **Description:** OCS Testbench — Diameter Gy Credit-Control traffic generator and test tool

## Repo

- **GitHub:** https://github.com/eddiecarpenter/ocs-testbench
- **Module:** github.com/eddiecarpenter/ocs-testbench

## Stack Notes

This is a mixed-stack project:
- **Go** — primary application (`cmd/`, `internal/`). Apply `standards/go.md`.
- **TypeScript + React** — SPA frontend under `web/src/`. Apply `standards/typescript.md` and `standards/react.md` when working in `web/`.

When making changes that span both stacks, run build verification for each stack independently.

## Commands

| Command | Description |
|---|---|
| `make build` | `go build ./...` |
| `make vet` | `go vet ./...` |
| `make test` | Unit tests only (excludes integration) |
| `make test-integration` | PostgreSQL tests via testcontainers-go (requires Docker) |
| `make test-all` | Both unit and integration passes |
| `make generate` | Regenerate sqlc bindings (`sqlc generate`) |
| `cd web && npm run build` | Build SPA into `web/dist` (go:embed target) |
| `cd web && npm run gen:api` | Regenerate TypeScript types from OpenAPI spec |

## Key Libraries

| Library | Purpose |
|---|---|
| `testcontainers-go` | Integration test PostgreSQL spin-up |
| `testify/assert` | Test assertions |
| `sqlc` | SQL → Go code generation |
| `golang-migrate` | Database migrations (`db/migrations/`) |
| `pgx` | PostgreSQL driver |

## Architecture Notes

- `internal/diameter` must NOT import `store` or any HTTP layer — this boundary is intentional
- Use `store.NewTestStore()` (in-memory) for unit tests — never require a real database in unit tests
- Integration tests use `//+build integration` tag and testcontainers-go
- Standard test helpers: `seedPeer`, `seedTemplate`, `seedSubscriber`, `fakePeerConnection`

## Session Init — Additional Context

On session initialisation, also read:
- `docs/ARCHITECTURE.md` — the evolving application architecture
