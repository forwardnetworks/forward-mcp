# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

**Requirements**: Go 1.24+, CGO enabled (required for SQLite)

```bash
# Build the MCP server (CGO_ENABLED=1 required for SQLite)
make build

# Run the server
make run

# Development mode (no build)
make dev

# Build test client
make build-test-client
make run-test-client
```

**Critical**: All builds require `CGO_ENABLED=1` because the project uses SQLite (`github.com/mattn/go-sqlite3`). Cross-compilation requires appropriate CGO setup for the target platform.

## Testing Commands

```bash
# Unit tests only (excludes integration tests - recommended for fast iteration)
make test

# Quick unit tests (no verbose output)
make test-quick

# Integration tests (requires valid Forward API credentials in .env)
make test-integration

# All tests (unit + integration, 120s timeout)
make test-all

# Coverage report (unit tests only)
make test-coverage
```

**Integration Test Warning**: Integration tests make real API calls to Forward Networks. They require:
- Valid `.env` file with `FORWARD_API_KEY`, `FORWARD_API_SECRET`, `FORWARD_API_BASE_URL`
- Can hang/fail if API is slow or credentials are invalid
- Tests use `-skip 'TestIntegration'` pattern to exclude them from unit test runs

## Tool Design Standards (ADR-2610091555)

All MCP tools follow Composio-inspired design standards for optimal agent experience:

### Description Template

**Every tool description follows:**
```
Tool to <what it does>.
Use when <situation>.
<Requirements and constraints>.
Returns <what you get>.
```

**Rules:**
- Maximum 1024 characters (OpenAI limit)
- No emojis or markdown formatting
- State constraints explicitly
- One consistent style across all tools

**Example:**
```
Tool to trace L3/L4 packet paths from source to destination through network devices and links.
Use when troubleshooting connectivity, verifying traffic flow, or analyzing routing decisions.
Requires dst_ip (IP or CIDR); from (device name) or src_ip optional.
For multiple queries use search_paths_bulk.
```

### Parameter Schema Standards

**Required for all parameters:**
- Explicit "Required" or "Optional" status
- Clear default behavior ("defaults to latest")
- Format hints where appropriate (`format=ipv4`, `format=email`)
- Constraint documentation ("at least one of X, Y required")

**Example:**
```go
NetworkID string `json:"network_id" jsonschema:"Network ID. Required."`
SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID. Optional; defaults to latest."`
SrcIP string `json:"src_ip,omitempty" jsonschema:"Source IP address or CIDR.;format=ipv4"`
```

**DO NOT use `format=uuid` for network_id or snapshot_id** — they are plain strings per Forward API spec.

### Error Message Standards

**Every error must:**
1. Name the problem
2. Say what to do next
3. Not expose secrets or internal paths

**Good example:**
```go
return nil, fmt.Errorf("network_id is required. Use list_networks to find available networks")
```

**Bad example:**
```go
return nil, fmt.Errorf("invalid input")  // no action
```

### When Adding New Tools

**Checklist:**
1. ✅ Write description following template
2. ✅ Add explicit Required/Optional to all parameters
3. ✅ Add format hints where appropriate (see docs/format-hints-guide.md)
4. ✅ Document explicit constraints ("at least one of")
5. ✅ Write LLM-friendly error messages
6. ✅ Filter response to task-relevant fields (no debug data)
7. ✅ Test with real agent queries
8. ✅ Add to appropriate category in registerTools()

**Current status:** 54/54 active tools follow standards (100%)

### References

- **ADR-2610091555:** Tool Quality Standards (docs/adrs/)
- **Format Hints Guide:** docs/format-hints-guide.md
- **Forward-MCP Guide Skill:** .claude/skills/forward-mcp-guide/SKILL.md
- **Composio Guide:** https://composio.dev/blog/how-to-build-tools-for-ai-agents-a-field-guide
- **Forward API Spec:** https://docs.fwd.app/latest/api/spec/complete.yaml

## Database & Embedding Management

```bash
# Check database status and metadata coverage
make database-status

# Check embedding cache status
make embedding-cache-info

# Generate keyword-based embeddings (fast, free, offline)
make embedding-generate-keyword

# Generate OpenAI embeddings (requires OPENAI_API_KEY, costs money)
make embedding-generate-openai

# Test semantic search functionality
make test-semantic-search

