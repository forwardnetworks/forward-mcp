---
id: ADR-2610091555
status: accepted
date: 2026-10-09
---
# ADR-2610091555: MCP tool quality standards from production agent behavior

**Status:** Proposed
**Date:** 2026-10-09
**Drivers:** Forward-MCP offers 54 MCP tools to Claude and other agents. Tool design—names, descriptions, parameters, errors, and response formats—directly affects agent success rate. Composio published field data showing a 10× reduction in tool failures after applying systematic design rules learned from anonymized production logs. Our tools follow MCP protocol correctly but have not been audited against agent-focused design standards.

## Context

Measured on `refactor/hexagonal` at commit `d1bfe8d`:

| Fact | Value |
|---|---|
| MCP tools registered | 54 |
| Tools with empty parameter structs | 9 (correct per MCP spec) |
| Tool description format | Varies; no template enforced |
| Parameter constraints documented | Some; no systematic audit |
| Error messages | LLM-friendly helpers exist; coverage varies |
| Response format | Raw Forward API JSON |
| Skills shipped | 0 (tools only) |

### What Composio Learned

From Composio's October 2026 retrospective on tool design:

1. **Descriptions are performance levers.** One missing sentence ("if format is json, jsonOptions is required") caused systematic failures. Adding it fixed an entire failure class. Template: "Tool to \<what\>. Use when \<situation\>." Keep under 1024 characters. State constraints first.

2. **Hidden parameter rules kill agents.** Document "at least one of X, Y, Z required" explicitly. Use schema enums for finite sets, not prose lists. Add `format` hints (`email`, `ipv4`, `uri`).

3. **Errors as instructions.** A good error tells the agent what to do next. Example: `"network_id is required. Use list_networks to find available networks"` (we already do this in `validateNetworkID`).

4. **Raw API responses hurt quality.** Agents get fields they don't need and miss fields they do. Filter responses to task-relevant data.

5. **Skills teach method; tools do work.** Composio ships a skill that explains *how* to use Composio (discovery workflow, auth, when to use which tool). Separate from tool definitions.

6. **Context is a budget.** Load tools just-in-time (we do this for NQE queries via semantic search). Composio's tool router searches 1,500 apps but loads only matches.

### What We Already Do Well

- **Atomic scope**: One tool = one action (`list_networks` does not create; `search_paths` does not analyze prefixes)
- **Context management**: Semantic search returns top NQE matches from 6,000+; bloom filters keep large results out of context
- **Cancellation**: Context flows end-to-end after hexagonal refactor; agents can cancel without wasting API quota
- **Validation helpers**: `validateNetworkID`, `validateQueryID` return actionable errors

### What Needs Work

1. **Tool descriptions**: No enforced template; quality varies
2. **Parameter docs**: Some constraints live in comments, not jsonschema; missing `format` hints
3. **Response format**: Returns raw Forward API JSON with debug fields, internal IDs, and API metadata agents don't need
4. **Skills**: No skill explaining the Forward-MCP workflow (network → snapshot → query; semantic search vs. direct ID; when to use memory storage)
5. **Error coverage audit**: Not all validation paths return LLM-friendly errors

## Decision

Adopt Composio's tool design standards and apply them to Forward-MCP's 54 tools:

### 1. Tool Description Template (Enforced)

Every tool description follows:

```
Tool to <what it does>. Use when <situation>. [Constraints if critical.]
```

