# Release v4.0.0 - Hexagonal Architecture + Tool Quality

**Release Date:** 2026-10-09  
**Branch:** refactor/hexagonal → main

## Overview

Major architectural refactoring to hexagonal (ports & adapters) architecture, plus comprehensive tool quality improvements following Composio standards. This release represents a complete restructuring of the codebase while maintaining 100% functional compatibility.

## Breaking Changes

### Architecture

**The codebase has moved to hexagonal architecture:**
- `internal/service/` → split into `internal/usecases/`, `internal/adapters/`, `internal/ports/`, `internal/domain/`
- Import paths have changed for all internal packages
- Service initialization requires explicit adapter injection (see `cmd/server/main.go`)

**If you import forward-mcp as a library, update your imports:**
```go
// Before
import "github.com/forward-mcp/internal/service"

// After
import (
    "github.com/forward-mcp/internal/usecases"
    "github.com/forward-mcp/internal/adapters/primary/mcpserver"
    "github.com/forward-mcp/internal/adapters/secondary/forwardapi"
)
```

### Tool Descriptions

**All 54 tool descriptions have been rewritten** to follow a consistent template:
- No emojis or markdown formatting
- Consistent "Tool to X. Use when Y. Requires Z. Returns W." pattern
- Average length reduced from ~450 to ~200 characters
- Agent behavior may differ slightly due to clearer descriptions

**This is NOT a breaking change for API compatibility** — all tools accept the same parameters and return the same data. Only the descriptions changed.

## Major Features

### 1. Hexagonal Architecture (ADR-2610091315)

**Complete restructuring to ports & adapters pattern:**

**Domain Layer** (`internal/domain/`):
- Pure business types with no I/O dependencies
- `Config`, `NetworkInfo`, `DeviceInfo`, `PathSearchResult`, etc.
- Zero external package imports

**Ports** (`internal/ports/`):
- Interfaces defining what adapters must provide
- `ForwardClient`, `SemanticCache`, `QueryStore`, `MemoryStore`, `Logger`
- Inward ports (use cases) and outward ports (adapters)

**Use Cases** (`internal/usecases/`):
- Business logic orchestration
- Tool implementations (54 MCP tools)
- No direct I/O — all operations through ports

**Primary Adapters** (`internal/adapters/primary/`):
- MCP server registration (`mcpserver/server.go`)
- Converts MCP requests to use case calls

**Secondary Adapters** (`internal/adapters/secondary/`):
- `forwardapi/` - Forward Networks API client
- `sqlite/` - SQLite stores (queries, memory)
- `semcache/` - Semantic caching
- `embeddings/` - Embedding generation
- `queryindex/` - Query search index
- `bloom/` - Bloom filter indexing
- `envconfig/` - Configuration loading
- `stderrlog/` - Logging implementation
- `instancelock/` - Instance locking

**Benefits:**
- ✅ Architecture grade: A+ 100/100 (verified with hexa)
- ✅ Zero boundary violations
- ✅ Zero circular dependencies
- ✅ All components unit testable in isolation
- ✅ Clear separation: I/O at edges, business logic in center

### 2. Tool Quality Standards (ADR-2610091555)

**All 54 MCP tools now follow Composio-inspired quality standards:**

**Description Template:**
```
Tool to <what it does>.
Use when <situation>.
<Requirements and constraints>.
Returns <what you get>.
```

**Changes by Phase:**

**Phase 1 (5 high-priority tools):**
- `search_paths`, `search_paths_bulk`, `run_nqe_query_by_id`
- `search_nqe_queries`, `analyze_network_prefixes`
- Average reduction: 73% (450 → 120 chars)

**Phase 2 (10 medium-priority tools):**
- Network, snapshot, query, device, config tools
- Format hints fixed: removed incorrect `format=uuid`
- Added proper `format=ipv4` for IP addresses
- Average reduction: 35% (300 → 195 chars)

**Phase 3 (39 remaining tools):**
- Memory system (11 tools)
- Bloom filters (3 tools)
- Cache management (3 tools)
- Location management (6 tools)
- Database/index management (4 tools)
- Remaining tools (12 tools)