# Demo smart query discovery
make demo-smart-search

# Clean database (destructive)
make database-clean
```

## High-Level Architecture

Hexagonal (ports and adapters), enforced by `hexa analyze . --grade A+` and `.hexa/ADR-rules.toml` (ADR-2610091315). Dependencies point inward: adapters → ports → domain. **Adapters must import `internal/ports`, never `internal/domain` directly** — the ports re-export the domain types (e.g. `ports.HTTPConfig = domain.HTTPConfig`). Breaking this drops the grade.

```
MCP client
   │ stdio                       │ Streamable HTTP /mcp (legacy SSE /sse)
   ▼                             ▼
adapters/primary/mcpserver   adapters/primary/httpserver   ← tools, prompts, resource; HTTP auth/rate limit/TLS
   │
   ▼
usecases/  (Service: 54 tool handlers, workflows, query loading — no I/O of its own)
   │ calls ports (interfaces)
   ▼
adapters/secondary/  forwardapi · sqlite · semcache · embeddings · queryindex · bloom · envconfig · stderrlog · instancelock
   │
   ▼
Forward Networks API, SQLite files, disk
```

| Layer | Path | Holds |
|-------|------|-------|
| Domain | `internal/domain/` | Pure types (config, Forward API types, cache, memory, query index). No imports. |
| Ports | `internal/ports/` | Interfaces (`ForwardAPI`, `QueryIndex`, `QueryStore`, `MemoryStore`, `Logger`, …) and type aliases of domain types |
| Use cases | `internal/usecases/` | `service.go` wires `Deps`; one file per feature (`nqe.go`, `paths.go`, `memory.go`, `inventory.go`, `prefixes.go`, `bloom.go`, `workflows.go`, `query_loader.go`); arg structs in `tools.go` |
| Primary adapters | `internal/adapters/primary/` | `mcpserver/` registers tools/prompts/resource with the MCP SDK; `httpserver/` serves them over HTTP |
| Secondary adapters | `internal/adapters/secondary/` | See below |
| Composition root | `cmd/server/main.go` | Builds the adapters (`newDeps`) and starts the transports |

### Secondary Adapters

- **forwardapi/** — Forward Networks API client. TLS 1.3, credential zeroing, LLM-friendly HTTP errors.
- **sqlite/** — `nqe_store.go` (query metadata, `~/.forward-mcp/data/nqe_queries.db`), `memory_store.go` (knowledge graph, `memory.db`), `rowquery.go` (read-only SQL over stored NQE results).
- **semcache/** — semantic result cache: embedding similarity (threshold 0.85), LRU/LFU/size/TTL/oldest eviction, gzip, disk overflow.
- **queryindex/** — NQE query library search: BM25 (`bm25.go`), fused with embedding cosine ranking via RRF when embeddings exist (`search.go`).
- **embeddings/** — OpenAI, offline keyword, and mock embedding services.
- **bloom/** — bloom filters for NQE results >100 items, persisted under `data/bloom_indexes/{instance_id}/{entity_id}/`.
- **envconfig/**, **stderrlog/**, **instancelock/** — configuration, logging (stderr only; stdout carries the MCP protocol), single-instance lock.

## Critical Implementation Details

### Storage Architecture
- **In-Memory**: SemanticCache, WorkflowManager sessions, NQEQueryIndex
- **SQLite**: Query metadata (`nqe_queries.db`), Knowledge graph (`memory.db`)
- **File System**: Bloom filter indexes, disk cache overflow
- **Redis**: not implemented (the unused interfaces and config were removed)

### Instance Partitioning Model
Every storage key includes `instance_id = SHA-256(apiBaseURL)[:16]`:
- Allows multiple Forward Networks deployments to coexist
- Isolates cache, database, and memory data by instance
- Uses SHA-256 for cryptographic security (prevents collision attacks)
- Example: `cache:{instance_id}:{query_hash}`

### Smart Caching Strategy (NQEDatabase)
Three-tier fallback for query metadata:
1. **Database** (fastest): SQLite with background refresh
2. **API** (fresh): Live fetch from Forward Networks API with enhanced metadata
3. **Spec file** (fallback): Static `spec/NQELibrary.json` if API unavailable (paths and IDs only, no descriptions)

Background refresh triggers on commit ID changes from API.

### Semantic Search Embeddings
Two embedding providers (configurable via `FORWARD_EMBEDDING_PROVIDER`):
- **keyword**: hand-weighted network keyword list plus SHA-256 hash features, no API required, fast, free
- **openai**: text-embedding-3-small (1536 dims), requires `OPENAI_API_KEY`, better semantic quality

Cache file: `spec/nqe-embeddings.json` (ships empty).

Query search does not depend on embeddings: `SearchQueries` ranks with BM25 (`internal/adapters/secondary/queryindex/bm25.go`) and, when queries carry embeddings, fuses that with cosine ranking via Reciprocal Rank Fusion (`search.go`).

### Bloom Filter Auto-Generation
When NQE query results >100 items:
1. Automatically creates bloom filter
2. Stores persistent index in `data/bloom_indexes/`
3. Future searches use O(1) bloom prefiltering
4. Falls back to full scan if bloom filter unavailable

## Configuration

### Required Environment Variables
```bash
FORWARD_API_KEY=<your-api-key>
FORWARD_API_SECRET=<your-api-secret>
FORWARD_API_BASE_URL=<api-url>
```

### Optional Configuration
```bash
# Network & snapshot defaults
FORWARD_DEFAULT_NETWORK_ID=<network-id>
FORWARD_DEFAULT_SNAPSHOT_ID=<snapshot-id>

