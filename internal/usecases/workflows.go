package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/forward-mcp/internal/domain"
)

// NqeQueryDiscoveryWorkflow implements the NQE query discovery workflow
func (s *Service) NqeQueryDiscoveryWorkflow(ctx context.Context, args NQEDiscoveryArgs) (*Result, error) {
	sessionID := fmt.Sprintf("session_%v", args.SessionID) // In practice, extract from context
	state := s.workflowManager.GetState(sessionID)

	switch state.CurrentStep {
	case "start":
		return s.startQueryDiscovery(sessionID)
	case "category_selected":
		return s.listQueriesInCategory(ctx, sessionID, state.Parameters["directory"].(string))
	case "query_selected":
		return s.collectQueryParameters(sessionID)
	case "parameters_collected":
		return s.executeSelectedQuery(ctx, sessionID)
	default:
		return s.startQueryDiscovery(sessionID)
	}
}

// NetworkDiscoveryWorkflow implements the network discovery workflow
func (s *Service) NetworkDiscoveryWorkflow(ctx context.Context, args NetworkDiscoveryArgs) (*Result, error) {
	networks, err := s.forwardClient.GetNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	promptText := "Available networks:\n"
	for i, network := range networks {
		promptText += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, network.Name, network.ID)
	}
	promptText += "\nWhat would you like to do?\n1. Select a network to explore\n2. Create a new network\n3. Search for specific devices"

	return textResult(promptText), nil
}

// LargeNQEResultsWorkflow implements the large NQE results workflow
func (s *Service) LargeNQEResultsWorkflow(args LargeNQEResultsWorkflowArgs) (*Result, error) {
	sessionID := fmt.Sprintf("session_%v", args.SessionID)
	state := s.workflowManager.GetState(sessionID)

	switch state.CurrentStep {
	case "start":
		return s.startLargeResultsWorkflow(sessionID)
	case "explain_process":
		return s.explainLargeResultsProcess(sessionID)
	case "show_example":
		return s.showLargeResultsExample(sessionID)
	case "demonstrate_sql":
		return s.demonstrateSQLAnalysis(sessionID)
	default:
		return s.startLargeResultsWorkflow(sessionID)
	}
}

// startLargeResultsWorkflow begins the large NQE results workflow
func (s *Service) startLargeResultsWorkflow(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "explain_process",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	promptText := `🔍 **Large NQE Results Workflow Guide**

Welcome! This workflow teaches you how to handle large NQE query results efficiently using our memory system and SQL analysis capabilities.

**What you'll learn:**
1. How large results are automatically stored in chunks
2. How to retrieve and analyze stored results
3. How to use SQL queries for complex data analysis
4. Best practices for working with large datasets

**Key Concepts:**
- **Chunking**: Large results are split into 200-row chunks for LLM-friendly processing
- **Memory System**: Results are stored persistently with metadata and summaries
- **SQL Analysis**: Full SQL query capabilities on stored data
- **Entity Management**: Each result gets a unique entity ID for easy reference

Would you like to:
1. Learn about the process step-by-step
2. See a practical example
3. Try SQL analysis on existing data
4. Get best practices and tips

Which would you prefer?`

	return textResult(promptText), nil
}

// explainLargeResultsProcess explains the large results workflow process
func (s *Service) explainLargeResultsProcess(sessionID string) (*Result, error) {
	state := s.workflowManager.GetState(sessionID)
	state.CurrentStep = "show_example"
	s.workflowManager.SetState(sessionID, state)

	promptText := `📋 **Large NQE Results Process Explained**

**Step 1: Automatic Detection & Storage**
When you run an NQE query with "all_results: true" or when results exceed size limits:
- System automatically detects large result sets
- Results are fetched in batches using pagination
- Data is stored in the memory system with chunking (200 rows per chunk)
- Each result gets a unique entity ID for easy reference

**Step 2: Memory System Storage**
- **Entity Creation**: Creates a result entity with metadata (query_id, network_id, snapshot_id, row_count)
- **Chunking**: Splits data into manageable chunks stored as observations
- **Summary**: Generates a summary observation with columns, row count, and metadata
- **Persistence**: All data is stored in SQLite database for later retrieval

**Step 3: Analysis Tools Available**
- **get_nqe_result_summary**: View metadata and structure of stored results
- **get_nqe_result_chunks**: Retrieve raw data chunks (all or specific chunk)
- **analyze_nqe_result_sql**: Run SQL queries on the complete dataset

**Step 4: SQL Analysis Workflow**
- Retrieve all chunks for an entity
- Reconstruct complete dataset in memory
- Create temporary SQLite database with the data
- Execute your SQL queries
- Return formatted results

**Benefits:**
✅ **No API Limits**: Work with unlimited data sizes
✅ **Persistent Storage**: Results remain available across sessions
✅ **SQL Power**: Full SQL query capabilities for complex analysis
✅ **LLM Friendly**: Chunked data is easier for LLMs to process
✅ **Performance**: Avoid re-running expensive queries

Would you like to see a practical example of this workflow?`

	return textResult(promptText), nil
}

