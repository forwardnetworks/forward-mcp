# Forward-MCP Guide

**Purpose:** Teach agents how to use Forward-MCP tools effectively for network analysis.

**When to use:** Load this skill when working with Forward Networks queries, path analysis, or network troubleshooting.

---

## Quick Start

**First time setup:**
1. Set default network: `set_default_network` with network ID or name
2. Initialize query index: `initialize_query_index` (required for semantic search)
3. List available queries: `search_nqe_queries` or `list_nqe_queries`

**Default network eliminates network_id from every tool call.**

---

## Discovery Workflow

### Finding the Right Query

**Three methods:**

1. **Semantic search (recommended):**
   - Tool: `search_nqe_queries`
   - Use: Natural language description
   - Example: "show me BGP routing issues"
   - Returns: Ranked query suggestions with similarity scores

2. **Directory browsing:**
   - Tool: `list_nqe_queries`
   - Use: Filter by directory path
   - Example: directory="/L3/Security/"
   - Returns: Queries in that category

3. **Similar queries:**
   - Tool: `suggest_similar_queries`
   - Use: Find queries related to your intent
   - Returns: Related queries based on semantic similarity

**Then execute:**
- Tool: `run_nqe_query_by_id`
- Requires: query_id (from discovery)
- Optional: all_results=true to fetch complete dataset

---

## Resource Hierarchy

**Forward Networks uses a three-level hierarchy:**

```
Network
  └─ Snapshot (point-in-time network state)
       └─ Query (run against that snapshot)
```

**Tools:**
- `list_networks` → Find available networks
- `list_snapshots` → Find snapshots for a network
- `get_latest_snapshot` → Get current network state
- `run_nqe_query_by_id` → Run query against network/snapshot

**Snapshots:**
- Optional in most tools (defaults to latest)
- Use specific snapshot_id for historical analysis
- Use `get_latest_snapshot` to verify current state

---

## Path Search Rules

### Source Specification

**Three options (at least one required):**

1. **from** (device name)
   - The API uses the device as the packet source
   - Example: from="router-1"

2. **src_ip** (IP address or CIDR)
   - The API resolves the IP to source locations
   - Example: src_ip="10.0.0.1"

3. **Both from + src_ip**
   - Precise packet header specification
   - Device as source + specific source IP

**Do NOT send neither from nor src_ip** — the query will fail or produce unexpected results.

### Destination Specification

**Required: dst_ip**
- Must be IP address or CIDR notation
- Device names are NOT supported
- Example: dst_ip="10.0.0.50"
- Example: dst_ip="192.168.0.0/16"

**Common mistake:** Using device name in dst_ip
- ❌ Wrong: dst_ip="router-2"
- ✅ Correct: dst_ip="10.0.0.2"

### Intent Parameter

**Controls path selection behavior:**

- `PREFER_DELIVERED` (default) - Show paths that successfully deliver packets
- `PREFER_VIOLATIONS` - Show paths with policy violations
- `VIOLATIONS_ONLY` - Show only paths that violate policy

**Use when:**
- Troubleshooting: PREFER_DELIVERED
- Security audit: VIOLATIONS_ONLY
- Compliance review: PREFER_VIOLATIONS

### Performance Tips

- Use `search_paths_bulk` for multiple queries (better performance)
- Set `max_results` to control response size (default handles most cases)
- Set `max_seconds` to timeout long searches
- Large result sets are automatically cached

---

## Memory System Usage

### Entity-Relation-Observation Model

**Three concepts:**

1. **Entity** - A thing (person, network, device, project, concept)
2. **Relation** - How entities connect (owns, manages, depends_on, contains)
3. **Observation** - Timestamped facts about an entity

### Common Patterns

**Store query results:**
```
1. Run query: run_nqe_query_by_id with all_results=true
2. System automatically stores in memory
3. Later: search_entities to find it
4. Later: get_entity to retrieve full results
```