# Semantic cache tuning
FORWARD_SEMANTIC_CACHE_ENABLED=true
FORWARD_SEMANTIC_CACHE_MAX_ENTRIES=1000
FORWARD_SEMANTIC_CACHE_TTL_HOURS=24
FORWARD_SEMANTIC_CACHE_SIMILARITY_THRESHOLD=0.85
FORWARD_SEMANTIC_CACHE_MAX_MEMORY_MB=512
FORWARD_SEMANTIC_CACHE_EVICTION_POLICY=lru  # lru|lfu|size|ttl|oldest
FORWARD_SEMANTIC_CACHE_COMPRESS_RESULTS=true

# Embedding provider
FORWARD_EMBEDDING_PROVIDER=keyword  # keyword|openai|mock
OPENAI_API_KEY=<key>  # Only needed if provider=openai

# Bloom filter settings
FORWARD_BLOOM_ENABLED=true
FORWARD_BLOOM_THRESHOLD=100
FORWARD_BLOOM_INDEX_PATH=data/bloom_indexes
```

Multi-tier hierarchy: Environment Variables → `.env` file → Defaults

## Development Workflow

### First-Time Setup
```bash
# Clone and install dependencies
git clone <repo>
cd forward-mcp
make deps

# Configure environment
cp env.example .env
# Edit .env with your Forward API credentials

# Build and run
make build
make run
```

### Common Development Tasks
```bash
# Run unit tests during development (fast)
make test-quick

# Test a specific package
go test -v ./internal/adapters/secondary/semcache -run TestSemanticCache

# Generate embeddings for query search (one-time)
make embedding-generate-keyword

# Check database and embedding status
make database-status
make embedding-cache-info

# Integration testing (requires valid API creds)
make test-integration

# Full test suite with coverage
make test-coverage-all
```

### Testing Individual Functions
```bash
# Test specific function
go test -v ./internal/usecases -run TestFunctionName

# Test with timeout
go test -v -timeout=30s ./internal/usecases -run TestName

# Integration test for specific feature
go test -v ./internal/usecases -run 'TestIntegration.*PathSearch'
```

## Docker Deployment

The image runs remote server mode (`FORWARD_HTTP_ENABLED=true`, port 8080) and needs CGO for SQLite.

```bash
make docker-build
docker run --env-file .env -p 8080:8080 \
  -e FORWARD_HTTP_TLS_CERT=/certs/tls.crt -e FORWARD_HTTP_TLS_KEY=/certs/tls.key \
  -v /path/to/certs:/certs:ro -v forward-mcp-data:/home/app/.forward-mcp \
  forward-mcp