// showLargeResultsExample shows a practical example
func (s *Service) showLargeResultsExample(sessionID string) (*Result, error) {
	state := s.workflowManager.GetState(sessionID)
	state.CurrentStep = "demonstrate_sql"
	s.workflowManager.SetState(sessionID, state)

	promptText := `💡 **Practical Example: Device Inventory Analysis**

**Scenario**: You want to analyze all devices in your network, but the result is too large for direct API response.

**Step 1: Run Query with Large Results**
{
  "tool": "run_nqe_query_by_id",
  "arguments": {
    "query_id": "device_basic_info",
    "network_id": "your_network_id",
    "all_results": true
  }
}

**Step 2: System Response**
Fetched all results in batches.
Total items: 1,247
Columns: [device_name, platform, ip_address, status, location]
Preview (first 5 rows): [...]
Stored in memory system as entity: device_basic_info-your_network_id-latest
You can use get_nqe_result_summary to analyze this result locally.

**Step 3: Get Result Summary**
{
  "tool": "get_nqe_result_summary",
  "arguments": {
    "entity_id": "device_basic_info-your_network_id-latest"
  }
}

**Step 4: SQL Analysis Examples**
{
  "tool": "analyze_nqe_result_sql",
  "arguments": {
    "entity_id": "device_basic_info-your_network_id-latest",
    "sql_query": "SELECT platform, COUNT(*) as count FROM nqe_result GROUP BY platform ORDER BY count DESC"
  }
}

**Common SQL Queries:**
- "SELECT COUNT(*) FROM nqe_result" - Total devices
- "SELECT status, COUNT(*) FROM nqe_result GROUP BY status" - Status breakdown
- "SELECT * FROM nqe_result WHERE status = 'down'" - Down devices
- "SELECT platform, AVG(CAST(ip_address AS INTEGER)) FROM nqe_result GROUP BY platform" - Platform analysis

Would you like to try SQL analysis on some existing data?`

	return textResult(promptText), nil
}

// demonstrateSQLAnalysis demonstrates SQL analysis capabilities
func (s *Service) demonstrateSQLAnalysis(sessionID string) (*Result, error) {
	state := s.workflowManager.GetState(sessionID)
	state.CurrentStep = "start"
	s.workflowManager.SetState(sessionID, state)

	promptText := `🚀 **SQL Analysis Capabilities**

**Available SQL Features:**
- **Full SQLite Support**: All standard SQL operations
- **Aggregation**: COUNT, SUM, AVG, MIN, MAX, GROUP BY
- **Filtering**: WHERE clauses with complex conditions
- **Sorting**: ORDER BY with multiple columns
- **Joins**: Self-joins within the same dataset
- **Subqueries**: Nested queries for complex analysis
- **Functions**: String, numeric, and date functions

**Best Practices:**
1. **Always use LIMIT** for large result sets (system adds LIMIT 100 by default)
2. **Use GROUP BY** for aggregations and summaries
3. **Leverage WHERE** for filtering before aggregation
4. **Consider data types** - all columns are stored as TEXT initially
5. **Use CAST()** for numeric operations on text columns

**Example Workflows:**
- **Compliance Audit**: Count devices by platform, status, location
- **Performance Analysis**: Find devices with specific configurations
- **Security Assessment**: Identify devices with open ports or weak policies
- **Capacity Planning**: Analyze resource utilization patterns

**Next Steps:**
1. Run a query with "all_results: true" to get a large dataset
2. Use "get_nqe_result_summary" to understand the data structure
3. Write SQL queries to analyze the data
4. Use the results for reports, dashboards, or further analysis

**Pro Tips:**
- Store frequently used queries as entities for quick access
- Use the memory system to track analysis results over time
- Combine multiple query results for comprehensive analysis
- Export SQL results for external reporting tools

Ready to try this workflow with your own data? Start by running a query with "all_results: true"!`

	return textResult(promptText), nil
}

