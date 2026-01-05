# Product Requirements Document: Redis Semantic Cache Migration

**Document Version:** 1.0
**Date:** January 5, 2026
**Author:** Engineering Team
**Status:** Draft

---

## Executive Summary

This PRD outlines the migration of the Forward MCP Server's semantic caching system from an in-memory Go implementation to a Redis-based distributed caching solution. This migration will enable horizontal scaling, improve cache performance through native vector search, and provide shared caching across multiple MCP server instances.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Goals & Success Metrics](#2-goals--success-metrics)
3. [User Stories](#3-user-stories)
4. [Functional Requirements](#4-functional-requirements)
5. [Non-Functional Requirements](#5-non-functional-requirements)
6. [Technical Architecture](#6-technical-architecture)
7. [Data Model](#7-data-model)
8. [API Specification](#8-api-specification)
9. [Migration Strategy](#9-migration-strategy)
10. [Security Considerations](#10-security-considerations)
11. [Rollout Plan](#11-rollout-plan)
12. [Dependencies & Risks](#12-dependencies--risks)
13. [Open Questions](#13-open-questions)
14. [Appendix](#14-appendix)
15. [Industry Patterns & Lessons Learned](#15-industry-patterns--lessons-learned)

---

## 1. Problem Statement

### Current State

The Forward MCP Server uses an in-memory semantic cache (`internal/service/semantic_cache.go`) with the following limitations:

| Limitation | Impact |
|------------|--------|
| **Single-instance cache** | Each MCP server maintains its own cache; no sharing between instances |
| **Linear vector search O(n)** | Performance degrades as cache grows; 10K entries = ~10ms search time |
| **Memory bound to process** | Cache limited by Go process heap; competes with application memory |
| **Cold start on restart** | Cache is lost when server restarts; requires re-warming |
| **No horizontal scaling** | Cannot distribute cache load across multiple nodes |
| **Custom eviction logic** | Maintenance burden for LRU/LFU/TTL implementations |

### Business Impact

- **Increased API costs**: Cache misses result in redundant Forward Networks API calls
- **Higher latency**: Linear vector search adds latency for semantic matching
- **Limited scalability**: Cannot scale MCP servers horizontally without cache fragmentation
- **Operational overhead**: Cache warm-up required after each deployment

---

## 2. Goals & Success Metrics

### Primary Goals

| Goal | Description |
|------|-------------|
| **G1** | Enable distributed caching across multiple MCP server instances |
| **G2** | Reduce semantic search latency from O(n) to O(log n) using HNSW |
| **G3** | Eliminate cache cold-start issues with persistent storage |
| **G4** | Reduce Forward Networks API call volume by improving cache hit rate |

### Success Metrics

| Metric | Current Baseline | Target | Measurement Method |
|--------|-----------------|--------|-------------------|
| Cache hit rate | ~40% (estimated) | >70% | Cache metrics endpoint |
| Semantic search latency (p99) | ~50ms at 10K entries | <5ms | Application metrics |
| API cost reduction | Baseline | 50% reduction | Forward API billing |
| Cache sharing efficiency | 0% (no sharing) | 100% | Cross-instance hit rate |
| Cold start time | Full re-warm needed | <1s (Redis connection) | Deployment metrics |

---

## 3. User Stories

### US-1: Shared Cache Across Instances
**As a** DevOps engineer
**I want** multiple MCP server instances to share the same cache
**So that** a cache entry created by one instance benefits all instances

**Acceptance Criteria:**
- Cache entry stored by Instance A is retrievable by Instance B
- Instance partitioning (by Forward Networks deployment) is maintained
- Cache consistency is maintained across instances

### US-2: Fast Semantic Search
**As an** LLM agent user
**I want** semantic cache lookups to be fast regardless of cache size
**So that** I get low-latency responses even with large cache populations

**Acceptance Criteria:**
- Semantic search completes in <10ms for up to 100K entries
- Similarity threshold matching works correctly
- Network/snapshot filtering is applied efficiently

### US-3: Persistent Cache
**As a** system administrator
**I want** the cache to survive server restarts
**So that** users don't experience degraded performance after deployments

**Acceptance Criteria:**
- Cache entries persist across MCP server restarts
- TTL expiration continues to work correctly
- No data loss during graceful shutdown

### US-4: Graceful Fallback
**As a** system operator
**I want** the system to continue working if Redis is unavailable
**So that** Redis failures don't cause complete service outages

**Acceptance Criteria:**
- System falls back to in-memory cache if Redis connection fails
- Automatic reconnection when Redis becomes available
- Clear logging of fallback events

### US-5: Observable Cache Performance
**As a** platform engineer
**I want** detailed metrics about cache performance
**So that** I can monitor and tune the caching system

**Acceptance Criteria:**
- Hit/miss rates exposed via metrics endpoint
- Latency percentiles for cache operations
- Memory usage and entry count metrics
- Eviction counts by policy

---

## 4. Functional Requirements

### FR-1: Cache Operations

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | Store cache entries with query, network_id, snapshot_id, embedding, and result | P0 |
| FR-1.2 | Retrieve cache entries by exact key match | P0 |
| FR-1.3 | Retrieve cache entries by semantic similarity (vector search) | P0 |
| FR-1.4 | Delete cache entries by key | P1 |
| FR-1.5 | Bulk delete cache entries by pattern (e.g., by network_id) | P2 |
| FR-1.6 | List cache entries with pagination | P2 |

### FR-2: Vector Search

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | Support cosine similarity for vector matching | P0 |
| FR-2.2 | Configurable similarity threshold (default: 0.85) | P0 |
| FR-2.3 | Filter vector search by network_id and snapshot_id | P0 |
| FR-2.4 | Return top-K similar results (K configurable) | P1 |
| FR-2.5 | Support HNSW algorithm for approximate nearest neighbor | P0 |

### FR-3: TTL & Eviction

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | Support per-entry TTL with configurable default (24 hours) | P0 |
| FR-3.2 | Automatic expiration of entries past TTL | P0 |
| FR-3.3 | Support memory-based eviction when limit reached | P1 |
| FR-3.4 | Update last-accessed time on cache hits | P0 |

### FR-4: Instance Partitioning

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | Partition cache entries by instance_id (SHA-256 of API base URL) | P0 |
| FR-4.2 | Prevent cross-instance cache pollution | P0 |
| FR-4.3 | Support querying cache statistics per instance | P1 |

### FR-5: Fallback & Resilience

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-5.1 | Fall back to in-memory cache when Redis unavailable | P0 |
| FR-5.2 | Automatic reconnection with exponential backoff | P0 |
| FR-5.3 | Circuit breaker to prevent cascade failures | P1 |
| FR-5.4 | Health check endpoint for Redis connectivity | P1 |

---

## 5. Non-Functional Requirements

### NFR-1: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | Exact match cache lookup latency | p99 < 5ms |
| NFR-1.2 | Semantic search latency (100K entries) | p99 < 10ms |
| NFR-1.3 | Cache write latency | p99 < 10ms |
| NFR-1.4 | Connection pool efficiency | >95% reuse |

### NFR-2: Scalability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-2.1 | Maximum cache entries per instance | 1M entries |
| NFR-2.2 | Maximum embedding dimensions | 3072 (future-proof) |
| NFR-2.3 | Concurrent cache operations | 10K ops/sec |
| NFR-2.4 | Horizontal scaling | Support Redis Cluster |

### NFR-3: Reliability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-3.1 | Cache availability | 99.9% |
| NFR-3.2 | Data durability (with persistence) | 99.99% |
| NFR-3.3 | Graceful degradation on Redis failure | 100% (fallback) |
| NFR-3.4 | Recovery time after Redis restart | < 30s |

### NFR-4: Security

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-4.1 | TLS encryption for Redis connections | Required in production |
| NFR-4.2 | Authentication for Redis access | Required |
| NFR-4.3 | No sensitive data in cache keys | Enforced |
| NFR-4.4 | Audit logging for cache operations | Optional |

### NFR-5: Observability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-5.1 | Metrics export (Prometheus format) | Required |
| NFR-5.2 | Distributed tracing support | Optional |
| NFR-5.3 | Structured logging for cache events | Required |
| NFR-5.4 | Alerting on cache degradation | Required |

---

## 6. Technical Architecture

### 6.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         MCP Clients                                  │
│                    (Claude Desktop, etc.)                            │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Load Balancer                                    │
└─────────────────────────────────────────────────────────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            ▼                   ▼                   ▼
┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
│  MCP Server 1   │   │  MCP Server 2   │   │  MCP Server N   │
│                 │   │                 │   │                 │
│ ┌─────────────┐ │   │ ┌─────────────┐ │   │ ┌─────────────┐ │
│ │CacheInterface│ │   │ │CacheInterface│ │   │ │CacheInterface│ │
│ └──────┬──────┘ │   │ └──────┬──────┘ │   │ └──────┬──────┘ │
│        │        │   │        │        │   │        │        │
│ ┌──────▼──────┐ │   │ ┌──────▼──────┐ │   │ ┌──────▼──────┐ │
│ │In-Memory    │ │   │ │In-Memory    │ │   │ │In-Memory    │ │
│ │(Fallback)   │ │   │ │(Fallback)   │ │   │ │(Fallback)   │ │
│ └─────────────┘ │   │ └─────────────┘ │   │ └─────────────┘ │
└────────┬────────┘   └────────┬────────┘   └────────┬────────┘
         │                     │                     │
         └─────────────────────┼─────────────────────┘
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Redis Cluster                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │
│  │   Primary   │  │   Primary   │  │   Primary   │                  │
│  │   (Shard 1) │  │   (Shard 2) │  │   (Shard N) │                  │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                  │
│         │                │                │                          │
│  ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐                  │
│  │   Replica   │  │   Replica   │  │   Replica   │                  │
│  └─────────────┘  └─────────────┘  └─────────────┘                  │
│                                                                      │
│  Modules: RedisJSON, RediSearch (Vector Search)                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.2 Component Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      ForwardMCPService                               │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                     CacheInterface                              │ │
│  │                                                                 │ │
│  │   Get(query, networkID, snapshotID) → (result, found)          │ │
│  │   Put(query, networkID, snapshotID, result) → error            │ │
│  │   Delete(query, networkID, snapshotID) → error                 │ │
│  │   Clear() → error                                               │ │
│  │   Stats() → CacheStats                                          │ │
│  │   Close() → error                                               │ │
│  │                                                                 │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                          │                                           │
│          ┌───────────────┴───────────────┐                          │
│          ▼                               ▼                          │
│  ┌───────────────────┐         ┌───────────────────┐                │
│  │  SemanticCache    │         │ RedisSemanticCache │                │
│  │  (in-memory)      │         │  (distributed)     │                │
│  │                   │         │                    │                │
│  │ - entries map     │         │ - redis.Client     │                │
│  │ - embeddingIndex  │         │ - vectorIndex      │                │
│  │ - mutex           │         │ - circuitBreaker   │                │
│  │ - metrics         │         │ - metrics          │                │
│  └───────────────────┘         └───────────────────┘                │
│                                         │                            │
│                                         ▼                            │
│                                ┌───────────────────┐                │
│                                │  EmbeddingService  │                │
│                                │                    │                │
│                                │ - keyword (TF-IDF) │                │
│                                │ - openai           │                │
│                                └───────────────────┘                │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.3 Redis Data Structures

| Purpose | Redis Data Structure | Key Pattern |
|---------|---------------------|-------------|
| Cache entries | JSON | `cache:{instance_id}:{entry_hash}` |
| Vector index | RediSearch FT index | `idx:cache:{instance_id}` |
| Metrics | Hash | `metrics:{instance_id}` |
| Locks | String with NX | `lock:{instance_id}:{operation}` |

---

## 7. Data Model

### 7.1 Cache Entry Schema (Redis JSON)

```json
{
  "$schema": "cache_entry_v1",
  "query": "string - original query text",
  "network_id": "string - Forward Networks network ID",
  "snapshot_id": "string - Forward Networks snapshot ID",
  "instance_id": "string - SHA-256 hash of API base URL (16 chars)",
  "embedding": [0.1, 0.2, ...],  // float64 array, 1536 dimensions
  "result": {
    "items": [...],
    "total_count": 100,
    "snapshot_id": "..."
  },
  "created_at": "2026-01-05T12:00:00Z",
  "last_accessed": "2026-01-05T12:30:00Z",
  "access_count": 42,
  "ttl_seconds": 86400,
  "compressed": false,
  "size_bytes": 15360
}
```

### 7.2 Vector Index Schema

```
FT.CREATE idx:cache:{instance_id}
  ON JSON
  PREFIX 1 cache:{instance_id}:
  SCHEMA
    $.embedding AS embedding VECTOR HNSW 6
      TYPE FLOAT64
      DIM 1536
      DISTANCE_METRIC COSINE
      INITIAL_CAP 10000
      M 16
      EF_CONSTRUCTION 200
    $.network_id AS network_id TAG
    $.snapshot_id AS snapshot_id TAG
    $.created_at AS created_at NUMERIC SORTABLE
    $.last_accessed AS last_accessed NUMERIC SORTABLE
    $.access_count AS access_count NUMERIC SORTABLE
```

### 7.3 Metrics Schema (Redis Hash)

```
HSET metrics:{instance_id}
  hit_count 1234
  miss_count 567
  total_queries 1801
  evicted_count 89
  current_entries 945
  memory_usage_bytes 104857600
  avg_response_time_ms 2.5
  last_updated 1704456000
```

---

## 8. API Specification

### 8.1 CacheInterface

```go
// CacheInterface defines the contract for semantic cache implementations
type CacheInterface interface {
    // Get retrieves a cached result by exact match or semantic similarity
    // Returns (result, true) on hit, (nil, false) on miss
    Get(ctx context.Context, query, networkID, snapshotID string) (*forward.NQERunResult, bool)

    // Put stores a query result in the cache
    // Generates embedding automatically if embedding service is available
    Put(ctx context.Context, query, networkID, snapshotID string, result *forward.NQERunResult) error

    // Delete removes a specific cache entry
    Delete(ctx context.Context, query, networkID, snapshotID string) error

    // DeleteByNetwork removes all cache entries for a network
    DeleteByNetwork(ctx context.Context, networkID string) error

    // Clear removes all cache entries for the current instance
    Clear(ctx context.Context) error

    // Stats returns current cache statistics
    Stats(ctx context.Context) (*CacheStats, error)

    // HealthCheck verifies cache backend connectivity
    HealthCheck(ctx context.Context) error

    // Close releases cache resources
    Close() error
}
```

### 8.2 Configuration

```go
// RedisCacheConfig extends SemanticCacheConfig for Redis
type RedisCacheConfig struct {
    // Redis connection
    Enabled     bool   `env:"REDIS_CACHE_ENABLED" default:"false"`
    Address     string `env:"REDIS_ADDRESS" default:"localhost"`
    Port        int    `env:"REDIS_PORT" default:"6379"`
    Password    string `env:"REDIS_PASSWORD"`
    Database    int    `env:"REDIS_DATABASE" default:"0"`

    // TLS
    TLSEnabled  bool   `env:"REDIS_TLS_ENABLED" default:"false"`
    CACertPath  string `env:"REDIS_CA_CERT_PATH"`

    // Connection pool
    PoolSize        int `env:"REDIS_POOL_SIZE" default:"10"`
    MinIdleConns    int `env:"REDIS_MIN_IDLE_CONNS" default:"5"`
    ConnMaxIdleTime int `env:"REDIS_CONN_MAX_IDLE_TIME" default:"300"`
    ConnMaxLifetime int `env:"REDIS_CONN_MAX_LIFETIME" default:"3600"`

    // Vector search
    VectorDimensions int     `env:"REDIS_VECTOR_DIM" default:"1536"`
    HNSWMaxElements  int     `env:"REDIS_HNSW_MAX_ELEMENTS" default:"100000"`
    HNSWM            int     `env:"REDIS_HNSW_M" default:"16"`
    HNSWEfConstruct  int     `env:"REDIS_HNSW_EF_CONSTRUCT" default:"200"`
    HNSWEfRuntime    int     `env:"REDIS_HNSW_EF_RUNTIME" default:"100"`

    // Cache behavior
    SimilarityThreshold float64 `env:"REDIS_SIMILARITY_THRESHOLD" default:"0.85"`
    DefaultTTLHours     int     `env:"REDIS_DEFAULT_TTL_HOURS" default:"24"`
    MaxMemoryMB         int     `env:"REDIS_MAX_MEMORY_MB" default:"512"`

    // Resilience
    EnableFallback      bool `env:"REDIS_ENABLE_FALLBACK" default:"true"`
    CircuitBreakerThreshold int `env:"REDIS_CB_THRESHOLD" default:"5"`
    CircuitBreakerTimeout   int `env:"REDIS_CB_TIMEOUT" default:"30"`
}
```

---

## 9. Migration Strategy

### Phase 1: Interface Extraction (Week 1)

**Objective:** Extract `CacheInterface` without changing behavior

| Task | Description | Risk |
|------|-------------|------|
| 1.1 | Define `CacheInterface` in `internal/service/cache_interface.go` | Low |
| 1.2 | Refactor `SemanticCache` to implement `CacheInterface` | Low |
| 1.3 | Update `ForwardMCPService` to use `CacheInterface` | Low |
| 1.4 | Add context parameter to cache methods | Low |
| 1.5 | Verify all existing tests pass | Low |

### Phase 2: Redis Client Integration (Week 2)

**Objective:** Add Redis client with basic connectivity

| Task | Description | Risk |
|------|-------------|------|
| 2.1 | Add `go-redis/redis/v9` dependency | Low |
| 2.2 | Implement `RedisClient` wrapper with connection pooling | Low |
| 2.3 | Add TLS support for Redis connections | Medium |
| 2.4 | Implement health check and reconnection logic | Medium |
| 2.5 | Add Redis connectivity tests | Low |

### Phase 3: Redis Cache Implementation (Week 3-4)

**Objective:** Implement `RedisSemanticCache`

| Task | Description | Risk |
|------|-------------|------|
| 3.1 | Implement `Put` with JSON storage | Medium |
| 3.2 | Implement exact-match `Get` | Low |
| 3.3 | Create vector index with HNSW | Medium |
| 3.4 | Implement semantic search `Get` | High |
| 3.5 | Implement `Delete` and `Clear` | Low |
| 3.6 | Implement `Stats` aggregation | Low |
| 3.7 | Add comprehensive unit tests | Medium |

### Phase 4: Fallback & Resilience (Week 5)

**Objective:** Implement hybrid caching with graceful degradation

| Task | Description | Risk |
|------|-------------|------|
| 4.1 | Implement `HybridCache` wrapper | Medium |
| 4.2 | Add circuit breaker for Redis failures | Medium |
| 4.3 | Implement automatic fallback to in-memory | Medium |
| 4.4 | Add fallback event logging and metrics | Low |
| 4.5 | Test failure scenarios | High |

### Phase 5: Observability & Metrics (Week 6)

**Objective:** Add production-ready observability

| Task | Description | Risk |
|------|-------------|------|
| 5.1 | Export Prometheus metrics | Low |
| 5.2 | Add distributed tracing spans | Low |
| 5.3 | Create Grafana dashboard | Low |
| 5.4 | Set up alerting rules | Low |
| 5.5 | Document runbooks | Low |

### Phase 6: Rollout (Week 7-8)

**Objective:** Gradual production deployment

| Task | Description | Risk |
|------|-------------|------|
| 6.1 | Deploy Redis infrastructure | Medium |
| 6.2 | Enable Redis cache for 10% traffic (feature flag) | Medium |
| 6.3 | Monitor metrics and validate behavior | Low |
| 6.4 | Increase to 50% traffic | Medium |
| 6.5 | Full rollout (100%) | Medium |
| 6.6 | Deprecate in-memory-only mode | Low |

---

## 10. Security Considerations

### 10.1 Authentication & Authorization

| Requirement | Implementation |
|-------------|----------------|
| Redis authentication | Use `requirepass` or ACL users |
| Credential storage | Environment variables or secrets manager |
| Credential rotation | Support dynamic credential refresh |

### 10.2 Encryption

| Requirement | Implementation |
|-------------|----------------|
| In-transit encryption | TLS 1.3 for all Redis connections |
| Certificate validation | Verify server certificates (no `InsecureSkipVerify`) |
| At-rest encryption | Redis Enterprise encryption or encrypted volumes |

### 10.3 Data Protection

| Requirement | Implementation |
|-------------|----------------|
| No PII in cache keys | Use hashed keys only |
| Instance isolation | Partition by instance_id in all keys |
| Cache entry validation | Validate data before storing |

### 10.4 Network Security

| Requirement | Implementation |
|-------------|----------------|
| Network isolation | Redis in private subnet |
| Firewall rules | Allow only MCP server IPs |
| No public exposure | No public Redis endpoints |

---

## 11. Rollout Plan

### 11.1 Feature Flags

```yaml
feature_flags:
  redis_cache_enabled: false          # Master switch
  redis_cache_percentage: 0           # Percentage of requests using Redis
  redis_fallback_enabled: true        # Enable in-memory fallback
  redis_metrics_enabled: true         # Export Redis-specific metrics
```

### 11.2 Rollout Stages

| Stage | Traffic % | Duration | Success Criteria |
|-------|-----------|----------|------------------|
| Canary | 1% | 24h | No errors, latency within bounds |
| Early Adopters | 10% | 48h | Hit rate > 50%, no degradation |
| Partial | 50% | 72h | Hit rate > 65%, stable metrics |
| Full | 100% | Ongoing | Hit rate > 70%, cost savings validated |

### 11.3 Rollback Triggers

| Trigger | Action |
|---------|--------|
| Redis error rate > 1% | Automatic fallback to in-memory |
| Redis latency p99 > 100ms | Alert + manual review |
| Cache hit rate < 30% | Alert + investigate |
| Memory usage > 90% | Scale Redis or reduce TTL |

---

## 12. Dependencies & Risks

### 12.1 Dependencies

| Dependency | Type | Owner | Status |
|------------|------|-------|--------|
| Redis 7.2+ with RediSearch | Infrastructure | Platform Team | Required |
| go-redis/redis/v9 | Library | Open Source | Available |
| OpenAI embedding API | External Service | OpenAI | Available |
| Prometheus/Grafana | Monitoring | Platform Team | Available |

### 12.2 Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Redis unavailability | Medium | High | Automatic fallback to in-memory cache |
| Vector search accuracy regression | Low | Medium | A/B testing with baseline comparison |
| Increased infrastructure cost | Low | Low | Cost monitoring, TTL tuning |
| Migration data loss | Low | Medium | Dual-write during transition |
| Performance regression | Medium | High | Gradual rollout with monitoring |

---

## 13. Open Questions

| ID | Question | Owner | Status |
|----|----------|-------|--------|
| OQ-1 | Should we use Redis Cloud managed service or self-hosted? | Platform Team | Open |
| OQ-2 | What is the budget for Redis infrastructure? | Product | Open |
| OQ-3 | Should we support Redis Cluster from day 1 or start with single node? | Engineering | Open |
| OQ-4 | How do we handle cache warming for new Redis deployments? | Engineering | Open |
| OQ-5 | Should we implement write-through or write-behind caching? | Engineering | Open |

---

## 14. Appendix

### A. Glossary

| Term | Definition |
|------|------------|
| HNSW | Hierarchical Navigable Small World - algorithm for approximate nearest neighbor search |
| Semantic Cache | Cache that matches queries by meaning rather than exact string match |
| Instance ID | Unique identifier for a Forward Networks deployment (SHA-256 hash of API URL) |
| TTL | Time To Live - duration before cache entry expires |
| Circuit Breaker | Pattern to prevent cascade failures by stopping requests to failing service |

### B. References

- [Redis Vector Search Documentation](https://redis.io/docs/interact/search-and-query/advanced-concepts/vectors/)
- [Redis LangCache](https://redis.io/blog/spring-release-2025/)
- [go-redis Client](https://github.com/redis/go-redis)
- [HNSW Algorithm Paper](https://arxiv.org/abs/1603.09320)

### C. Current Implementation Files

| File | Purpose |
|------|---------|
| `internal/service/semantic_cache.go` | Current in-memory cache implementation |
| `internal/config/config.go` | Configuration including `RedisConfig` |
| `internal/service/mcp_service.go` | MCP service using cache |

---

## 15. Industry Patterns & Lessons Learned

This section documents patterns observed in other MCP servers with semantic search capabilities, informing our design decisions.

### 15.1 Surveyed Implementations

| Project | Vector DB | Caching | Hybrid Search | Key Insight |
|---------|-----------|---------|---------------|-------------|
| [Qdrant MCP Server](https://github.com/qdrant/mcp-server-qdrant) | Qdrant | None | No | Thin wrapper pattern; 2 tools (store/find) |
| [Ultimate MCP Server](https://github.com/Dicklesworthstone/ultimate_mcp_server) | Pluggable | 3-tier | Yes (Marqo) | Multi-level caching most sophisticated |
| [TxtAI Assistant MCP](https://github.com/rmtech1/txtai-assistant-mcp) | TxtAI | Yes | Yes | All-in-one semantic memory |
| [Claude Context (Zilliz)](https://github.com/zilliztech/claude-context) | Milvus | No | Yes (BM25+dense) | 40% token reduction claimed |
| [Document MCP](https://github.com/yairwein/document-mcp) | LanceDB | No | No | Local-first, no cloud dependencies |
| [AWS Bedrock MCP](https://aws.amazon.com/blogs/machine-learning/unlocking-the-power-of-model-context-protocol-mcp-on-aws/) | OpenSearch | Managed | Yes | Enterprise/cloud-native pattern |

### 15.2 Key Lessons Learned

#### Lesson 1: Hybrid Search is Industry Standard

**Observation:** Most production systems combine keyword (BM25) and vector (semantic) search.

**Rationale:**
- Keyword search handles exact matches efficiently
- Semantic search catches conceptual similarity
- Combined approach improves recall without sacrificing precision

**Recommendation for our design:**
```go
func (c *RedisCache) Get(query string) (*Result, bool) {
    // Layer 1: Exact hash match (fastest)
    if result := c.exactMatch(query); result != nil {
        return result, true
    }

    // Layer 2: Keyword/tag search (fast)
    if result := c.keywordSearch(query); result != nil {
        return result, true
    }

    // Layer 3: Semantic vector search (comprehensive)
    return c.vectorSearch(query)
}
```

**Action:** Add hybrid search to Phase 3 of migration plan.

---

#### Lesson 2: Multi-Level Caching Maximizes Hit Rate

**Observation:** Ultimate MCP Server implements 3-tier caching:
1. **Exact match** - Hash-based lookup
2. **Semantic similarity** - Vector distance
3. **Task-aware** - Context-based caching

**Our current design:** 2-tier (exact + semantic)

**Gap:** We lack task-aware caching that considers query context.

**Recommendation:**
```go
type CacheKey struct {
    Query      string  // The NQE query or search
    TaskType   string  // "nqe_query", "path_search", "device_list"
    NetworkID  string
    SnapshotID string
}
```

**Benefit:** Same query text in different task contexts should be cached separately.

---

#### Lesson 3: Thin Wrapper vs. Full-Featured Server

**Observation:** Two distinct patterns emerge:

| Pattern | Example | Complexity | Use Case |
|---------|---------|------------|----------|
| **Thin Wrapper** | Qdrant MCP | Low | Single-purpose vector memory |
| **Full-Featured** | Ultimate MCP | High | Multi-capability server |

**Our position:** Full-featured (47+ tools, caching, memory system, bloom filters)

**Lesson:** Our Redis migration should enhance the existing full-featured architecture, not simplify to a thin wrapper.

---

#### Lesson 4: Embedding Strategy Matters

**Observation:** Different embedding approaches observed:

| Approach | Latency | Cost | Quality |
|----------|---------|------|---------|
| Local (FastEmbed, sentence-transformers) | ~10ms | Free | Good |
| Cloud (OpenAI, Cohere) | ~100ms | $$ | Excellent |
| Hybrid (local + cloud fallback) | Variable | $ | Good+ |

**Our current approach:** Keyword (TF-IDF) or OpenAI

**Recommendation:** Keep hybrid approach but add local embedding option:
```go
type EmbeddingProvider string

const (
    EmbeddingKeyword   EmbeddingProvider = "keyword"    // Free, fast, offline
    EmbeddingLocal     EmbeddingProvider = "local"      // sentence-transformers
    EmbeddingOpenAI    EmbeddingProvider = "openai"     // Best quality
    EmbeddingHybrid    EmbeddingProvider = "hybrid"     // Local + OpenAI fallback
)
```

---

#### Lesson 5: Token Reduction is a Key Metric

**Observation:** Claude Context (Zilliz) measures success by token reduction:
> "~40% token reduction under equivalent retrieval quality"

**Our current metrics:** Hit rate, latency, API cost

**Recommendation:** Add token reduction metric:
```go
type CacheMetrics struct {
    // Existing
    HitRate           float64
    AvgLatencyMs      float64
    APICostSaved      float64

    // New: Token efficiency
    TokensServedFromCache  int64   // Tokens returned from cache hits
    TokensAvoidedByCache   int64   // Estimated tokens saved vs. fresh API call
    TokenReductionPercent  float64 // Overall reduction percentage
}
```

---

#### Lesson 6: Local-First Option Valuable

**Observation:** Document MCP uses LanceDB for fully local operation (no cloud dependencies).

**Our design:** Requires Redis (external dependency)

**Recommendation:** Maintain in-memory fallback as first-class option, not just failure mode:
```yaml
# Configuration options
cache_mode: redis      # Primary: distributed Redis
cache_mode: local      # Alternative: in-memory only (single instance)
cache_mode: hybrid     # Redis with local fallback
```

---

### 15.3 Architectural Decisions Informed by Research

| Decision | Options Considered | Choice | Rationale |
|----------|-------------------|--------|-----------|
| **Vector DB** | Qdrant, Milvus, LanceDB, Redis | Redis | Already planned; provides caching + vectors |
| **Embedding** | Cloud-only, Local-only, Hybrid | Hybrid | Balance cost, latency, quality |
| **Search** | Vector-only, Hybrid | Hybrid | Industry standard; improves precision |
| **Caching tiers** | 2-tier, 3-tier | 3-tier | Add task-aware for better hit rate |
| **Architecture** | Thin wrapper, Full-featured | Full-featured | Matches existing 47+ tool design |
| **Deployment** | Embedded, Separate service | Embedded | Lower latency, simpler deployment |

### 15.4 Updated Feature Priorities

Based on industry analysis, the following features are elevated in priority:

| Feature | Original Priority | New Priority | Justification |
|---------|------------------|--------------|---------------|
| Hybrid search (BM25 + vector) | Not planned | P1 | Industry standard |
| Task-aware caching | Not planned | P2 | Improves hit rate |
| Token reduction metrics | Not planned | P2 | Key success metric |
| Local embedding option | P3 | P2 | Reduces cost/latency |
| Local-first mode | Fallback only | P2 | Valuable for dev/edge |

### 15.5 References

- [Qdrant MCP Server](https://github.com/qdrant/mcp-server-qdrant) - Official Qdrant implementation
- [Ultimate MCP Server](https://github.com/Dicklesworthstone/ultimate_mcp_server) - Multi-level caching patterns
- [TxtAI Assistant MCP](https://github.com/rmtech1/txtai-assistant-mcp) - Semantic memory management
- [Claude Context (Zilliz)](https://github.com/zilliztech/claude-context) - Token reduction benchmarks
- [Milvus MCP Documentation](https://milvus.io/docs/milvus_and_mcp.md) - Vector DB integration
- [AWS Bedrock MCP](https://aws.amazon.com/blogs/machine-learning/unlocking-the-power-of-model-context-protocol-mcp-on-aws/) - Cloud-native patterns
- [Anthropic MCP Documentation](https://docs.anthropic.com/en/docs/build-with-claude/mcp) - Protocol specification

---

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Product Manager | | | |
| Engineering Lead | | | |
| Security Review | | | |
| Platform/Infra | | | |

---

*Document generated: January 5, 2026*
*Last updated: January 5, 2026 - Added Industry Patterns & Lessons Learned*