```

Behind a TLS-terminating proxy, set `FORWARD_HTTP_ALLOW_INSECURE=true` instead of the certificate variables.

## Key File Locations

- **Entry Point / composition root**: `cmd/server/main.go`
- **Use cases**: `internal/usecases/service.go` (wiring), feature files beside it
- **Tool arg structs**: `internal/usecases/tools.go`
- **Tool registration and descriptions**: `internal/adapters/primary/mcpserver/server.go`
- **HTTP transport**: `internal/adapters/primary/httpserver/`
- **API Client**: `internal/adapters/secondary/forwardapi/client.go`
- **Configuration**: `internal/adapters/secondary/envconfig/config.go` (types in `internal/domain/config.go`)
- **Databases**: `~/.forward-mcp/data/` (created at runtime)
- **Bloom Indexes**: `data/bloom_indexes/` (created at runtime)
- **Query library (offline fallback)**: `spec/NQELibrary.json`
- **Embedding Cache**: `spec/nqe-embeddings.json`

## Important Go Dependencies

- `github.com/modelcontextprotocol/go-sdk` v1.7.0 - official MCP SDK (protocol 2026-07-28; negotiates down to 2024-11-05)
- `github.com/mattn/go-sqlite3` v1.14.28 - SQLite (requires CGO)
- `github.com/danthegoodman1/bloomsearch` - Bloom filters
- `github.com/joho/godotenv` v1.5.1 - Environment loading
- `golang.org/x/sync` v0.20.0 - Concurrent operations (errgroup for graceful shutdown)

## Security & Compliance Specifications

### Security Best Practices (Implemented)

**Critical Security Requirements:**
1. **Cryptographic Hashing**: Use SHA-256 for all security-sensitive hashing (instance IDs, cache keys)
   - ❌ NEVER use MD5 or SHA-1 (cryptographically broken)
   - ✅ Use `crypto/sha256` with minimum 16-character output

2. **TLS Configuration**: TLS 1.3 minimum with secure cipher suites only
   - ❌ NEVER allow `InsecureSkipVerify` or certificate validation bypass
   - ✅ Enforce `MinVersion: tls.VersionTLS13`
   - ✅ Restrict to: `TLS_AES_128_GCM_SHA256`, `TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`
   - See: `internal/adapters/secondary/forwardapi/client.go` (TLS config)

3. **Path Traversal Protection**: Validate all file paths before filesystem operations
   - ✅ Use `filepath.Clean()` and `filepath.Abs()` for all user-provided paths
   - ✅ Verify paths stay within designated directories using `strings.HasPrefix()`
   - ✅ Validate hash formats with regex before using in file paths
   - See: `internal/adapters/secondary/semcache/semantic_cache.go` (disk path validation)

4. **Concurrency Safety**: Use atomic operations for shared state
   - ✅ Use `sync/atomic` for metrics and counters
   - ✅ Proper mutex locking (RLock for reads, Lock for writes)
   - ✅ Run with `-race` detector during testing
   - See: `internal/adapters/secondary/semcache/semantic_cache.go` (atomic metrics)

5. **Credential Security**: Zero sensitive data from memory after use
   - ✅ Use byte slices with `defer` cleanup for credentials
   - ✅ Explicitly zero memory: `for i := range credentials { credentials[i] = 0 }`
   - See: `internal/adapters/secondary/forwardapi/client.go` (credential zeroing)

6. **SQL Injection Prevention**: Escape LIKE patterns and use parameterized queries
   - ✅ Escape special characters: `\`, `%`, `_` in user input
   - ✅ Use `?` placeholders for all SQL values
   - See: `internal/adapters/secondary/sqlite/memory_store.go` (LIKE escaping)

### MCP Protocol Compliance

**Tool Definition Requirements:**
1. **No Dummy Parameters**: Empty structs are valid - don't add dummy fields
   ```go
   // ✅ Correct
   type ListNetworksArgs struct {
       // No parameters needed - MCP handles empty structs correctly
   }

   // ❌ Wrong
   type ListNetworksArgs struct {
       Dummy string `json:"dummy"`  // Violates MCP spec
   }
   ```

2. **Input Validation**: All tool handlers must validate required inputs
   - ✅ Use validation helpers: `validateNetworkID()`, `validateQueryID()`, etc.
   - ✅ Provide LLM-friendly error messages with actionable guidance
   - Example: `"network_id is required. Use list_networks to find available networks"`

3. **Error Message Standards**: Make errors helpful for LLMs and users
   - ✅ Include context: what failed, why, and how to fix
   - ✅ HTTP errors: explain status code meaning and required action
   - ❌ Don't expose sensitive data (API keys, internal paths, full stack traces)
   - See: `internal/adapters/secondary/forwardapi/client.go` (HTTP error messages)

4. **Session Management**: Automatic cleanup with TTL
   - ✅ WorkflowManager: max 1000 sessions, 24-hour TTL
   - ✅ Cleanup goroutine runs every hour
   - ✅ Graceful shutdown on service termination

5. **Shutdown Behavior**: Concurrent shutdown with timeout enforcement
   - ✅ Use `errgroup.WithContext` for parallel component shutdown
   - ✅ Enforce timeout using context.WithTimeout
   - ✅ Return timeout error if shutdown exceeds duration
   - See: `internal/usecases/service.go` (`Shutdown`)

### Security Testing Requirements

```bash
# Always test for race conditions
go test -race ./internal/...