// GetNetworkContext provides contextual network information as a resource
func (s *Service) GetNetworkContext(ctx context.Context, args NetworkContextArgs) (interface{}, error) {
	networks, err := s.forwardClient.GetNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network context: %w", err)
	}

	context := map[string]interface{}{
		"networks":          networks,
		"timestamp":         "current",
		"available_queries": []string{"/L3/Basic/", "/L3/Advanced/", "/L3/Security/"},
	}

	contextJSON, _ := json.MarshalIndent(context, "", "  ")
	return string(contextJSON), nil
}

// startQueryDiscovery begins the NQE query discovery workflow
func (s *Service) startQueryDiscovery(sessionID string) (*Result, error) {
	state := &WorkflowState{
		CurrentStep: "category_selection",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	promptText := "Welcome to NQE Query Discovery!\n\nSelect a query category:\n1. Basic (/L3/Basic/) - Device inventory, basic connectivity\n2. Advanced (/L3/Advanced/) - Complex routing, performance analysis\n3. Security (/L3/Security/) - Security policies, compliance\n\nWhich category interests you?"
	return textResult(promptText), nil
}

// listQueriesInCategory lists available queries in the selected category
func (s *Service) listQueriesInCategory(ctx context.Context, sessionID, directory string) (*Result, error) {
	queries, err := s.forwardClient.GetNQEQueries(ctx, directory)
	if err != nil {
		return nil, fmt.Errorf("failed to get queries: %w", err)
	}

	state := s.workflowManager.GetState(sessionID)
	state.CurrentStep = "query_selection"
	state.Parameters["directory"] = directory
	s.workflowManager.SetState(sessionID, state)

	promptText := fmt.Sprintf("Available queries in %s:\n", directory)
	for i, query := range queries {
		promptText += fmt.Sprintf("%d. %s (ID: %s)\n   Purpose: %s\n", i+1, query.Path, query.QueryID, query.Intent)
	}
	promptText += "\nWhich query would you like to run?"

	return textResult(promptText), nil
}

// collectQueryParameters collects parameters needed for the selected query
func (s *Service) collectQueryParameters(sessionID string) (*Result, error) {
	state := s.workflowManager.GetState(sessionID)

	// Check if we have network_id
	if _, exists := state.Parameters["network_id"]; !exists {
		return textResult("Missing required parameter: network_id"), nil
	}

	// Check if we have snapshot_id
	if _, exists := state.Parameters["snapshot_id"]; !exists {
		return textResult("Missing required parameter: snapshot_id"), nil
	}

	// All parameters collected, ready to execute
	state.CurrentStep = "ready_to_execute"
	s.workflowManager.SetState(sessionID, state)

	return textResult("All parameters collected! Ready to execute query. Proceed?"), nil
}

// executeSelectedQuery executes the query with collected parameters
// This function is part of the workflow system that is now activated via MCP prompt registration
func (s *Service) executeSelectedQuery(ctx context.Context, sessionID string) (*Result, error) {
	state := s.workflowManager.GetState(sessionID)

	params := &domain.NQEQueryParams{
		NetworkID:  state.NetworkID,
		QueryID:    state.SelectedQuery,
		SnapshotID: state.SnapshotID,
	}

	result, err := s.forwardClient.RunNQEQueryByID(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	promptText := fmt.Sprintf("Query executed successfully! Found %d results:\n%s\n\nWhat would you like to do next?\n1. Export results\n2. Run another query\n3. Get more details\n4. Exit", len(result.Items), string(resultJSON))

	return textResult(promptText), nil
}
