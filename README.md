# Forward MCP

**Version 4.0.0** • [![Architecture Grade](https://img.shields.io/badge/hexa-A%2B%20100%2F100-brightgreen)](https://github.com/dreambigou/hexa)

Forward MCP is an open-source server that provides a set of tools and APIs for interacting with Forward Networks' platform. It enables automation, analysis, and integration with network data using the Model Context Protocol (MCP).

Built with hexagonal architecture for clean separation of concerns, full test coverage, and easy extensibility.

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
└────────────────┬────────────────────────────┘
                 │ stdio
┌────────────────▼────────────────────────────┐
│      Primary Adapter (MCP Server)           │
│   internal/adapters/primary/mcpserver/      │
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
- `internal/adapters/secondary/forwardapi/client.go` - Forward Networks API client
- `.claude/skills/forward-mcp-guide.md` - Agent workflow guide (439 lines)

**Architecture Validation:**
```sh
hexa analyze . --grade A+
# Result: A+ 100/100, zero violations, zero cycles
```

## Prerequisites
- Go 1.25 or later
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
- `.claude/skills/forward-mcp-guide.md` - Comprehensive agent workflow guide (439 lines)
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