# Run security scanner
gosec ./...

# Check for known vulnerabilities
govulncheck ./...

# Integration tests with real credentials
make test-integration
```

**Pre-commit Checklist:**
- [ ] No hardcoded credentials or API keys
- [ ] All file paths validated against traversal
- [ ] Race detector passes (`-race`)
- [ ] Input validation on all user-provided data
- [ ] Error messages don't leak sensitive information
- [ ] TLS configuration doesn't allow insecure options

### Instance Partitioning (Updated)

**Hash Generation**: Uses SHA-256 (not MD5) for instance IDs
```go
// internal/usecases/instance.go
hasher := sha256.New()
hasher.Write([]byte(apiBaseURL))
hash := hex.EncodeToString(hasher.Sum(nil))
instanceID := hash[:16]  // 16 chars (was 8 with MD5)
```

Partition key format: `cache:{instance_id}:{resource_hash}`
- Isolates data between different Forward Networks deployments
- Prevents collision attacks (SHA-256 vs legacy MD5)
- Used in: SemanticCache, MemorySystem, BloomIndexManager

## Recent Security & Compliance Improvements

### Summary of Implemented Fixes (January 2026)

All security vulnerabilities and MCP protocol compliance issues have been addressed:

**Critical Security Fixes:**
- ✅ **MD5 → SHA-256**: Replaced cryptographically broken MD5 with SHA-256 for instance ID generation
- ✅ **TLS 1.3 Enforcement**: Removed InsecureSkipVerify, enforced TLS 1.3 with secure cipher suites only
- ✅ **Path Traversal Protection**: Added absolute path validation in semantic cache file operations

**High Priority Security:**
- ✅ **Race Condition Fixes**: Converted cache metrics to atomic operations, proper mutex usage
- ✅ **Credential Zeroing**: Implemented memory zeroing for API credentials after use
- ✅ **SQL Injection Prevention**: Added LIKE pattern escaping for user input in database queries
- ✅ **Path Validation**: Enhanced hash format validation to prevent path injection

**MCP Protocol Compliance:**
- ✅ **Removed Dummy Parameters**: Fixed 9 tool definitions to follow MCP specification
- ✅ **Input Validation**: Added validation helpers with LLM-friendly error messages
- ✅ **Session Cleanup**: Implemented automatic workflow session cleanup with TTL
- ✅ **Error Messages**: Improved HTTP error messages with actionable guidance for LLMs
- ✅ **Shutdown Timeout**: Added concurrent shutdown with timeout enforcement using errgroup

**Build Quality:**
- ✅ **Race Detector Clean**: All tests pass with `-race` flag
- ✅ **Zero Build Errors**: Core codebase builds successfully without warnings

### Security Testing Commands

```bash
# Run with race detector (required before commits)
go test -race ./internal/...

# Security scanning
gosec ./...

# Vulnerability check
govulncheck ./...

# Full test suite with coverage
make test-coverage-all

# Verify build
make build
```

### Development Guidelines

**When Adding New Features:**
1. ✅ Use SHA-256 for any hashing operations (never MD5/SHA-1)
2. ✅ Validate all user input with helpful error messages
3. ✅ Use atomic operations for shared state/metrics
4. ✅ Validate file paths before filesystem access
5. ✅ Test with `-race` detector
6. ✅ Follow MCP protocol spec (no dummy parameters)
7. ✅ Zero sensitive data from memory when done
8. ✅ Use TLS 1.3+ for all external connections

**When Modifying Security-Critical Code:**
- Review the Security & Compliance Specifications section
- Consult existing implementations as reference
- Run security tests before committing
- Document any security-relevant changes in commit messages