**Rules:**
- Maximum 1024 characters (OpenAI's cap; other providers similar)
- State critical constraints first (e.g., "Requires network_id")
- One description style across all tools
- No marketing language; pure function

**Example (before):**
```
Searches network paths
```

**Example (after):**
```
Tool to find all possible L3/L4 paths between source and destination in the network.
Use when troubleshooting connectivity, verifying traffic flow, or analyzing path diversity.
Requires network_id; snapshot_id optional (defaults to latest).
```

**Enforcement:** Linter rule in `.hexa/ADR-rules.toml` checks tool registration calls for template compliance.

### 2. Parameter Schema Standards

**Rules:**
- Document "at least one of X, Y, Z" explicitly in the parameter description
- Use jsonschema `enum` for finite sets (not prose)
- Add `format` hints: `"format": "email"`, `"format": "ipv4"`, `"format": "uri"`, `"format": "date-time"`
- Embed tiny examples in descriptions for complex formats (e.g., NQE query syntax)

**Example (before):**
```go
SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID"`
```

**Example (after):**
```go
SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID (UUID format). Optional; defaults to latest snapshot for the network.;format=uuid"`
```

**Action:** Audit all 54 tools' parameter structs; add missing `format` and constraint docs.

### 3. Response Format Optimization

**Rule:** Return only task-relevant fields; filter out debug data, internal IDs, and API metadata.

**Before (raw Forward API):**
```json
{
  "id": "internal-query-id",
  "status": "completed",
  "debug": {...},
  "columns": ["name", "platform"],
  "rows": [[...]]
}
```

**After (filtered):**
```json
{
  "columns": ["name", "platform"],
  "rows": [[...]],
  "count": 42,
  "query_id": "devices"
}
```

**Implementation:** Add response filter functions in `usecases/` that wrap Forward API results.

### 4. Error Message Audit

**Standard:** Every error must:
1. Name the problem
2. Say what to do next
3. Not expose secrets, internal paths, or stack traces

**Good example (current code):**
```
"network_id is required. Use list_networks to find available networks"
```

**Bad example (hypothetical):**
```
"invalid input"  // no action
"API key invalid"  // exposes auth details
```

**Action:** Audit all error returns in `usecases/`; rewrite any that fail the three-part test.

### 5. Forward-MCP Skill

Create `forward-mcp-guide` skill that teaches agents:
- **Discovery workflow**: Semantic search (`search_nqe_queries`) vs. direct ID (`run_nqe_query_by_id`)
- **Resource hierarchy**: Network → Snapshot → Query
- **Large result handling**: When bloom filters activate; pagination strategy
- **Memory system**: When to store results vs. return them
- **Common patterns**: Device inventory, path analysis, config diff workflow

**Format:** Markdown skill file in `.claude/skills/` (if using Claude Code) or shipped as MCP prompt template.

**Not a tool:** Skills explain *how*; tools *do*. The skill loads once per session; tools execute per-call.

### 6. Optional: CLI Alongside MCP

**Decision deferred.** CLI would be another primary adapter (`internal/adapters/primary/cli`). Business logic stays in `usecases/`. This ADR does not mandate it, but the hexagonal structure allows it if needed.

### Enforcement

Rules that carry the decision:

1. **Tool descriptions must match the template.** Linter checks during `hexa analyze`.
2. **Parameters must document constraints and use `format` hints.** Manual audit, then enforced by tests.
3. **Responses must be filtered.** Each tool handler calls a filter function before returning.
4. **Errors must pass the three-part test.** Covered by unit tests.
5. **Skill ships with the MCP server.** Registered as MCP prompt or skill file.

## Consequences

### Positive

- **Higher agent success rate.** Composio saw 10× fewer failures after applying these rules.
- **Less user intervention.** Agents recover from errors without human help.
- **Consistent UX.** All tools follow the same description and error pattern.
- **Easier onboarding.** New agents learn the workflow from the skill.
- **Future-proof.** Standards apply whether the interface is MCP, CLI, or SDK.

### Negative

- **Upfront work.** Auditing 54 tools and rewriting descriptions takes time.
- **Description length limits.** 1024 characters is tight for complex tools; may require splitting.
- **Response filtering adds code.** Each tool needs a filter function (but keeps agents' context clean).

### Neutral

- **Skills are not tools.** Some users expect tools to be self-documenting. The skill is a separate layer.
- **Enforcement requires discipline.** Linter catches descriptions; parameter audit is manual until automated.

## Alternatives Considered

### 1. Keep Current Descriptions

**Rejected.** Production data from Composio shows descriptions directly affect failure rates. "If it works, it's good enough" ignores the agent's experience.

### 2. Generate Descriptions from Code Comments

**Rejected.** Comments drift from code. Descriptions are part of the API contract and must be explicit.

### 3. Return Raw API Responses

**Rejected.** Composio's data shows raw responses "quietly hurt agent quality." Extra fields confuse; missing fields force retry loops.

### 4. Make Skills Optional

**Rejected.** The workflow (network → snapshot → query; semantic vs. direct) is not obvious. Agents that skip the skill will make systematic mistakes.

## Migration Path

**Phase 1: Audit and Document (1 week)**
1. Audit all 54 tool descriptions; rewrite to template
2. Audit parameter constraints; add missing `format` and docs
3. Audit error messages; rewrite failures

**Phase 2: Response Filters (1 week)**
1. Write filter functions for common Forward API responses
2. Update tool handlers to call filters
3. Test with real agent queries

**Phase 3: Skill and Enforcement (1 week)**
1. Write `forward-mcp-guide` skill
2. Register as MCP prompt
3. Add linter rule for description template
4. Document standards in CLAUDE.md

**Phase 4: Production Validation (ongoing)**
1. Monitor anonymized tool-call logs (if available)
2. Classify failures: invocation confusion, parameter errors, response parsing
3. Iterate on descriptions and filters

## Gate

The refactoring is complete when:

```bash
# All tests pass
make test

# Architecture grade remains A+
hexa analyze . --grade A+

# Description linter passes
hexa analyze . --violations-only --exit-code

# Manual checks:
# - All 54 tool descriptions match template
# - All parameters have format hints where applicable
# - All error paths return actionable messages
# - forward-mcp-guide skill is registered
```

## References

- Composio: How to Build Great Tools for AI Agents (Sep 2025) — https://composio.dev/blog/how-to-build-tools-for-ai-agents-a-field-guide
- Composio: Universal CLI (Mar 2026) — https://composio.dev/blog/announcing-universal-cli-by-composio
- Composio: Claude Skills Guide — https://composio.dev/content/claude-skills-and-ai-agents-guide
- MCP Protocol Specification — https://spec.modelcontextprotocol.io/
- ADR-2610091315: forward-mcp is a hexagon
