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

### MCP Server Structure
```
MCP Client (Claude Desktop)
    ↓ stdio transport
ForwardMCPService (mcp_service.go)
    ├─→ SemanticCache (semantic_cache.go)
    ├─→ MemorySystem (memory_system.go) - Knowledge graph
    ├─→ NQEQueryIndex (nqe_query_index.go) - Vector search
    ├─→ NQEDatabase (nqe_db.go) - SQLite metadata
    ├─→ BloomSearchManager (bloom_search.go) - Large result filtering
    └─→ ForwardClient (internal/forward/client.go)
         ↓
Forward Networks API
```

### Core Components

**ForwardMCPService** (`internal/service/mcp_service.go`):
- Main orchestrator that registers 54 MCP tools, 6 prompts, and 1 resource
- Manages component lifecycle and coordination
- Handles workflow state and session management with automatic cleanup (1000 max sessions, 24h TTL)
- Instance partitioning: Uses SHA-256(apiBaseURL)[:16] as instance_id for multi-tenant data isolation

**SemanticCache** (`internal/service/semantic_cache.go`):
- AI-powered query result caching with embedding-based similarity (threshold: 0.85)
- Multiple eviction policies: LRU, LFU, Size-based, TTL, Oldest
- Compression support (gzip) for large results
- Disk persistence for entries >5% of max memory
- Stores cache with instance_id partitioning

**MemorySystem** (`internal/service/memory_system.go`):
- Knowledge graph using Entity-Relation-Observation (ERO) model
- SQLite backend at `~/.forward-mcp/data/memory.db`
- Full-text search on observations
- Instance partitioning for multi-tenant support

**NQEQueryIndex** (`internal/service/nqe_query_index.go`):
- Vector search for semantic query discovery from 6000+ queries
- Supports keyword embeddings (free, offline) or OpenAI embeddings
- Fast query lookup with cosine similarity matching

**NQEDatabase** (`internal/service/nqe_db.go`):
- SQLite storage at `~/.forward-mcp/data/nqe_queries.db`
- Smart caching strategy: Database → API → Spec file fallback
- Background refresh with commit-based change detection
- Auto-hydration on first run

**BloomSearchManager** (`internal/service/bloom_search.go`):
- Automatic bloom filter generation for NQE results >100 items
- 80%+ memory reduction for large datasets (5000+ items)
- Persistent indexes at `data/bloom_indexes/{instance_id}/{entity_id}/`
- Block-based partitioning (1000 items/block)

## Critical Implementation Details

### Storage Architecture
- **In-Memory**: SemanticCache, WorkflowManager sessions, NQEQueryIndex
- **SQLite**: Query metadata (`nqe_queries.db`), Knowledge graph (`memory.db`)
- **File System**: Bloom filter indexes, disk cache overflow
- **Redis**: Interfaces defined in `redis_store.go` but not implemented (future enhancement)

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
3. **Spec file** (fallback): Static `spec/nqe-queries.json` if API unavailable

Background refresh triggers on commit ID changes from API.

### Semantic Search Embeddings
Two embedding providers (configurable via `FORWARD_EMBEDDING_PROVIDER`):
- **keyword**: TF-IDF based, no API required, fast, free (default)
- **openai**: text-embedding-3-small (1536 dims), requires `OPENAI_API_KEY`, better semantic quality

Cache file: `spec/nqe-embeddings.json`

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
cp .env.example .env
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
go test -v ./internal/service -run TestSemanticCache

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
go test -v ./internal/service -run TestFunctionName

# Test with timeout
go test -v -timeout=30s ./internal/service -run TestName

# Integration test for specific feature
go test -v ./internal/service -run 'TestIntegration.*PathSearch'
```

## Docker Deployment

```bash
# Build and run with Docker
make docker-build
make docker-run

