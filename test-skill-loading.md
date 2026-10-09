# Agent Test: Forward-MCP Skill Usage

This test verifies that the forward-mcp-guide skill provides useful context to agents.

## Test Scenario

An agent needs to find and run Forward Networks queries. Can it use the skill to:
1. Understand the discovery workflow
2. Know which tools to call in what order
3. Handle errors correctly

## Expected Behavior

The agent should:
- Load the skill when working with Forward-MCP tools
- Follow the discovery workflow: search_nqe_queries → run_nqe_query_by_id
- Know that dst_ip is required for path searches
- Understand the Entity-Relation-Observation memory model

## Test Questions

1. **Discovery**: "How do I find the right NQE query to run?"
   - Expected: Mentions search_nqe_queries for semantic search
   - Expected: Mentions list_nqe_queries for directory browsing
   - Expected: Mentions the workflow ends with run_nqe_query_by_id

2. **Path Search**: "How do I trace a packet from device A to IP address 10.0.0.50?"
   - Expected: Knows to use search_paths tool
   - Expected: Knows from="device A" and dst_ip="10.0.0.50"
   - Expected: Knows dst_ip must be an IP, not a device name

3. **Memory System**: "How do I store query results for later?"
   - Expected: Mentions all_results=true for automatic storage
   - Expected: Explains Entity-Relation-Observation model
   - Expected: Mentions search_entities to find stored results

## Manual Test

Run this test with an agent that has access to the skill:

```
You are helping troubleshoot a Forward Networks deployment.
The user asks: "I need to find queries related to BGP routing and run one."

Use the forward-mcp-guide skill to answer:
1. Which tools should I use?
2. In what order?
3. What parameters are required?
```

## Success Criteria

✅ Agent loads the skill without errors
✅ Agent provides accurate workflow (search → run)
✅ Agent mentions required parameters correctly
✅ Agent references specific sections of the skill guide