**Build knowledge graph:**
```
1. create_entity for each device/network
2. create_relation to link them
3. add_observation for facts (e.g., "high CPU observed")
4. get_relations to explore connections
```

**Track discoveries:**
```
1. create_entity for investigation (type="investigation")
2. add_observation for each finding
3. create_relation to link to affected devices
4. get_observations to review all findings
```

---

## Common Patterns

### Device Inventory

**Basic information:**
1. `get_device_basic_info` → names, platforms, management IPs
2. Optional: filters and pagination for large networks

**Hardware details:**
1. `get_device_hardware` → models, serial numbers
2. Combine with basic info for complete inventory

**Security/compliance:**
1. `get_hardware_support` → EOL dates, support status
2. `get_os_support` → OS versions, security patches
3. Both are critical for compliance audits

### Configuration Analysis

**Search for patterns:**
1. `search_configs` with hierarchical pattern
2. Pattern format: indented lines, use {name:type} for variables
3. Example pattern:
   ```
   interface
     zone-member security
     ip address {ip:string}
   ```

**Compare changes:**
1. `get_config_diff` with before_snapshot and after_snapshot
2. Shows what changed between two points in time
3. Essential for change tracking and troubleshooting drift

### Connectivity Analysis

**Single path:**
1. `search_paths` with dst_ip (and from or src_ip)
2. Returns trace through network devices and links

**Multiple paths:**
1. `search_paths_bulk` with array of queries
2. Better performance than multiple single searches

**Site-to-site matrix:**
1. `analyze_network_prefixes` with prefix_levels
2. Example: prefix_levels=["/8", "/16", "/24"]
3. Returns connectivity matrices at each aggregation level
4. Use for segmentation validation and multi-site planning

### Large Result Handling

**Automatic chunking:**
- Queries with all_results=true are automatically chunked
- Stored in memory system
- Retrieved with `get_nqe_result_chunks`

**Bloom filters (>100 items):**
1. `build_bloom_filter` creates fast search index
2. `search_bloom_filter` for instant membership testing
3. Use for "contains" checks without loading full dataset

**SQL analysis:**
- `analyze_nqe_result_sql` runs SQL on stored results
- Example: SELECT COUNT(*) FROM nqe_result WHERE column='value'
- Advanced aggregations, filtering, joins

---

## Database & Index Management

### First-Time Setup

**Required once:**
```
1. initialize_query_index (builds search index)
2. Optional: hydrate_database (loads queries from API)
```

**If you see "query index is empty" errors:**
- Run `initialize_query_index`
- If still failing, run `hydrate_database` then retry

### Periodic Maintenance

**When to refresh:**
- After Forward Networks adds new queries
- When semantic search seems stale
- Periodically (monthly) to stay current

**Commands:**
```
1. hydrate_database (loads latest queries from API)
2. refresh_query_index (rebuilds in-memory index)
3. get_database_status (check health)
```

**Refresh is fast (<1s), hydration is slow (minutes).**

---

## Error Recovery

### Common Errors

**"network_id is required"**
- Fix: Set default network with `set_default_network`
- Or: Add network_id parameter to tool call

**"query index is empty"**
- Fix: Run `initialize_query_index`
- If fails: Run `hydrate_database` first

**"snapshot not found"**
- Fix: Use `list_snapshots` to see available snapshots
- Or: Omit snapshot_id to use latest (default)

**Path search returns nothing:**
- Check: dst_ip is an IP address, not device name
- Check: At least one of from or src_ip is provided
- Check: IPs are valid for the network

### Validation Tools

**Before running queries:**
- `get_default_settings` - Check current defaults
- `get_database_status` - Verify database health
- `list_snapshots` - Confirm snapshot exists

**After running queries:**
- `get_cache_stats` - Check if query was cached
- `get_query_analytics` - View query performance

---

## Best Practices

### Always

- Set default network first to simplify all tool calls
- Use semantic search to discover queries (faster than browsing)
- Use all_results=true for complete datasets (automatic chunking)
- Store important results in memory system for later analysis

