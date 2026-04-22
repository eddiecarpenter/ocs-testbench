# API Contract

This directory holds the **OpenAPI specification** that is the single source
of truth for the HTTP contract between:

- the Go backend (`internal/api`) — which implements it
- the React frontend (`web/`) — which consumes it via generated types
- the MSW mock layer (`web/src/mocks`) — which mirrors it for offline dev

## Files

| File | Purpose |
|---|---|
| `openapi.yaml` | The specification. Edit this, nothing else downstream. |

## Workflow

1. **Edit `openapi.yaml`** when adding endpoints, fields, or changing shapes.
2. **Regenerate frontend types** with `cd web && npm run gen:api`. The script
   writes to `web/src/api/schema.d.ts` — the file is generated; do not edit.
3. **Implement on the backend** (Go handlers in `internal/api/`). JSON field
   names must match the spec exactly (use struct tags with camelCase).
4. **Mirror in MSW handlers** (`web/src/mocks/handlers.ts`) so dev and tests
   keep working against a realistic fake.

## Conventions

- **Base path:** `/api/v1/...`. The path version is incremented only for breaking contract changes.
- **JSON case:** `camelCase` in requests and responses.
- **Timestamps:** ISO 8601 UTC, e.g. `2026-04-21T12:00:00Z`. The frontend
  formats relative times ("12s ago") for display.
- **Enums:** lowercase (`connected`, `running`, `interactive`).
- **Errors:** RFC 7807 `application/problem+json` on all non-2xx responses.
- **Pagination:** envelope with `items` + `page` (`total`, `limit`, `offset`).
  Collection endpoints that grow unbounded use pagination; small fixed sets
  (peers, subscribers, scenarios) return a plain array.
- **IDs:** strings everywhere. Avoids JS number-precision issues, lets the
  backend choose UUID / ulid / integer later without breaking the wire format.

## Out of scope here

- **SSE event stream** — specified in `docs/ARCHITECTURE.md §9` and the
  frontend SSE client (`web/src/api/sse.ts`). Live updates flow over SSE,
  not REST.
