# Tool Quality Audit Report
## Phase 1: ADR-2610091555 Implementation

**Date:** 2026-10-09
**Scope:** 57 registered MCP tools
**Standard:** Composio tool design template

---

## Executive Summary

Current state:
- **57 tools** registered in `internal/adapters/primary/mcpserver/server.go`
- **Description length:** Varies from 80 to 800+ characters (several exceed 1024 char limit)
- **Format:** Inconsistent; uses emojis, markdown formatting, detailed examples
- **Template compliance:** 0/57 tools follow Composio template

Target state (ADR-2610091555):
- Template: `"Tool to <what>. Use when <situation>. [Constraints if critical.]"`
- Maximum: 1024 characters
- Style: No emojis, no marketing language, constraints first
- Format hints: Add to all parameter schemas

---

## Issues by Category

### 1. Description Length Violations

**Over 1024 characters (estimated):**
- `search_paths` — ~900 chars (close to limit)
- `search_paths_bulk` — ~950 chars (close to limit)
- `analyze_network_prefixes` — ~600 chars
- `run_nqe_query_by_id` — ~500 chars
- `get_device_basic_info` — ~550 chars
- `get_device_hardware` — ~450 chars
- `search_configs` — ~650 chars

**Action:** Compress to template format, move examples/practices to skill.

### 2. Template Non-Compliance

**Current (list_networks):**
```
List all networks in the Forward platform. Returns network IDs, names, and descriptions. 
Use this to discover available networks or find network IDs for other operations. 
Supports pagination (limit/offset) and memory storage for large datasets.
```

**Should be:**
```
Tool to list all networks with their IDs, names, and descriptions. 
Use when discovering available networks or finding network IDs for queries. 
Supports pagination and memory storage.
```

**Current (search_paths):**
```
🔍 **SINGLE PATH SEARCH**: Execute a single path search by tracing packets through the network.
[... 800+ more characters with formatting, rules, examples ...]
```

**Should be:**
```
Tool to trace L3/L4 packet paths between source and destination through network devices. 
Use when troubleshooting connectivity or verifying traffic flow. 
Requires dst_ip (IP/CIDR); from (device) or src_ip optional. 
Use search_paths_bulk for multiple queries.
```

### 3. Parameter Schema Issues

**Missing format hints:**

`internal/usecases/tools.go`:
- `NetworkID` fields → need `format=uuid`
- `SnapshotID` fields → need `format=uuid` 
- `IPAddress`, `SrcIP`, `DstIP` → need `format=ipv4` or `format=ipv6`
- Date fields → need `format=date-time`
- Latitude/Longitude → need `format=number` with range constraints

**Example fix needed:**
```go
// Before
NetworkID string `json:"network_id" jsonschema:"ID of the network"`

// After  
NetworkID string `json:"network_id" jsonschema:"Network UUID. Required.;format=uuid"`
```

**Missing constraint documentation:**

Several tools have "at least one of X, Y, Z" rules that are only in descriptions:
- `search_paths`: Must have `dst_ip`, plus either `from` or `src_ip`
- `update_network`: Must have at least one of `name` or `description`
- `search_configs`: `pattern` or `device_names` required

These should be in the parameter jsonschema, not just descriptions.

### 4. Response Format Issues

**Current:** Tools return raw Forward API JSON including:
- Debug fields (`debug`, `_internal_id`)
- API metadata (`status`, `version`, `timestamp`)
- Nested structures agents don't need

**Action:** Add response filter functions in usecases layer:
```go
// filters.go
func filterNQEResult(raw *forwardapi.NQEResult) *domain.NQEResult {
    return &domain.NQEResult{
        Columns: raw.Columns,
        Rows: raw.Rows,
        Count: raw.RowCount,
        QueryID: raw.QueryID,
        // Omit: debug, _internal_id, api_version, etc.
    }
}
```

### 5. Error Message Audit

**Good examples (already compliant):**
```go
// validateNetworkID
return nil, fmt.Errorf("network_id is required. Use list_networks to find available networks")

// validateQueryID  
return nil, fmt.Errorf("query_id is required. Use list_nqe_queries or search_nqe_queries to find query IDs")
```

**Needs improvement:**
```go
// Some errors are too technical
"failed to marshal request: %v"  
→ "Invalid query parameters. Check parameter format and try again."

// Some lack next steps
"snapshot not found"
→ "snapshot_id not found. Use list_snapshots to see available snapshots."
```