### Usually

- Use search_paths_bulk for multiple path queries
- Filter results early (use Options parameter with filters)
- Check snapshot_id when comparing historical data
- Review descriptions with list_nqe_queries before running

### Sometimes

- Build bloom filters for very large result sets (>1000 items)
- Use SQL analysis for complex aggregations
- Create knowledge graph for investigations (entities + relations)
- Hydrate database when queries seem out of date

### Never

- Use device names in dst_ip (must be IP address)
- Skip initializing query index before semantic search
- Fetch all results without pagination for exploratory queries
- Assume default snapshot is current (verify with get_latest_snapshot)

---

## Performance Optimization

**Context window management:**
- Use bloom filters for large datasets (keeps results out of context)
- Use chunking for large result sets (fetch one chunk at a time)
- Store in memory system instead of returning huge JSON

**Query performance:**
- Semantic search is cached (subsequent searches are instant)
- NQE queries are cached (repeated queries return immediately)
- Path searches cache by parameters (same search = cache hit)

**Bulk operations:**
- search_paths_bulk for multiple paths
- create_locations_bulk for multiple locations
- update_device_locations for multiple assignments

---

## Troubleshooting Checklist

**Query not finding what I expect:**
1. Check query description: `list_nqe_queries` with filter
2. Try different search terms: `search_nqe_queries` with variations
3. Review query parameters: some queries need parameters

**Path search failing:**
1. Verify dst_ip is IP address (not device name)
2. Verify at least one of from or src_ip is provided
3. Check IPs exist in network: use get_device_basic_info
4. Try simpler query: minimal parameters first

**Semantic search not working:**
1. Check index exists: `get_database_status`
2. Initialize if needed: `initialize_query_index`
3. Refresh if stale: `refresh_query_index`
4. Last resort: `hydrate_database` (slow but thorough)

**Results seem outdated:**
1. Check snapshot: `get_latest_snapshot`
2. Specify snapshot_id explicitly in query
3. Verify network state hasn't changed: `list_snapshots`

---

## Advanced Techniques

### Building a Site Map

```
1. get_device_basic_info → device inventory
2. get_device_locations → device-to-location mappings
3. list_locations → location coordinates
4. analyze_network_prefixes → site-to-site connectivity
5. Store in memory: create_entity for each site, create_relation for connections
```

### Security Posture Assessment

```
1. get_hardware_support → EOL devices
2. get_os_support → vulnerable OS versions
3. search_configs → risky configurations (e.g., default passwords)
4. search_paths with VIOLATIONS_ONLY → policy violations
5. Store findings: create_entity for each issue, add_observation for details
```

### Change Impact Analysis

```
1. get_config_diff → what changed
2. search_paths for affected flows → connectivity impact
3. get_query_analytics → query performance impact
4. Create investigation entity, link all findings with relations
```

### Historical Trend Analysis

```
1. list_snapshots → get snapshot IDs over time
2. run_nqe_query_by_id for each snapshot → time series data
3. Store in memory: create_entity for each snapshot, add_observation for metrics
4. SQL analysis: analyze_nqe_result_sql for aggregations
```

---

## Quick Reference

| Task | Tool | Key Parameters |
|------|------|----------------|
| Find query | search_nqe_queries | query (natural language) |
| Run query | run_nqe_query_by_id | query_id, all_results |
| Trace path | search_paths | dst_ip, from or src_ip |
| Device info | get_device_basic_info | network_id (or default) |
| Config search | search_configs | pattern (hierarchical) |
| Store result | run_nqe_query_by_id | all_results=true (automatic) |
| Find stored | search_entities | query (name pattern) |
| Set default | set_default_network | network_id or name |

---

## References

- Forward API Docs: https://docs.fwd.app/latest/api/
- ADR-2610091555: Tool Quality Standards
- Format Hints Guide: docs/format-hints-guide.md
- Tool Audit: docs/tool-quality-audit.md
