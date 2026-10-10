# Forward MCP

**Version 4.3.1** • [![Architecture Grade](https://img.shields.io/badge/hexa-A%2B%20100%2F100-brightgreen)](https://github.com/gaberger/hexa)

Forward MCP is an open-source server that provides a set of tools and APIs for interacting with Forward Networks' platform. It enables automation, analysis, and integration with network data using the Model Context Protocol (MCP).

Built with hexagonal architecture for clean separation of concerns and easy extensibility. Runs locally over stdio, or as a remote server over Streamable HTTP.

## Features
- **54 High-Quality MCP Tools**: All tools follow Composio-inspired design standards with consistent descriptions, clear parameters, and proper format hints
- **Hexagonal Architecture**: Clean separation between business logic (use cases), I/O (adapters), and interfaces (ports). Architecture grade: A+ 100/100
- **Official MCP Go SDK v1.7.0**: Protocol revision 2026-07-28, negotiates down to 2024-11-05 for compatibility
- **Agent-Friendly Design**: LLM-optimized tool descriptions, explicit required/optional parameters, actionable error messages
- **Tool Behavior Annotations**: `readOnlyHint`, `destructiveHint`, `idempotentHint` help clients distinguish safe from destructive operations
- **Comprehensive Documentation**: Forward-MCP guide skill (439 lines) with workflows, patterns, troubleshooting
- **Semantic Cache**: AI-powered query result caching with embedding-based similarity matching
- **Knowledge Graph Memory**: Entity-Relation-Observation model for storing network discoveries
- **Bloom Filter Search**: Automatic optimization for large datasets (>100 items) with 80%+ memory reduction
- **Security Hardened**: TLS 1.3+ enforcement, SHA-256 hashing, path traversal protection, race-condition free
- **Remote Server Mode (preview)**: Streamable HTTP transport (plus legacy SSE) with API-key or JWT (JWKS) authentication, per-user rate limits, and health endpoints
- **BM25 Query Search**: ranks the NQE query library by term rarity and field weight, fused with embedding similarity when embeddings exist

## What's New in 4.3.1

### Zero-Configuration First Run
The server now sets itself up automatically on first startup. No manual steps required.

**What happens automatically:**
1. **Query database hydration** — loads ~1800+ queries from the Forward Networks API
   - Both org queries (your custom queries) and fwd queries (Forward's official library)
   - Full metadata including descriptions (not available in static spec files)
   - Runs in background during startup (non-blocking)
2. **Keyword embedding generation** — creates semantic search embeddings
   - Free, fast, keyword-based (no API key needed)
   - Enables semantic query search immediately
   - Works for binary installs (no source/make commands needed)
3. **Continuous synchronization** — background refresh on every startup
   - Checks for new or changed queries using incremental API updates
   - Adapts automatically when Forward Networks releases library updates
   - Only fetches queries with new commit IDs (efficient)

**First-run experience**: Start the server. It handles the rest.

**Why this matters**: The NQE library changes frequently. The system now stays current automatically without manual intervention.

## What's New in 4.2.0

### Snapshot Comparison
New `compare_nqe_results` tool compares the same NQE query across two snapshots, showing what changed after a maintenance window or configuration update.

### Enhanced Path Search
The `search_paths` tool now accepts:
- **ICMP**: `icmp_type` parameter for tracing ping and ICMP traffic
- **TCP flags**: `fin`, `syn`, `rst`, `psh`, `ack`, `urg` (0 or 1 each) for connection analysis
- **Layer 7**: `app_id`, `user_id`, `user_group_id`, `url`, `domain` for application-aware tracing
- **Display**: `include_tags` option to show device tags in path results

### NQE Query Version Control
`run_nqe_query_by_id` now accepts:
- `commit_id` — run a specific version of a query
- `use_latest_data_files` — use uploaded data files instead of snapshot versions

### Documentation Cleanup
Removed 23 obsolete files (10,872 lines) including stale guides, outdated planning docs, and broken test scripts.

## What's New in 4.1.0

### Remote Server Mode — Streamable HTTP Transport (ADR-2610091600, preview)
You can now run forward-mcp on a separate host and connect to it over the network. The Forward API credentials stay on that one server, not on every laptop.

- **New primary adapter**: `internal/adapters/primary/httpserver/`. The use cases did not change.
- **Authentication**: API keys, or JWT validated against your identity provider's JWKS keys (refreshed every hour).
- **TLS required by default**: the server refuses to start without a certificate unless you set `FORWARD_HTTP_ALLOW_INSECURE=true`.
- **Safe CORS**: a `*` origin is rejected; origins must be `https://` (or `http://localhost` for development).
- **Rate limits**: a token bucket per user, 100 requests per minute by default.

See [Remote Server Mode](#remote-server-mode-streamable-http-preview) for setup, and [Known Limitations](#known-limitations-of-remote-server-mode) before you deploy it.

### BM25 Query Search
`search_nqe_queries` now ranks the query library with Okapi BM25. A rare word like "bgp" counts for more than a common word like "device", and a word in a query's intent counts three times as much as a word in its code.

- **Hybrid when embeddings exist**: the BM25 ranking and the embedding ranking are merged with Reciprocal Rank Fusion. A query that has no embedding yet is still found.
- **Scores stay between 0 and 1**: the old keyword score could exceed 5, which the tools showed as "500% similarity".
- **Offline search works again**: the bundled `spec/NQELibrary.json` has paths but no descriptions, and the loader used to drop all 1,879 queries. It now keeps them and searches on the path.

## What's New in 4.0.0

### Hexagonal Architecture (ADR-2610091315)
Complete refactoring from monolithic 5,334-line service file to clean hexagonal layers:

- **Domain Layer**: Pure business types with zero external dependencies (`internal/domain/`)
- **Ports Layer**: Interfaces defining what adapters must provide (`internal/ports/`)
- **Use Cases Layer**: Business logic orchestration with no direct I/O (`internal/usecases/`)
- **Primary Adapters**: MCP server registration (`internal/adapters/primary/mcpserver/`)
- **Secondary Adapters**: API client, SQLite stores, cache, embeddings (`internal/adapters/secondary/`)

**Result**: Zero boundary violations, zero circular dependencies, 100% testable components, A+ 100/100 architecture grade.

### Tool Quality Standards (ADR-2610091555)
All 54 MCP tools rewritten to follow Composio-inspired quality standards:

- **Consistent Template**: Every tool uses "Tool to X. Use when Y. Requires Z. Returns W." format
- **Context Savings**: ~6,200 characters saved across all tool descriptions
- **Format Hints Corrected**: Removed incorrect `format=uuid` (network_id/snapshot_id are plain strings per API spec)
- **Explicit Parameters**: All parameters state "Required" or "Optional" with clear defaults
- **No Clutter**: Zero emojis, zero markdown formatting, zero verbose examples in descriptions
- **Agent Workflows**: 439-line forward-mcp-guide skill with discovery patterns, troubleshooting, best practices

**Result**: 100% template compliance, better agent behavior, clearer tool usage.

### Security Improvements
Critical security hardening across the codebase:

- **SHA-256 Hashing**: Instance IDs use SHA-256 (not MD5) for cryptographic security
- **TLS 1.3+ Enforcement**: Minimum TLS 1.3, secure cipher suites only, no certificate bypass
- **Path Traversal Protection**: All file paths validated before filesystem operations
- **Race Condition Fixes**: Atomic operations for shared state, passes `-race` detector
- **Credential Zeroing**: API credentials zeroed from memory after use
- **SQL Injection Prevention**: LIKE pattern escaping for user input

### MCP SDK Upgrade
Upgraded to `github.com/modelcontextprotocol/go-sdk` v1.7.0:

- Protocol version 2026-07-28 (negotiates down to 2024-11-05)
- Fixed 9 tools with dummy parameters (MCP spec violation)
- Improved error handling and tool schema validation

### Breaking Changes
**Import paths changed** due to hexagonal architecture:

```go
// Before (v3.x)
import "github.com/forward-mcp/internal/service"

// After (v4.x)
import (
    "github.com/forward-mcp/internal/usecases"
    "github.com/forward-mcp/internal/adapters/primary/mcpserver"
    "github.com/forward-mcp/internal/adapters/secondary/forwardapi"
)
```

**Functional compatibility**: 100% maintained. All tools work the same, only descriptions improved.

See `CHANGELOG-v4.0.0.md` for complete details.

## Architecture

Forward-MCP follows **hexagonal architecture** (ports & adapters) for clean separation of concerns:

```
┌─────────────────────────────────────────────┐
│         MCP Client (Claude, etc)            │
└──────────┬───────────────────────┬──────────┘
           │ stdio (local)         │ HTTP (remote)     
┌──────────▼───────────────────────▼──────────┐
│           Primary Adapters                  │
│  mcpserver/  - registers tools & prompts    │
│  httpserver/ - HTTP, auth, rate limit, TLS  │
└────────────────┬────────────────────────────┘
                 │
┌────────────────▼────────────────────────────┐
│          Use Cases Layer                    │
│        internal/usecases/                   │
│  • 54 MCP tool implementations              │
│  • Business logic orchestration             │
│  • No direct I/O                            │
└──┬───────────────────────────────────────┬──┘
   │                                       │
   │ Uses ports (interfaces)               │
   │                                       │
┌──▼───────────────────┐    ┌──────────────▼───┐
│   Domain Layer       │    │    Ports Layer   │
│   internal/domain/   │    │  internal/ports/ │
│ • Pure business types│    │ • Interfaces     │
│ • Zero dependencies  │    │ • Contracts      │
└──────────────────────┘    └──────────────────┘
                 │
┌────────────────▼────────────────────────────┐
│      Secondary Adapters (I/O)               │
│   internal/adapters/secondary/              │
│  • forwardapi/   - API client               │
│  • sqlite/       - Databases                │
│  • semcache/     - Semantic cache           │
│  • embeddings/   - Vector embeddings        │
│  • queryindex/   - Query search             │
│  • bloom/        - Bloom filters            │
│  • envconfig/    - Configuration            │
│  • stderrlog/    - Logging                  │
└─────────────────────────────────────────────┘
```

**Key Files:**
- `cmd/server/main.go` - Entry point, composes adapters and starts server
- `internal/usecases/service.go` - Main service orchestrator
- `internal/adapters/primary/mcpserver/server.go` - MCP protocol handler
- `internal/adapters/primary/httpserver/` - Streamable HTTP + legacy SSE transport, auth, middleware
- `internal/adapters/secondary/forwardapi/client.go` - Forward Networks API client
- `.claude/skills/forward-mcp-guide/SKILL.md` - Agent workflow guide (439 lines)

**Architecture Validation:**
```sh
hexa analyze . --grade A+
# Result: A+ 100/100, zero violations, zero cycles
```

## Prerequisites
- Go 1.26 or later
- CGO enabled (required for SQLite)
- Access to Forward Networks API (API URL and API Key)

## Build Instructions
```sh
git clone https://github.com/gaberger/forward-mcp.git
cd forward-mcp
CGO_ENABLED=1 go build -o forward-mcp ./cmd/server
# or: make build
```

## Run Instructions
Set the following environment variables before running:
- `FORWARD_API_BASE_URL` – Base URL for the Forward Networks API
- `FORWARD_API_KEY` – Your Forward Networks API key
- `FORWARD_API_SECRET` - Your Forward Networks API Secret
- `FORWARD_DEFAULT_NETWORK_ID` – (Optional) Default network ID

Note: TLS 1.3 is enforced for all API connections; certificate verification cannot be disabled.

### Self-Signed Certificates

Self-signed and internal-CA certificates are supported. You trust the specific certificate; checks are never switched off, so nobody on the network can read your API keys.

**Forward server with a self-signed certificate.** Save its certificate and point `FORWARD_CA_CERT_PATH` at it:
```sh
openssl s_client -connect forward.example.com:443 -showcerts </dev/null \
  | openssl x509 -outform PEM > forward.pem
export FORWARD_CA_CERT_PATH=$PWD/forward.pem
```
Check the fingerprint (`openssl x509 -in forward.pem -noout -fingerprint -sha256`) against the server before you trust it. If an internal CA issued the certificate, use the CA certificate instead. The file is added to the system trust store, and the server refuses to start if the file is missing or holds no certificate.

**Identity provider with a self-signed certificate (JWT mode).** Set `FORWARD_HTTP_JWKS_CA_CERT` the same way.

**This server with a self-signed certificate (remote mode).** Pass it as `FORWARD_HTTP_TLS_CERT` / `FORWARD_HTTP_TLS_KEY`; any valid certificate works. Clients must trust it. For Claude Code, set `NODE_EXTRA_CA_CERTS=/path/to/cert.pem` before starting it.

Common errors, and what they mean:
- *"not trusted"*: set `FORWARD_CA_CERT_PATH` as above.
- *"does not match its host name"*: use the host name the certificate was issued for. Certificates that set only the Common Name, with no Subject Alternative Name, are rejected. Reissue them with a SAN.
- *"does not support TLS 1.3"*: the Forward server, or a proxy in front of it, only offers TLS 1.2.

### Bloomsearch Configuration (Optional)
- `FORWARD_BLOOM_ENABLED` – (Optional, default: true) Enable bloomsearch for large results
- `FORWARD_BLOOM_THRESHOLD` – (Optional, default: 100) Minimum result size to trigger bloom filter creation
- `FORWARD_BLOOM_INDEX_PATH` – (Optional, default: data/bloom_indexes) Path for bloom index storage

### Instance Lock Configuration (Optional)
- `FORWARD_LOCK_DIR` – (Optional, default: /tmp) Directory for server instance lock file

Run the server:
```sh
./forward-mcp
```

The server will start and listen for MCP protocol messages via stdio (compatible with Claude Desktop, Claude Code, and other MCP clients).

## Remote Server Mode (Streamable HTTP, preview)

Run forward-mcp on one host and connect to it from other machines.

### Start the server

Production (TLS and JWT):
```sh
FORWARD_HTTP_ENABLED=true \
FORWARD_HTTP_PORT=8080 \
FORWARD_HTTP_TLS_CERT=/etc/forward-mcp/tls.crt \
FORWARD_HTTP_TLS_KEY=/etc/forward-mcp/tls.key \
FORWARD_HTTP_AUTH_MODE=jwt \
FORWARD_HTTP_JWT_ISSUER=https://auth.example.com/ \
FORWARD_HTTP_JWT_AUDIENCE=forward-mcp \
FORWARD_HTTP_JWT_PUBLIC_KEY_URL=https://auth.example.com/.well-known/jwks.json \
./forward-mcp
```

Local development (no TLS, API key):
```sh
FORWARD_HTTP_ENABLED=true \
FORWARD_HTTP_ALLOW_INSECURE=true \
FORWARD_HTTP_HOST=127.0.0.1 \
FORWARD_HTTP_AUTH_MODE=api-key \
FORWARD_HTTP_API_KEYS="dev-key:alice" \
./forward-mcp
```

Do not set `FORWARD_HTTP_ALLOW_INSECURE=true` on a server that other machines can reach. Without TLS, API keys and tokens cross the network in plain text.

### Endpoints

| Path | Auth | Purpose |
|------|------|---------|
| `/mcp` | Required | MCP over Streamable HTTP (current MCP transport) |
| `/sse` | Required | MCP over the older HTTP+SSE transport, for clients that need it |
| `/health` | None | Returns 200 while the process runs |
| `/ready` | None | Returns 200 when the server can take traffic |

### Connect a client

Claude Code:
```sh
claude mcp add --transport http forward-mcp https://forward-mcp.example.com/mcp \
  --header "Authorization: Bearer <api-key-or-jwt>"
```

Check that the server is up:
```sh
curl https://forward-mcp.example.com/health
```

### Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `FORWARD_HTTP_ENABLED` | `false` | Turns on the HTTP transports. Stdio is not started when this is on. |
| `FORWARD_HTTP_HOST` | `0.0.0.0` | Address to listen on |
| `FORWARD_HTTP_PORT` | `8080` | Port to listen on |
| `FORWARD_HTTP_TLS_CERT` / `FORWARD_HTTP_TLS_KEY` | — | Certificate and key. Required unless insecure mode is on. |
| `FORWARD_HTTP_ALLOW_INSECURE` | `false` | Development only. Allows plain HTTP. |
| `FORWARD_HTTP_AUTH_MODE` | `api-key` | `api-key`, `jwt`, or `none` (development only) |
| `FORWARD_HTTP_API_KEYS` | — | `key1:user1,key2:user2` |
| `FORWARD_HTTP_JWT_ISSUER` | — | Expected `iss` claim |
| `FORWARD_HTTP_JWT_AUDIENCE` | `forward-mcp` | Expected `aud` claim |
| `FORWARD_HTTP_JWT_PUBLIC_KEY_URL` | — | JWKS URL of your identity provider (`https://`) |
| `FORWARD_HTTP_JWKS_CA_CERT` | — | PEM certificate to trust for the JWKS URL (self-signed or internal CA) |
| `FORWARD_HTTP_MAX_CONNECTIONS` | `100` | Open MCP requests and streams; more get `503` with `Retry-After` |
| `FORWARD_HTTP_CORS_ORIGINS` | — | Comma-separated `https://` origins. `*` is rejected. |
| `FORWARD_HTTP_RATE_LIMIT` | `100` | Requests per minute per user |
| `FORWARD_HTTP_READ_TIMEOUT` | `30` | Seconds to read a request |
| `FORWARD_HTTP_WRITE_TIMEOUT` | `30` | Seconds; health probes only |

### Known Limitations of Remote Server Mode

This mode works, but it is a preview. Read these before you deploy it:

1. **The cache and the memory graph are shared between users, by design.** Every user reaches Forward with the server's credentials, so they see the same data. Do not store anything in the memory graph that other users must not see. The default network (`set_default_network`) is per session.
2. **Not built yet:** OpenTelemetry metrics and tracing, and Kubernetes manifests (ADR-2610091600, phases 3–5).

`FORWARD_HTTP_WRITE_TIMEOUT` applies only to `/health` and `/ready`. The MCP streams have no write timeout, so they stay open for as long as the client needs.

## Bloomsearch Capabilities

### Automatic Bloom Filter Generation
- Automatically creates bloom filters for NQE results with >100 items
- Enables fast prefiltering to reduce memory usage and improve search performance
- Persistent storage of bloom indexes for reuse across sessions

### Enhanced Search Performance
- Bloom filters provide O(1) lookup time for membership queries
- Reduces memory footprint by only loading relevant data blocks
- Supports complex search patterns with multiple terms

### Integration with Existing Systems
- Works seamlessly with the semantic cache and memory system
- Automatically uses bloom filters when available for search operations
- Maintains backward compatibility with existing workflows

## Testing

**Quick tests (recommended for development):**
```sh
make test              # Unit tests (fast, no integration)
make test-quick        # Same, no verbose output
```

**Quality gates (run before commits):**
```sh
CGO_ENABLED=1 go build ./...                                     # Build verification
CGO_ENABLED=1 go test -race ./internal/...                       # Race detector
CGO_ENABLED=1 go test -count=1 -skip TestIntegration ./internal/... # All unit tests
hexa analyze . --grade A+                                        # Architecture check
```

**Tool quality verification:**
```sh
./scripts/test-tool-quality.sh         # Static analysis (template compliance, format hints)
go run scripts/test-mcp-tools/main.go  # MCP protocol test (server startup, tool registration)
```

**Integration tests (requires Forward API credentials):**
```sh
make test-integration  # Live API tests (needs .env with FORWARD_API_KEY, etc)
make test-all          # Unit + integration tests
make test-coverage     # Coverage report
```

## Documentation

**Architecture & Design:**
- `docs/adrs/ADR-2610091315-forward-mcp-is-a-hexagon.md` - Hexagonal architecture decision
- `docs/adrs/ADR-2610091555-mcp-tool-quality-standards.md` - Tool quality standards
- `CLAUDE.md` - Tool design standards for contributors
- `CHANGELOG-v4.0.0.md` - Complete v4.0.0 release notes (402 lines)

**Agent Workflows:**
- `.claude/skills/forward-mcp-guide/SKILL.md` - Comprehensive agent workflow guide (439 lines)
  - Discovery workflow (search → run queries)
  - Path search rules
  - Memory system usage
  - Common patterns
  - Error recovery

**Development Guides:**
- `docs/format-hints-guide.md` - When to use format hints (IPs, dates) vs when not to (IDs, strings)
- `docs/tool-quality-test-results.md` - Test suite results and metrics
- `docs/INSTANCE_LOCK_GUIDE.md` - Instance lock protection
- `docs/NEW_API_FUNCTIONS.md` - API client usage

## Contributing

Contributions are welcome! Please open issues or pull requests for bug fixes, features, or documentation improvements.

**Before submitting:**
1. Run all quality gates: `make test && hexa analyze . --grade A+`
2. Follow hexagonal architecture patterns (see `docs/adrs/`)
3. New tools must follow standards in `CLAUDE.md` (template format, format hints, error messages)
4. All tests must pass including race detector: `go test -race ./internal/...`

**Tool Design Checklist:**
- ✅ Description follows "Tool to X. Use when Y. Requires Z. Returns W." template
- ✅ Parameters have explicit "Required" or "Optional"
- ✅ Format hints only for IPs, dates, emails (NOT for IDs)
- ✅ Error messages name problem and suggest fix
- ✅ No emojis, markdown, or verbose formatting 

## AI Attribution

Portions of this project were generated or assisted by AI tools, including OpenAI GPT-4, Cursor, and Claude. All AI-generated content was reviewed and, where necessary, modified by human contributors.