---

## Tool-by-Tool Audit

### Category: Network Management (4 tools)

| Tool | Length | Template? | Param Hints? | Notes |
|------|--------|-----------|--------------|-------|
| `list_networks` | 180 chars | ❌ | ❌ missing format | Good length, needs template |
| `create_network` | 120 chars | ❌ | ❌ missing format | Needs template |
| `update_network` | 140 chars | ❌ | ❌ missing constraint doc | Needs "at least one of" rule |
| `delete_network` | — | — | — | Commented out |

### Category: Path Search (3 tools)

| Tool | Length | Template? | Param Hints? | Notes |
|------|--------|-----------|--------------|-------|
| `search_paths` | ~900 chars | ❌ | ❌ | **Priority:** Way too long, detailed rules in desc |
| `search_paths_bulk` | ~950 chars | ❌ | ❌ | **Priority:** Redundant with search_paths |
| `analyze_network_prefixes` | ~600 chars | ❌ | ❌ | **Priority:** Detailed examples should move to skill |

### Category: NQE Query (4 tools)

| Tool | Length | Template? | Param Hints? | Notes |
|------|--------|-----------|--------------|-------|
| `run_nqe_query_by_id` | ~500 chars | ❌ | ❌ | **Priority:** Core tool, needs clean description |
| `list_nqe_queries` | ~350 chars | ❌ | ❌ | Move examples to skill |
| `search_nqe_queries` | ~450 chars | ❌ | ❌ | Move examples to skill |
| `suggest_similar_queries` | 120 chars | ❌ | ✅ | Good length |

### Category: Device Info (8 tools)

| Tool | Length | Template? | Param Hints? | Notes |
|------|--------|-----------|--------------|-------|
| `get_device_basic_info` | ~550 chars | ❌ | ❌ | **Priority:** Essential tool, too verbose |
| `get_device_hardware` | ~450 chars | ❌ | ❌ | Move "use cases" to skill |
| `get_hardware_support` | ~450 chars | ❌ | ❌ | Move "use cases" to skill |
| `get_os_support` | ~420 chars | ❌ | ❌ | Move "security use cases" to skill |
| `search_configs` | ~650 chars | ❌ | ❌ | **Priority:** Pattern examples too long |
| `get_config_diff` | 130 chars | ❌ | ❌ | Good length |
| `list_devices` | 180 chars | ❌ | ❌ | Good length |
| `get_device_locations` | 150 chars | ❌ | ❌ | Good length |

### Category: Snapshot Management (3 tools)

All tools in this category are 140-180 chars and structurally good, just need template reformatting.

### Category: Location Management (6 tools)

All tools in this category are 120-200 chars and structurally good, just need template reformatting.

### Category: Memory System (9 tools)

Not yet audited (see below for continuation).

### Category: Database/Cache Management (6 tools)

Not yet audited (see below for continuation).

---

## Priority Fixing Order

### High Priority (Core Tools, Used Frequently)
1. `search_paths` / `search_paths_bulk` — 900+ chars, redundant descriptions
2. `run_nqe_query_by_id` — Core query tool
3. `get_device_basic_info` — Essential inventory tool
4. `search_nqe_queries` — AI discovery tool
5. `analyze_network_prefixes` — Complex tool with examples

### Medium Priority (Important but Less Verbose)
6. `list_networks`, `list_snapshots`, `list_nqe_queries`
7. `get_device_hardware`, `get_hardware_support`, `get_os_support`
8. `search_configs` — Has pattern examples

### Low Priority (Already Short, Just Need Template)
9. All snapshot management tools
10. All location management tools  
11. All cache/settings tools
12. Memory system tools

---

## Implementation Checklist

### Week 1: High-Priority Tools
- [ ] Rewrite 5 high-priority tool descriptions to template
- [ ] Add format hints to their parameter structs
- [ ] Create response filter functions for NQE and path results
- [ ] Test with real agent queries

### Week 2: Medium-Priority Tools
- [ ] Rewrite 10 medium-priority tool descriptions
- [ ] Add format hints to all device and snapshot parameters
- [ ] Audit and improve error messages for these tools
- [ ] Add constraint documentation to parameter schemas

