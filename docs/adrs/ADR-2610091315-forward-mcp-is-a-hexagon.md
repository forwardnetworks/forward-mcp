---
id: ADR-2610091315
status: accepted
date: 2026-10-09
---
# ADR-2610091315: forward-mcp is a hexagon, and the MCP server is one adapter of it

**Status:** Accepted
**Date:** 2026-10-09
**Drivers:** `internal/service` is one Go package of about 11,500 lines. One file,
`mcp_service.go`, holds 5,334 of them. MCP handlers, business rules, SQLite
storage, bloom indexes, embedding HTTP calls and the wiring all share one
package and one struct. `hexa analyze` grades the tree B (80) with zero
violations, but only because it classifies the whole package as use cases: the
code has no ports for a violation to cross.

## Context

Measured on `refactor/hexagonal` at `2a7c158`:

| Fact | Value |
|---|---|
| Grade | B, 80/100, 0 violations, 0 cycles |
| Files in a layer | 27 of 28 |
| Dead exports | 37 |
| `ForwardMCPService` fields | 13, every one built inside its own constructor |
| Forward API methods taking a `context.Context` | 4 of 36 |

Two defects follow from the shape:

1. **A client cannot cancel a tool call.** `addTool` drops the request context
   (`internal/service/mcp_adapter.go:32`), and `makeRequest` builds requests
   with `http.NewRequest` (`internal/forward/client.go:487`). A cancelled call
   keeps calling the Forward API to the end.
2. **Nothing can be tested without everything.** The use cases reach the
   concrete SQLite stores, the logger and the config loader directly, so a
   unit test of one tool constructs the whole service.

## Decision

The code moves to the hexagonal layout that `hexa analyze` checks:

| Layer | Package | Holds | May import |
|---|---|---|---|
| domain | `internal/domain` | Forward API types, NQE and memory types, config values | the standard library, minus I/O |
| ports | `internal/ports` | interfaces for every outside capability; type aliases that re-export domain types | `domain` |
| use cases | `internal/usecases` | the tool logic, with no MCP types | `domain`, `ports` |
| primary adapter | `internal/adapters/primary/mcp` | tool, prompt and resource registration; argument types; result formatting | `ports`, `usecases` |
| secondary adapters | `internal/adapters/secondary/...` | `forwardapi`, `sqlite`, `embeddings`, `bloom`, `cache`, `queryindex`, `envconfig`, `stderrlog` | `ports` |
| entry point | `cmd/server/main.go` | the composition root: builds each adapter and hands it to the use cases | anything |

Rules that carry the decision:

- **Every port method that can block takes a `context.Context` first.** The MCP
  request context reaches the HTTP request. This fixes defect 1.
- **The use cases receive their dependencies.** Nothing below the entry point
  calls a constructor of an adapter.
- **Configuration is data.** The config structs move to `domain`; reading the
  environment and `.env` is the `envconfig` adapter's job.
- **Logging is a port.** The use cases log through `ports.Logger`.
- **Behaviour does not change.** The 54 tools, 6 prompts and 1 resource keep
  their names, schemas and outputs. A changed tool is a separate decision.

The move happens in four phases, each one a commit with the gate green:

1. `domain`, `ports` and the `forwardapi` adapter, with contexts threaded from
   the MCP handler to the HTTP request.
2. Logger, config and storage ports: `sqlite`, `embeddings`, `bloom`, `cache`,
   `queryindex`.
3. `mcp_service.go` splits into `usecases` and `adapters/primary/mcp`; the
   composition moves to `cmd/server/main.go`.
4. Hardening: every file in a layer, dead exports removed, an import policy for
   `usecases`, docs updated.

## Consequences

- Every Forward API call is cancellable.
- A use case can be tested with fakes for its ports.
- Go's package-private access goes away at each boundary, so names that tests
  use today become exported or move with their tests.
- Import paths change for `scripts/` and `cmd/test-client`.

## Gate

```bash
CGO_ENABLED=1 go build ./... && CGO_ENABLED=1 go vet ./internal/... ./cmd/... && CGO_ENABLED=1 go test -race -count=1 -skip TestIntegration ./internal/... && hexa analyze . --violations-only --exit-code
```

Phase 4 adds `hexa analyze . --grade A`.