# Or use docker-compose for Claude Flow integration
docker-compose -f docker-compose.optimized.yml up
```

## Key File Locations

- **Entry Point**: `cmd/server/main.go`
- **Core Service**: `internal/service/mcp_service.go` (4422 lines)
- **Tool Definitions**: `internal/service/tools.go`
- **API Client**: `internal/forward/client.go`
- **Configuration**: `internal/config/config.go`
- **Databases**: `~/.forward-mcp/data/` (created at runtime)
- **Bloom Indexes**: `data/bloom_indexes/` (created at runtime)
- **Query Specs**: `spec/nqe-queries.json`
- **Embedding Cache**: `spec/nqe-embeddings.json`

## Important Go Dependencies

- `github.com/modelcontextprotocol/go-sdk` v1.7.0 - official MCP SDK (protocol 2026-07-28 with legacy negotiation)
- `github.com/mattn/go-sqlite3` v1.14.28 - SQLite (requires CGO)
- `github.com/danthegoodman1/bloomsearch` - Bloom filters
- `github.com/joho/godotenv` v1.5.1 - Environment loading
- `golang.org/x/sync` v0.19.0 - Concurrent operations (errgroup for graceful shutdown)

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
   - See: `internal/forward/client.go:84-110`

3. **Path Traversal Protection**: Validate all file paths before filesystem operations
   - ✅ Use `filepath.Clean()` and `filepath.Abs()` for all user-provided paths
   - ✅ Verify paths stay within designated directories using `strings.HasPrefix()`
   - ✅ Validate hash formats with regex before using in file paths
   - See: `internal/service/semantic_cache.go:867-896`

4. **Concurrency Safety**: Use atomic operations for shared state
   - ✅ Use `sync/atomic` for metrics and counters
   - ✅ Proper mutex locking (RLock for reads, Lock for writes)
   - ✅ Run with `-race` detector during testing
   - See: `internal/service/semantic_cache.go:296-447`

5. **Credential Security**: Zero sensitive data from memory after use
   - ✅ Use byte slices with `defer` cleanup for credentials
   - ✅ Explicitly zero memory: `for i := range credentials { credentials[i] = 0 }`
   - See: `internal/forward/client.go:433-447`

6. **SQL Injection Prevention**: Escape LIKE patterns and use parameterized queries
   - ✅ Escape special characters: `\`, `%`, `_` in user input
   - ✅ Use `?` placeholders for all SQL values
   - See: `internal/service/memory_system.go:296-303`

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
   - See: `internal/forward/client.go:470-528`

4. **Session Management**: Automatic cleanup with TTL
   - ✅ WorkflowManager: max 1000 sessions, 24-hour TTL
   - ✅ Cleanup goroutine runs every hour
   - ✅ Graceful shutdown on service termination

5. **Shutdown Behavior**: Concurrent shutdown with timeout enforcement
   - ✅ Use `errgroup.WithContext` for parallel component shutdown
   - ✅ Enforce timeout using context.WithTimeout
   - ✅ Return timeout error if shutdown exceeds duration
   - See: `internal/service/mcp_service.go:333-395`

### Security Testing Requirements

```bash
# Always test for race conditions
go test -race ./internal/service/...

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
// internal/service/instance.go
hasher := sha256.New()
hasher.Write([]byte(apiBaseURL))
hash := hex.EncodeToString(hasher.Sum(nil))
instanceID := hash[:16]  // 16 chars (was 8 with MD5)
```

Partition key format: `cache:{instance_id}:{resource_hash}`
- Isolates data between different Forward Networks deployments
- Prevents collision attacks (SHA-256 vs legacy MD5)
- Used in: SemanticCache, MemorySystem, BloomIndexManager

### Redis Configuration (Available)

Redis support is configured but not actively used. Configuration available:

```bash
# Redis Settings (future enhancement)
REDIS_ENABLED=false
REDIS_ADDRESS=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DATABASE=0
REDIS_POOL_SIZE=10

# Redis TLS
REDIS_TLS_ENABLED=false
REDIS_CA_CERT_PATH=

# Redis Connection Tuning
REDIS_MAX_RETRIES=3
REDIS_MIN_IDLE_CONNS=5
REDIS_CONN_MAX_IDLE_TIME=300  # seconds
REDIS_CONN_MAX_LIFETIME=3600  # seconds
```

Interface defined in `internal/service/redis_store.go` for distributed caching when needed.

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
- ✅ **Redis Configuration**: Added complete RedisConfig struct for future distributed caching
- ✅ **Race Detector Clean**: All tests pass with `-race` flag
- ✅ **Zero Build Errors**: Core codebase builds successfully without warnings

### Files Modified (18 total)

**Security Implementations:**
- `internal/service/instance.go` - SHA-256 hashing
- `internal/config/config.go` - TLS enforcement, RedisConfig
- `internal/forward/client.go` - TLS 1.3, credential zeroing, LLM-friendly errors
- `internal/service/semantic_cache.go` - Path traversal fixes, atomic operations
- `internal/service/memory_system.go` - SQL injection prevention

**MCP Protocol:**
- `internal/service/tools.go` - Removed dummy parameters from 9 structs
- `internal/service/mcp_service.go` - Input validation, session cleanup, shutdown timeout

**Tests Updated:**
- `internal/service/smart_search_test.go`
- `internal/service/mcp_service_test.go`
- `internal/forward/credentials_test.go`

**Supporting Files:**
- `cmd/server/main.go`
- `cmd/test-client/main.go`
- `go.mod` - Added `golang.org/x/sync v0.19.0`

### Security Testing Commands

```bash
# Run with race detector (required before commits)
go test -race ./internal/service/...
go test -race ./internal/forward/...

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