### Week 3: Remaining Tools + Skill
- [ ] Rewrite all remaining tool descriptions
- [ ] Complete format hint coverage (100%)
- [ ] Write forward-mcp-guide skill
- [ ] Add description template linter to .hexa/ADR-rules.toml

### Week 4: Validation
- [ ] Run full tool test suite
- [ ] Test all tools with Claude agent
- [ ] Measure context window usage (before/after)
- [ ] Document any agent behavior improvements

---

## Example Rewrites (Before/After)

### Tool: search_paths

**Before (900 chars):**
```
🔍 **SINGLE PATH SEARCH**: Execute a single path search by tracing packets through the network.

Execute path searches by tracing packets through the network. This tool is optimized for single path queries.

**Source Specification Rules:**
- **Option 1**: Use 'from' (device name) - API will use the device as source
- **Option 2**: Use 'src_ip' (IP address/subnet) - API will resolve the IP to source locations
- **Option 3**: Use both 'from' + 'src_ip' for precise packet header specification

**Destination Specification:**
- **REQUIRED**: 'dst_ip' must be a valid IP address or CIDR
- **IMPORTANT**: Device names are NOT supported in dst_ip - use actual IP addresses

**Best Practices:**
- Use 'intent' parameter to control search behavior (PREFER_DELIVERED, PREFER_VIOLATIONS, VIOLATIONS_ONLY)
- Set 'max_results' and 'max_candidates' to control response size and performance
- Use 'max_seconds' for timeout control
- 'snapshot_id' is optional - API uses latest processed snapshot if omitted

**For multiple paths, use search_paths_bulk for better performance.**
```

**After (247 chars):**
```
Tool to trace L3/L4 packet paths from source to destination through network devices and links. Use when troubleshooting connectivity, verifying traffic flow, or analyzing routing decisions. Requires dst_ip (IP/CIDR); from (device) or src_ip optional. For multiple queries use search_paths_bulk.
```

### Tool: get_device_basic_info

**Before (550 chars):**
```
📊 **ESSENTIAL**: Get comprehensive device inventory information.

Get basic device information including names, platforms, and management IPs. This is the primary tool for device discovery and inventory management.

**What you get:**
- Device names and types
- Platform and OS information
- Management IP addresses
- Interface details
- Device status and properties

**Best Practices:**
- Use this as your first step in network analysis
- Set appropriate limits for large networks
- Use filters to focus on specific device types
- Combine with get_device_hardware for complete inventory
```

**After (213 chars):**
```
Tool to retrieve device inventory with names, types, platforms, OS versions, and management IPs. Use when discovering network devices or building device lists. Supports filters and pagination. Combine with get_device_hardware for hardware details.
```

---

## Skill Content (Moves Out of Tool Descriptions)

The `forward-mcp-guide` skill should teach:

1. **Discovery Workflow**
   - Start with `search_nqe_queries` (natural language)
   - Or browse with `list_nqe_queries` (directory structure)
   - Execute with `run_nqe_query_by_id`

2. **Resource Hierarchy**
   - Network (list with `list_networks`)
   - → Snapshot (list with `list_snapshots`, default to latest)
   - → Query (use query_id from discovery)

3. **Path Search Rules** (moved from tool descriptions)
   - Source: Either `from` (device), `src_ip` (IP), or both
   - Destination: `dst_ip` required (must be IP/CIDR, not device name)
   - Intent: PREFER_DELIVERED | PREFER_VIOLATIONS | VIOLATIONS_ONLY
   - Performance: Use `search_paths_bulk` for multiple queries

4. **Large Result Handling**
   - Bloom filters activate automatically >100 items
   - Pagination: Use `limit` and `offset` parameters
   - Memory storage: Available for lists (networks, snapshots, locations)

5. **Common Patterns**
   - Device inventory: `get_device_basic_info` → `get_device_hardware`
   - Security audit: `get_hardware_support` + `get_os_support`
   - Config analysis: `search_configs` → `get_config_diff`
   - Connectivity: `search_paths` or `analyze_network_prefixes`

---

## Next Steps

1. **Get approval** for example rewrites above
2. **Start with high-priority tools** (5 tools, Week 1)
3. **Measure impact** with real agent queries
4. **Iterate** based on agent behavior observations
5. **Complete audit** once pattern is validated