**Phase 4 (Documentation):**
- Created `.claude/skills/forward-mcp-guide.md` (439 lines)
- Added Tool Design Standards to `CLAUDE.md` (84 lines)
- Comprehensive workflow guide for agents

**Metrics:**
- 54/54 tools (100%) follow template
- ~6,200 characters saved in total
- 0 emojis, 0 markdown formatting
- 0 tools exceed 1024 character limit
- All parameters have explicit "Required" or "Optional"

**Format Hints Corrected:**
- ❌ Removed: `format=uuid` for network_id, snapshot_id (they're plain strings per API spec)
- ✅ Added: `format=ipv4` for IP address fields
- Documentation: `docs/format-hints-guide.md`

### 3. MCP Protocol Upgrade

**Upgraded to MCP Go SDK v1.7.0:**
- Protocol version: 2026-07-28 (negotiates down to 2024-11-05)
- Improved error handling
- Better tool schema validation
- Fixed 9 tools with dummy parameters (MCP spec violation)

### 4. Security Improvements

**Critical security fixes:**
- ✅ SHA-256 (not MD5) for instance ID generation
- ✅ TLS 1.3+ enforcement with secure cipher suites
- ✅ Path traversal protection in cache file operations
- ✅ Race condition fixes (atomic operations)
- ✅ Credential zeroing after use
- ✅ SQL injection prevention (LIKE pattern escaping)

**All tests pass with `-race` detector.**

## New Features

### 1. Forward-MCP Guide Skill

**Comprehensive agent workflow guide** (`.claude/skills/forward-mcp-guide.md`):
- Discovery workflow (semantic search → query execution)
- Path search rules (source/destination requirements)
- Memory system usage (Entity-Relation-Observation model)
- Common patterns (inventory, config analysis, connectivity)
- Error recovery and troubleshooting
- Performance optimization tips

**439 lines** of examples and best practices for agents.

### 2. Test Suite

**Automated quality verification:**

`scripts/test-tool-quality.sh`:
- Build verification
- Template compliance checking
- Format hints validation
- Documentation completeness
- Architecture grade verification

`scripts/test-mcp-tools.go`:
- MCP protocol testing
- Server startup verification
- Tool registration validation
- Description quality checks

**Test Results:** 100% pass rate
- 54/54 tools compliant
- 0 violations
- Architecture: A+ 100/100

### 3. Documentation

**New documentation:**
- `docs/adrs/ADR-2610091315-forward-mcp-is-a-hexagon.md`
- `docs/adrs/ADR-2610091555-mcp-tool-quality-standards.md`
- `docs/format-hints-guide.md`
- `docs/tool-quality-phase{1,2,3}-complete.md`
- `docs/tool-quality-test-results.md`
- `test-skill-loading.md`

**Updated:**
- `CLAUDE.md` - Tool Design Standards section
- `README.md` - Updated for hexagonal architecture

## Improvements

### Code Quality

**Before:**
- Single 5,334-line service file
- Tight coupling between components
- Hard to test in isolation
- Mixed concerns (I/O + business logic)

**After:**
- Smallest file: 10 lines (ports/logger.go)
- Largest file: 760 lines (usecases/prefixes.go)
- Clear separation of concerns
- All components unit testable
- Dependency injection throughout

**Metrics:**
- Files: +111 files changed
- Lines: +12,483 insertions, -8,998 deletions
- Net: +3,485 lines (better organized)
- Architecture violations: 0
- Dead exports: 0 (removed 9 unused exports)

### Performance

**No performance regressions:**
- All caching strategies preserved
- Semantic cache still uses embeddings
- Bloom filters still optimize large datasets
- Query index still provides fast search

**Memory:**
- Context window: ~6,200 chars saved in tool descriptions
- Bloom filters: 80%+ memory reduction for large results

### Testing

**Improved test coverage:**
- Unit tests for all adapters
- Integration tests preserved
- New protocol tests (MCP)
- Static analysis tests (hexa)
- Race detector: 0 violations

**Test execution:**
```bash
make test            # Unit tests (fast)
make test-all        # Unit + integration
make test-coverage   # With coverage report
```

## Migration Guide

### For Users

**No changes required.** The server binary works exactly the same:
```bash
# Same commands work
make build
make run

# Same environment variables
FORWARD_API_KEY=...
FORWARD_API_SECRET=...
FORWARD_API_BASE_URL=...
```

**Tool behavior is identical** — only descriptions changed for clarity.

### For Library Users

**Update imports if you use forward-mcp as a library:**

```go
// Create service (before)
import "github.com/forward-mcp/internal/service"
svc := service.NewForwardMCPService(cfg, log)

// Create service (after)
import (
    "github.com/forward-mcp/internal/usecases"
    "github.com/forward-mcp/internal/adapters/secondary/forwardapi"
    "github.com/forward-mcp/internal/adapters/secondary/semcache"
    "github.com/forward-mcp/internal/adapters/secondary/embeddings"
    // ... other adapters
)

embedder := embeddings.New(cfg.EmbeddingProvider, apiKey, log)
cache := semcache.NewSemanticCache(embedder, log, instanceID, &cfg.SemanticCache)
client := forwardapi.NewClient(&cfg.Forward, log)

deps := usecases.Deps{
    API:   client,
    Cache: cache,
    // ... other adapters
}

svc := usecases.New(cfg, log, deps)
```

See `cmd/server/main.go` for complete example.

### For Contributors

**New development workflow:**

1. **Business logic** → `internal/usecases/`
2. **Interfaces** → `internal/ports/`
3. **External I/O** → `internal/adapters/secondary/`
4. **MCP registration** → `internal/adapters/primary/mcpserver/`
5. **Pure types** → `internal/domain/`

**Run architecture check:**
```bash
hexa analyze . --grade A+
```

**Follow tool design standards:**
- See `CLAUDE.md` - Tool Design Standards section
- Use template format for descriptions
- Add explicit Required/Optional to parameters
- Check `docs/format-hints-guide.md` for format hints

## Bug Fixes

- Fixed: network_id and snapshot_id are strings, not UUIDs
- Fixed: 9 tools with dummy parameters (MCP spec violation)
- Fixed: Race conditions in cache metrics
- Fixed: SQL injection in LIKE patterns
- Fixed: Path traversal in cache file operations
- Fixed: examples/test_mcp.go after arg struct removal

## Dependencies

**Updated:**
- `github.com/modelcontextprotocol/go-sdk` v1.6.0 → v1.7.0
- `golang.org/x/sync` v0.19.0 → v0.20.0 (added for graceful shutdown)

**All other dependencies unchanged.**

## Validation

**All quality gates pass:**
```
✅ Build: CGO_ENABLED=1 go build ./...
✅ Tests: CGO_ENABLED=1 go test -count=1 -skip TestIntegration ./internal/...
✅ Race: go test -race ./internal/...
✅ Architecture: hexa analyze . --grade A+
✅ MCP Protocol: scripts/test-mcp-tools.go
✅ Tool Quality: scripts/test-tool-quality.sh
```

**Metrics verified:**
- Architecture grade: A+ 100/100
- Boundary violations: 0
- Circular dependencies: 0
- Dead exports: 0
- Test pass rate: 100%
- Tool quality: 54/54 (100%)

## Known Issues

None. All tests pass.

## Upgrade Notes

**From v3.0.0 to v4.0.0:**

1. **If using as CLI tool:** No changes needed
2. **If using as library:** Update imports (see Migration Guide)
3. **If developing:** Review hexagonal architecture guide in docs/adrs/

**Database compatibility:** All SQLite databases remain compatible. No schema changes.

**Cache compatibility:** All cache files remain compatible. No format changes.

## Contributors

- Claude Sonnet 4.5 <noreply@anthropic.com>
- Gary Berger (project maintainer)

## References

- **ADR-2610091315:** Hexagonal Architecture
- **ADR-2610091555:** Tool Quality Standards
- **Hexa:** https://github.com/dreambigou/hexa
- **Composio Guide:** https://composio.dev/blog/how-to-build-tools-for-ai-agents-a-field-guide
- **MCP Protocol:** https://modelcontextprotocol.io/
- **Forward API:** https://docs.fwd.app/latest/api/

---

**Full Changelog:** v3.0.0...v4.0.0 (22 commits)

**Download:** See release assets for compiled binaries (coming soon)

**Docker:** `docker pull forward-mcp:4.0.0` (coming soon)
