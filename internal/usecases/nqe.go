package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/forward-mcp/internal/domain"
)

// Helper function to convert service NQEQueryOptions to forward NQEQueryOptions
func (s *Service) convertNQEQueryOptions(options *NQEQueryOptions) *domain.NQEQueryOptions {
	if options == nil {
		return nil
	}

	// Apply default limit if not specified
	limit := options.Limit
	if limit == 0 {
		limit = s.getQueryLimit(0)
	}

	forwardOptions := &domain.NQEQueryOptions{
		Limit:  limit,
		Offset: options.Offset,
		Format: options.Format,
	}

	// The API accepts a single sort order; use the first entry if provided
	if len(options.SortBy) > 0 {
		forwardOptions.SortBy = &domain.NQESortBy{
			ColumnName: options.SortBy[0].ColumnName,
			Order:      options.SortBy[0].Order,
		}
		if len(options.SortBy) > 1 {
			s.logger.Warn("NQE API supports a single sort column; ignoring %d additional sort criteria", len(options.SortBy)-1)
		}
	}

	if options.Filters != nil {
		forwardOptions.Filters = make([]domain.NQEColumnFilter, len(options.Filters))
		for i, filter := range options.Filters {
			forwardOptions.Filters[i] = domain.NQEColumnFilter{
				ColumnName: filter.ColumnName,
				Value:      filter.Value,
			}
		}
	}

	return forwardOptions
}

// NQE Tool Implementations
func (s *Service) RunNQEQueryByID(ctx context.Context, args RunNQEQueryByIDArgs) (*Result, error) {
	s.logToolCall("run_nqe_query_by_id", args, nil)

	// Validate required fields
	if err := s.validateQueryID(args.QueryID); err != nil {
		return nil, err
	}

	// Use defaults if not specified
	networkID := s.getNetworkID(ctx, args.NetworkID)
	if err := s.validateNetworkID(networkID); err != nil {
		return nil, err
	}
	snapshotID := s.getSnapshotID(args.SnapshotID)

	// Proactive warning for potentially large queries
	if (args.Options == nil || args.Options.Limit == 0 || args.Options.Limit > 1000) && !args.AllResults {
		warnMsg := "⚠️ This query may return a large result set. To avoid hitting API size limits, consider setting 'all_results: true' to fetch results in batches for local analysis, or limit the output with a smaller 'limit' value.\n"
		warnMsg += "Would you like to proceed as is, or update your request?\n"
		warnMsg += "Example: { \"all_results\": true } or { \"options\": { \"limit\": 100 } }\n"
		return textResult(warnMsg), nil
	}

	if args.AllResults {
		// Fetch all results in batches using pagination
		limit := s.getQueryLimit(0)
		if args.Options != nil && args.Options.Limit > 0 {
			limit = args.Options.Limit
		}
		offset := 0
		if args.Options != nil && args.Options.Offset > 0 {
			offset = args.Options.Offset
		}

		allItems := []map[string]interface{}{}
		var lastResult *domain.NQERunResult
		for {
			params := &domain.NQEQueryParams{
				NetworkID:          networkID,
				QueryID:            args.QueryID,
				SnapshotID:         snapshotID,
				CommitID:           args.CommitID,
				UseLatestDataFiles: args.UseLatestDataFiles,
				Parameters:         args.Parameters,
				Options: &domain.NQEQueryOptions{
					Limit:  limit,
					Offset: offset,
					// Format: "json", // REMOVED: API does not support this field
				},
			}
			result, err := s.forwardClient.RunNQEQueryByID(ctx, params)
			if err != nil {
				return nil, fmt.Errorf("failed to run NQE query (batch at offset %d): %w", offset, err)
			}
			if lastResult == nil {
				lastResult = result
			}
			allItems = append(allItems, result.Items...)
			if len(result.Items) < limit {
				break // No more data
			}
			offset += limit
		}
		// Use lastResult as template for metadata, but replace Items
		if lastResult == nil {
			return textResult("No results found."), nil
		}
		lastResult.Items = allItems

		// Store in memory system/database with chunking
		var entityID string
		if s.memorySystem != nil {
			id, chunkErr := s.memorySystem.StoreNQEResultWithChunking(args.QueryID, networkID, snapshotID, lastResult, 200)
			if chunkErr != nil {
				s.logger.Warn("Failed to store NQE result with chunking: %v", chunkErr)
			} else {
				s.logger.Debug("Stored NQE result in memory system with chunking (entity: %s)", id)
				entityID = id

				// Automatically build bloom filter for large results
				if s.bloomManager != nil && len(allItems) > 100 {
					filterType := s.determineFilterType(args.QueryID, allItems)
					buildErr := s.bloomManager.BuildFilterFromNQEResult(networkID, filterType, lastResult, 200)
					if buildErr != nil {
						s.logger.Warn("Failed to auto-build bloom filter for large result: %v", buildErr)
					} else {
						s.logger.Info("Auto-built bloom filter for large result - Network: %s, Type: %s, Items: %d", networkID, filterType, len(allItems))
					}
				}
			}
		}

		// Prepare summary
		rowCount := len(allItems)
		var columns []string
		if rowCount > 0 {
			for k := range allItems[0] {
				columns = append(columns, k)
			}
		}
		previewRows := 5
		if rowCount < previewRows {
			previewRows = rowCount
		}
		preview := allItems[:previewRows]
		response := "Fetched all results in batches.\n"
		response += fmt.Sprintf("Total items: %d\nColumns: %v\n", rowCount, columns)
		previewJSON, _ := json.MarshalIndent(preview, "", "  ")
		response += fmt.Sprintf("Preview (first %d rows):\n%s\n", previewRows, string(previewJSON))
		if entityID != "" {
			response += fmt.Sprintf("Stored in memory system as entity: %s\n", entityID)
			response += "You can use get_nqe_result_summary to analyze this result locally.\n"
		}
		return textResult(response), nil
	}

	// Single page (default) behavior
	// Validate query ID against database index if available
	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	if totalQueries > 0 {
		if entry, err := s.queryIndex.GetQueryByID(args.QueryID); err != nil {
			s.logger.Warn("Query ID %s not found in database index - may be deprecated or invalid", args.QueryID)
			// Continue execution anyway in case it's a newer query not yet in the database
		} else {
			s.logger.Debug("Executing validated query: %s (Path: %s)", entry.QueryID, entry.Path)
		}
	}

	// Create cache key from query parameters
	cacheKey := fmt.Sprintf("query_id:%s|params:%v", args.QueryID, args.Parameters)

	// Try to get result from cache first. Exact match only: the key is an ID
	// plus parameters, and a "similar" key is a different query or device.
	if s.config.Forward.SemanticCache.Enabled && s.semanticCache != nil {
		if cachedResult, found := s.semanticCache.GetExact(cacheKey, networkID, snapshotID); found {
			s.logger.Debug("Cache hit for NQE query %s", args.QueryID)
			return textResult(MarshalCompactJSONString(cachedResult)), nil
		}
	}

	params := &domain.NQEQueryParams{
		NetworkID:          networkID,
		QueryID:            args.QueryID,
		SnapshotID:         snapshotID,
		CommitID:           args.CommitID,
		UseLatestDataFiles: args.UseLatestDataFiles,
		Parameters:         args.Parameters,
		Options:            s.convertNQEQueryOptions(args.Options),
	}

	// Ensure we have options even if none were provided
	if params.Options == nil {
		params.Options = &domain.NQEQueryOptions{
			Limit: s.getQueryLimit(0),
		}
	}

	// Track execution time for API memory tracking
	start := time.Now()
	result, err := s.forwardClient.RunNQEQueryByID(ctx, params)
	executionTime := time.Since(start)

	if err != nil {
		s.logToolCall("run_nqe_query_by_id", args, err)

		// Check for specific NQE query errors and provide helpful messages
		errorStr := err.Error()
		if strings.Contains(errorStr, "Invalid module path") {
			return nil, fmt.Errorf("query contains outdated module imports (this is a data quality issue in the Forward Networks repository) - query ID: %s. Try using search_nqe_queries to discover alternative queries", args.QueryID)
		}
		if strings.Contains(errorStr, "NQE_RUNTIME_ERROR") {
			return nil, fmt.Errorf("query execution failed due to code issues (this may be a data quality issue) - query ID: %s. Try using search_nqe_queries to find working alternatives. Error: %w", args.QueryID, err)
		}
		if strings.Contains(errorStr, "result exceeds maximum length") {
			// Automatic fallback to batch mode for large results
			s.logger.Warn("Result too large, retrying with all_results: true for query %s", args.QueryID)
			args.AllResults = true
			// Inform the user that we're retrying in batch mode
			msg := "The result was too large to return directly. Fetching all results in batches for local analysis. A summary will be provided.\n"
			batchResp, batchErr := s.RunNQEQueryByID(ctx, args)
			if batchErr != nil {
				return nil, batchErr
			}
			// Try to get a summary if possible
			if s.memorySystem != nil && batchResp != nil {
				// Try to extract entity ID from the batch response text
				text := batchResp.Text
				entityID := ""
				if idx := strings.Index(text, "entity: "); idx != -1 {
					end := strings.Index(text[idx:], "\n")
					if end != -1 {
						entityID = strings.TrimSpace(text[idx+len("entity: ") : idx+end])
					} else {
						entityID = strings.TrimSpace(text[idx+len("entity: "):])
					}
				}
				if entityID != "" {
					summaryArgs := GetNQEResultChunksArgs{EntityID: entityID}
					summaryResp, summaryErr := s.GetNQEResultSummary(ctx, summaryArgs)
					if summaryErr == nil && summaryResp != nil {
						msg += "\n" + summaryResp.Text
					}
				}
			}
			// Prepend our message to the batch response
			if batchResp != nil {
				batchResp.Text = msg + "\n" + batchResp.Text
			}
			return batchResp, nil
		}
		if strings.Contains(errorStr, "Provided argument") && strings.Contains(errorStr, "is not a parameter to the given query") {
			// Parameter mismatch error, suggest search_nqe_queries
			return nil, fmt.Errorf("Query parameter mismatch: %s. Try using search_nqe_queries to find working alternatives or check the required parameters for this query.", errorStr)
		}
		return nil, fmt.Errorf("failed to run NQE query: %w", err)
	}

	// Track the query execution in memory system
	if s.apiTracker != nil {
		if trackErr := s.apiTracker.TrackNetworkQuery(args.QueryID, networkID, snapshotID, result, executionTime); trackErr != nil {
			s.logger.Debug("Failed to track query execution in memory system: %v", trackErr)
		}
	}

	// Store result in memory system with chunking for LLM/large result use
	if s.memorySystem != nil {
		_, chunkErr := s.memorySystem.StoreNQEResultWithChunking(args.QueryID, networkID, snapshotID, result, 200) // 200 rows per chunk
		if chunkErr != nil {
			s.logger.Warn("Failed to store NQE result with chunking: %v", chunkErr)
		} else {
			s.logger.Debug("Stored NQE result in memory system with chunking (entity: %s)", args.QueryID)
		}
	}

	// Store result in cache for future use
	if s.config.Forward.SemanticCache.Enabled && s.semanticCache != nil {
		if cacheErr := s.semanticCache.Put(cacheKey, networkID, snapshotID, result); cacheErr != nil {
			s.logger.Warn("Failed to cache NQE query result for %s: %v", args.QueryID, cacheErr)
		} else {
			s.logger.Debug("Cached result for NQE query %s (items: %d)", args.QueryID, len(result.Items))
		}
	}

	resultJSON := MarshalCompactJSONString(result)
	s.logger.Debug("NQE query completed with %d items", len(result.Items))

	response := fmt.Sprintf("NQE query completed. Found %d items:\n%s\n\n", len(result.Items), resultJSON)

	// Pagination warning if results may be truncated
	if params.Options != nil && len(result.Items) == params.Options.Limit {
		response += "\n⚠️ Results may be truncated. Use the 'offset' parameter to fetch the next page.\n"
		response += fmt.Sprintf("Example: set 'offset' to %d to get the next page.\n", params.Options.Offset+params.Options.Limit)
		response += "Or set 'all_results: true' in your request to fetch all results in batches.\n"
	}

	// Add helpful suggestions for predefined queries
	response += "Would you like to:\n" +
		"1. Run a different predefined query?\n" +
		"2. Create a custom query?\n" +
		"3. Export these results?"

	return textResult(response), nil
}

func (s *Service) ListNQEQueries(ctx context.Context, args ListNQEQueriesArgs) (*Result, error) {
	s.logToolCall("list_nqe_queries", args, nil)

	// Inline readiness check
	if !s.queryIndex.IsReady() {
		return nil, fmt.Errorf("Query index is not initialized. Try running 'initialize_query_index' tool to manually initialize.")
	}

	// Check if query index is initialized
	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	if totalQueries == 0 {
		s.logger.Info("Query index empty, initializing from database...")

		// Try to initialize from database first, then fallback to spec
		if s.database != nil {
			queries, err := queryLoader{s.database}.loadWithSmartCachingContext(s.ctx, s.forwardClient, s.logger)
			if err != nil {
				s.logger.Warn("Database loading failed, falling back to spec file: %v", err)
				if err := s.queryIndex.LoadFromSpec(); err != nil {
					return nil, fmt.Errorf("failed to initialize query index: %w", err)
				}
			} else {
				if err := s.queryIndex.LoadFromQueries(queries); err != nil {
					return nil, fmt.Errorf("failed to load queries into index: %w", err)
				}
			}
		} else {
			if err := s.queryIndex.LoadFromSpec(); err != nil {
				return nil, fmt.Errorf("failed to initialize query index: %w", err)
			}
		}
		s.logger.Info("Query index initialized successfully")
	}

	// Use database-backed query index instead of direct API calls
	filteredEntries := s.queryIndex.FilterQueriesByDirectory(args.Directory)

	// Convert domain.NQEQueryIndexEntry to domain.NQEQuery for compatibility
	var queries []domain.NQEQuery
	for _, entry := range filteredEntries {
		queries = append(queries, entry.ConvertToNQEQuery())
	}

	// Format the response with proper JSON structure
	result := MarshalCompactJSONString(queries)

	s.logger.Debug("Found %d valid NQE queries from database index", len(queries))

	// Build a helpful response message
	response := fmt.Sprintf("Found %d NQE queries (from database cache):\n%s\n\n", len(queries), result)

	// Add helpful suggestions based on the results
	if len(queries) == 0 {
		response += "No queries found in the specified directory. Try these common directories:\n" +
			"- /L3/Basic/: Basic network queries\n" +
			"- /L3/Advanced/: Advanced network analysis\n" +
			"- /L3/Security/: Security-related queries\n\n" +
			"Would you like to:\n" +
			"1. Try a different directory?\n" +
			"2. Create a custom query?\n" +
			"3. List all available directories?"
	} else {
		response += "To run a query:\n" +
			"1. Copy the 'queryId' field from the query you want to run\n" +
			"2. Use run_nqe_query_by_id with that queryId\n" +
			"3. Optionally specify limit, offset, or other options\n\n" +
			"Would you like to:\n" +
			"1. Run one of these queries?\n" +
			"2. See more details about a specific query?\n" +
			"3. Try a different directory?"
	}

	return textResult(response), nil
}

// GetCacheStats returns semantic cache performance statistics
func (s *Service) GetCacheStats(ctx context.Context, args GetCacheStatsArgs) (*Result, error) {
	s.logToolCall("get_cache_stats", args, nil)

	stats := s.semanticCache.GetStats()

	statsJSON := MarshalCompactJSONString(stats)

	summary := fmt.Sprintf("Semantic Cache Performance Statistics:\n%s\n\nCache Summary:\n", statsJSON)
	summary += fmt.Sprintf("• Total Queries: %v\n", stats["total_queries"])
	summary += fmt.Sprintf("• Hit Rate: %v\n", stats["hit_rate_percent"])
	summary += fmt.Sprintf("• Active Entries: %v/%v\n", stats["total_entries"], stats["max_entries"])
	summary += fmt.Sprintf("• Similarity Threshold: %v\n", stats["threshold"])

	return textResult(summary), nil
}

// SuggestSimilarQueries provides intelligent query suggestions based on cache history
func (s *Service) SuggestSimilarQueries(ctx context.Context, args SuggestSimilarQueriesArgs) (*Result, error) {
	s.logToolCall("suggest_similar_queries", args, nil)

	if args.Query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}

	similarQueries, err := s.semanticCache.FindSimilarQueries(args.Query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar queries: %w", err)
	}

	if len(similarQueries) == 0 {
		return textResult(fmt.Sprintf("No similar queries found for: '%s'\n\nTry running some NQE queries first to build up the cache.", args.Query)), nil
	}

	response := fmt.Sprintf("Similar queries found for: '%s'\n\n", args.Query)
	for i, entry := range similarQueries {
		response += fmt.Sprintf("%d. (%.1f%% similarity) %s\n", i+1, entry.SimilarityScore*100, entry.Query)
		if entry.NetworkID != "" {
			response += fmt.Sprintf("   Network: %s", entry.NetworkID)
			if entry.SnapshotID != "" {
				response += fmt.Sprintf(", Snapshot: %s", entry.SnapshotID)
			}
			response += "\n"
		}
		response += fmt.Sprintf("   Used %d times, last accessed: %s\n\n", entry.AccessCount, entry.LastAccessed.Format("2006-01-02 15:04:05"))
	}

	response += "You can use these suggestions to refine your query or explore related network analysis patterns."

	return textResult(response), nil
}

// ClearCache removes expired or all cache entries
func (s *Service) ClearCache(ctx context.Context, args ClearCacheArgs) (*Result, error) {
	s.logToolCall("clear_cache", args, nil)

	var removed int
	var operation string

	if args.ClearAll {
		stats := s.semanticCache.GetStats()
		totalEntries := stats["total_entries"].(int)

		s.semanticCache.Clear()

		removed = totalEntries
		operation = "Cleared all cache entries"
	} else {
		removed = s.semanticCache.ClearExpired()
		operation = "Cleared expired cache entries"
	}

	response := fmt.Sprintf("%s: %d entries removed\n\n", operation, removed)

	// Show updated stats
	newStats := s.semanticCache.GetStats()
	response += "Updated cache status:\n"
	response += fmt.Sprintf("• Active entries: %v\n", newStats["total_entries"])
	response += fmt.Sprintf("• Hit rate: %v\n", newStats["hit_rate_percent"])

	return textResult(response), nil
}

// AI-Powered Query Discovery Tool Implementations

// SearchNQEQueries performs AI-powered search through the NQE query library
func (s *Service) SearchNQEQueries(ctx context.Context, args SearchNQEQueriesArgs) (*Result, error) {
	s.logToolCall("search_nqe_queries", args, nil)

	// Inline readiness check
	if !s.queryIndex.IsReady() {
		return nil, fmt.Errorf("Query index is not initialized. Try running 'initialize_query_index' tool to manually initialize.")
	}

	if args.Query == "" {
		return textResult("Please provide a search query describing what you want to analyze (e.g., 'AWS security vulnerabilities', 'BGP routing issues', 'interface statistics')"), nil
	}

	// Set default limit
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}

	// Initialize query index if needed
	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	if totalQueries == 0 {
		s.logger.Info("Query index empty, initializing...")
		if err := s.queryIndex.LoadFromSpec(); err != nil {
			return textResult(fmt.Sprintf("Failed to initialize query index: %v\n\n**Manual Fix:** Run this command:\n```json\n{\"tool\": \"initialize_query_index\", \"arguments\": {\"generate_embeddings\": false}}\n```", err)), nil
		}
		s.logger.Info("Query index initialized successfully")
	}

	// Use semantic search if embeddings are available, otherwise fallback to keyword search
	results, err := s.queryIndex.SearchQueries(args.Query, limit)
	if err != nil {
		return textResult(fmt.Sprintf("Search failed: %v", err)), nil
	}

	// Apply category/subcategory filters if specified
	var filteredResults []*domain.QuerySearchResult
	categoryFilterApplied := args.Category != ""
	subcategoryFilterApplied := args.Subcategory != ""

	for _, result := range results {
		if categoryFilterApplied && !strings.EqualFold(result.Category, args.Category) {
			continue
		}
		if subcategoryFilterApplied && !strings.EqualFold(result.Subcategory, args.Subcategory) {
			continue
		}
		filteredResults = append(filteredResults, result)
	}

	if len(filteredResults) == 0 {
		return textResult("No relevant NQE queries found for your search. Try different keywords or check your query index."), nil
	}

	// Format the response
	response := fmt.Sprintf("%s search found %d relevant NQE queries for: '%s'\n\n",
		filteredResults[0].MatchType, len(filteredResults), args.Query)
	for i, result := range filteredResults {
		if i >= limit {
			break
		}
		response += fmt.Sprintf("**%d. %s** (%.1f%% match)\n   **Intent:** %s\n   **Description:** %s\n   **Category:** %s\n   **Query ID:** `%s`\n\n",
			i+1, result.Path, result.SimilarityScore*100, result.Intent, result.Description, result.Category, result.QueryID)
	}

	return textResult(response), nil
}

// InitializeQueryIndex builds or rebuilds the AI-powered query index
func (s *Service) InitializeQueryIndex(ctx context.Context, args InitializeQueryIndexArgs) (*Result, error) {
	s.logToolCall("initialize_query_index", args, nil)

	response := "🔧 Initializing AI-powered NQE query index...\n\n"

	// Prefer database data over spec file if available
	var queries []domain.NQEQueryDetail
	var dataSource string

	if s.database != nil {
		response += "📊 Checking database for query data...\n"
		dbQueries, err := s.database.LoadQueries()
		if err != nil {
			response += fmt.Sprintf("⚠️  Database load failed: %v\n", err)
		} else if len(dbQueries) > 0 {
			queries = dbQueries
			dataSource = "database"
			response += fmt.Sprintf("✅ Found %d queries in database (includes enhanced metadata)\n", len(queries))

			// Count queries with enhanced metadata
			enhancedCount := 0
			for _, q := range dbQueries {
				if q.SourceCode != "" || q.Description != "" {
					enhancedCount++
				}
			}
			if enhancedCount > 0 {
				response += fmt.Sprintf("🚀 %d queries have enhanced metadata (source code/descriptions)\n", enhancedCount)
			}
		} else {
			response += "📭 Database is empty\n"
		}
	}

	// Fallback to spec file if no database data
	if len(queries) == 0 {
		response += "📖 Loading from spec file as fallback...\n"

		// Check if spec file exists using robust path resolution
		specPath, err := s.queryIndex.SpecPath()
		if err != nil {
			return textResult(fmt.Sprintf("No database data available and NQE spec file not found. Error: %v\n\n💡 **Solutions:**\n• Run 'hydrate_database' to load queries from API\n• Ensure the spec file exists in the 'spec' directory\n• Check that the MCP server is running from the correct directory", err)), nil
		}

		response += fmt.Sprintf("📁 Found spec file at: %s\n", specPath)

		if err := s.queryIndex.LoadFromSpec(); err != nil {
			return nil, fmt.Errorf("failed to load query index from spec: %w", err)
		}
		dataSource = "spec file"
	} else {
		// Load database queries into index
		if err := s.queryIndex.LoadFromQueries(queries); err != nil {
			return nil, fmt.Errorf("failed to load database queries into index: %w", err)
		}
	}

	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	embeddedQueries := stats["embedded_queries"].(int)

	response += fmt.Sprintf("✅ Loaded %d NQE queries successfully from %s\n", totalQueries, dataSource)

	if embeddedQueries > 0 {
		coverage := stats["embedding_coverage"].(float64)
		response += fmt.Sprintf("Found %d cached embeddings (%.1f%% coverage) for offline AI search\n", embeddedQueries, coverage*100)
	}
	response += "\n"

	// Generate embeddings if requested
	if args.GenerateEmbeddings {
		if s.queryIndex.UsesSyntheticEmbeddings() {
			response += "Cannot generate embeddings: OpenAI API key not configured\n"
			response += "Set OPENAI_API_KEY environment variable to enable embedding generation\n"
			response += "Current functionality limited to keyword-based search\n\n"
		} else {
			response += "Generating AI embeddings for semantic search...\n"
			response += "   This will take several minutes for thousands of queries\n"
			response += "   Embeddings will be cached for offline use\n\n"

			if err := s.queryIndex.GenerateEmbeddings(); err != nil {
				if strings.Contains(err.Error(), "cannot generate real embeddings") {
					response += "Embedding generation failed: OpenAI API key required\n"
					response += "   Set FORWARD_EMBEDDING_PROVIDER=keyword for basic functionality\n\n"
				} else {
					return nil, fmt.Errorf("failed to generate embeddings: %w", err)
				}
			} else {
				updatedStats := s.queryIndex.GetStatistics()
				newEmbeddedCount := updatedStats["embedded_queries"].(int)
				newCoverage := updatedStats["embedding_coverage"].(float64)

				response += fmt.Sprintf("Generated and cached %d embeddings (%.1f%% coverage)\n", newEmbeddedCount, newCoverage*100)
				response += "Embeddings saved to spec/nqe-embeddings.json for offline use\n\n"
			}
		}
	}

	// Show final statistics
	finalStats := s.queryIndex.GetStatistics()
	response += "📊 **Query Index Status:**\n"
	response += fmt.Sprintf("• Total queries: %d\n", finalStats["total_queries"].(int))

	if categories, ok := finalStats["categories"].(map[string]int); ok {
		response += "• Categories:\n"
		categoryCount := 0
		for category, count := range categories {
			if category != "" && categoryCount < 5 { // Show top 5 categories
				response += fmt.Sprintf("  - %s: %d queries\n", category, count)
				categoryCount++
			}
		}
		if len(categories) > 5 {
			response += fmt.Sprintf("  - ... and %d more categories\n", len(categories)-5)
		}
	}

	finalEmbedded := finalStats["embedded_queries"].(int)
	if finalEmbedded > 0 {
		finalCoverage := finalStats["embedding_coverage"].(float64)
		response += fmt.Sprintf("• AI embeddings: %d queries (%.1f%% coverage) 🧠\n", finalEmbedded, finalCoverage*100)
		response += "  → Full semantic search available\n"
	} else {
		response += "• AI embeddings: None available\n"
		response += "  → Using keyword-based search fallback\n"
	}

	response += "\n**Query index ready!**\n"
	if finalEmbedded > 0 {
		response += "Use `search_nqe_queries` for AI-powered semantic search\n"
		response += "Works offline with cached embeddings (no OpenAI API calls needed)\n"
	} else {
		response += "Use `search_nqe_queries` for keyword-based search\n"
		response += "Generate embeddings with OpenAI for better semantic matching\n"
	}

	return textResult(response), nil
}

// HydrateDatabase hydrates the database by loading queries from the Forward Networks API
func (s *Service) HydrateDatabase(ctx context.Context, args HydrateDatabaseArgs) (*Result, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database is not available")
	}

	// Set defaults
	if args.MaxRetries == 0 {
		args.MaxRetries = 3
	}

	s.logger.Info("🔄 Starting database hydration (async mode)...")

	// Check if we need to force refresh or if database is empty
	existingQueries, err := s.database.LoadQueries()
	if err != nil {
		s.logger.Warn("🔄 Failed to load existing queries: %v", err)
		existingQueries = []domain.NQEQueryDetail{}
	}

	if len(existingQueries) > 0 && !args.ForceRefresh {
		return textResult(fmt.Sprintf("Database already contains %d queries. Use force_refresh=true to refresh anyway.", len(existingQueries))), nil
	}

	// Run hydration in background. It outlives this tool call, so it takes the
	// service's context, which Shutdown cancels, not the request's.
	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
		defer cancel()
		var queries []domain.NQEQueryDetail
		var err error
		if args.EnhancedMode {
			existingCommitIDs := make(map[string]string)
			for _, query := range existingQueries {
				if query.LastCommit.ID != "" {
					existingCommitIDs[query.Path] = query.LastCommit.ID
				}
			}
			queries, err = s.forwardClient.GetNQEAllQueriesEnhanced(ctx, existingCommitIDs)
			if err != nil {
				s.logger.Warn("🔄 Enhanced API failed, falling back to basic API: %v", err)
				queries, err = queryLoader{s.database}.loadFromBasicAPI(ctx, s.forwardClient, s.logger)
			}
		} else {
			queries, err = queryLoader{s.database}.loadFromBasicAPI(ctx, s.forwardClient, s.logger)
		}
		if err != nil {
			s.logger.Error("failed to load queries from API: %v", err)
			return
		}
		if !args.ForceRefresh && len(existingQueries) > 0 {
			queries = queryLoader{s.database}.mergeQueries(existingQueries, queries)
		}
		if err := s.database.SaveQueries(queries); err != nil {
			s.logger.Error("failed to save queries to database: %v", err)
			return
		}
		if err := s.database.SetMetadata("last_sync", time.Now().Format(time.RFC3339)); err != nil {
			s.logger.Warn("🔄 Failed to update sync time: %v", err)
		}
		s.logger.Info("🔄 Database hydration completed with %d queries", len(queries))
		s.logger.Info("🔄 Refreshing query index after hydration...")
		if s.queryIndex != nil {
			if err := s.queryIndex.LoadFromQueries(queries); err != nil {
				s.logger.Warn("🔄 Failed to refresh query index: %v", err)
			} else {
				s.logger.Info("🔄 Query index refreshed with %d queries", len(queries))
				stats := s.queryIndex.GetStatistics()
				embeddedCount := stats["embedded_queries"].(int)
				if embeddedCount > 0 && embeddedCount < len(queries) {
					s.logger.Info("🧠 Consider regenerating embeddings to include new queries in semantic search")
				}
			}
		}
		if s.queryIndex != nil && args.RegenerateEmbeddings {
			s.logger.Info("🧠 Regenerating AI embeddings after hydration...")
			if s.queryIndex.UsesSyntheticEmbeddings() {
				s.logger.Warn("⚠️  Cannot generate embeddings: OpenAI API key not configured")
			} else {
				if err := s.queryIndex.GenerateEmbeddings(); err != nil {
					s.logger.Warn("🧠 Failed to regenerate embeddings: %v", err)
				} else {
					updatedStats := s.queryIndex.GetStatistics()
					newEmbeddedCount := updatedStats["embedded_queries"].(int)
					newCoverage := updatedStats["embedding_coverage"].(float64)
					s.logger.Info("🧠 Successfully regenerated %d embeddings (%.1f%% coverage)", newEmbeddedCount, newCoverage*100)
				}
			}
		}
		// Optionally: log completion
		s.logger.Info("Database hydration background process complete.")
	}()

	return textResult("Database hydration has started in the background. This process may take several minutes. You can continue using other tools, or check the status with get_database_status. Once hydration is complete, the query index will be refreshed automatically."), nil
}

// RefreshQueryIndex refreshes the query index from the current database content
func (s *Service) RefreshQueryIndex(ctx context.Context, args RefreshQueryIndexArgs) (*Result, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database is not available")
	}

	if s.queryIndex == nil {
		return nil, fmt.Errorf("query index is not available")
	}

	s.logger.Info("🔄 Refreshing query index from database...")

	// Load queries from database
	queries, err := s.database.LoadQueries()
	if err != nil {
		return nil, fmt.Errorf("failed to load queries from database: %w", err)
	}

	if len(queries) == 0 {
		return textResult("No queries found in database. Use hydrate_database to load queries first."), nil
	}

	// Load queries into index
	if err := s.queryIndex.LoadFromQueries(queries); err != nil {
		return nil, fmt.Errorf("failed to load queries into index: %w", err)
	}

	s.logger.Info("🔄 Query index refreshed with %d queries", len(queries))

	return textResult(fmt.Sprintf("Query index refreshed successfully with %d queries.", len(queries))), nil
}

// GetDatabaseStatus returns the current status of the database and query index
func (s *Service) GetDatabaseStatus(ctx context.Context, args GetDatabaseStatusArgs) (*Result, error) {
	status := map[string]interface{}{
		"database_available":    s.database != nil,
		"query_index_available": s.queryIndex != nil,
		"timestamp":             time.Now().Format(time.RFC3339),
	}

	if s.database != nil {
		// Get database stats
		queries, err := s.database.LoadQueries()
		if err != nil {
			status["database_error"] = err.Error()
			status["query_count"] = 0
		} else {
			status["query_count"] = len(queries)
		}

		// Get last sync time
		if lastSync, err := s.database.GetMetadata("last_sync"); err == nil {
			status["last_sync"] = lastSync
		}

		// Get database path
		status["database_path"] = s.database.Path()
	}

	if s.queryIndex != nil {
		// Get query index stats
		status["query_index_empty"] = !s.queryIndex.IsReady()
		status["query_index_loading"] = s.queryIndex.IsLoading()

		// Get index stats if available
		indexStats := s.queryIndex.GetStatistics()
		if indexStats != nil {
			status["index_stats"] = indexStats
		}
	}

	// Marshal to JSON for pretty output
	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal status: %w", err)
	}

	return textResult(string(statusJSON)), nil
}

// Memory Management Tool Implementations

// ListInstanceIDs lists all available Forward Networks instance IDs in the database
func (s *Service) ListInstanceIDs(ctx context.Context, args ListInstanceIDsArgs) (*Result, error) {
	s.logToolCall("list_instance_ids", args, nil)

	if s.database == nil {
		return nil, fmt.Errorf("database is not available")
	}

	instances, err := s.database.GetAllInstanceIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance IDs: %w", err)
	}

	if len(instances) == 0 {
		return textResult("No Forward Networks instances found in the database."), nil
	}

	// Build response
	var responseText strings.Builder
	responseText.WriteString(fmt.Sprintf("Found %d Forward Networks instances in the database:\n\n", len(instances)))

	for i, instance := range instances {
		responseText.WriteString(fmt.Sprintf("%d. **Instance ID: %s**\n", i+1, instance.ID))
		responseText.WriteString(fmt.Sprintf("   - Query Count: %d\n", instance.QueryCount))
		responseText.WriteString(fmt.Sprintf("   - First Sync: %s\n", instance.FirstSync.Format("2006-01-02 15:04:05")))
		responseText.WriteString(fmt.Sprintf("   - Last Sync: %s\n", instance.LastSync.Format("2006-01-02 15:04:05")))
		responseText.WriteString("\n")
	}

	responseText.WriteString("**To use a specific instance:**\n")
	responseText.WriteString("1. Set the FORWARD_INSTANCE_ID environment variable to the desired instance ID\n")
	responseText.WriteString("2. Restart the MCP server\n")
	responseText.WriteString("3. The server will then use queries from that instance\n\n")
	responseText.WriteString("**Example:**\n")
	responseText.WriteString("```bash\n")
	responseText.WriteString("export FORWARD_INSTANCE_ID=49bd3225\n")
	responseText.WriteString("# Then restart your MCP server\n")
	responseText.WriteString("```")

	return textResult(responseText.String()), nil
}

func (s *Service) CompareNQEResults(ctx context.Context, args CompareNQEResultsArgs) (*Result, error) {
	// Validate required parameters
	if args.BeforeSnapshotID == "" {
		return nil, fmt.Errorf("before_snapshot_id is required")
	}
	if args.AfterSnapshotID == "" {
		return nil, fmt.Errorf("after_snapshot_id is required")
	}
	if args.QueryID == "" {
		return nil, fmt.Errorf("query_id is required. Use list_nqe_queries or search_nqe_queries to find query IDs")
	}

	// Build diff request
	diffRequest := &domain.NQEDiffRequest{
		QueryID:    args.QueryID,
		CommitID:   args.CommitID,
		Options:    s.convertNQEQueryOptions(args.Options),
		Parameters: args.Parameters,
	}

	// Execute diff query
	result, err := s.forwardClient.DiffNQEQuery(ctx, args.BeforeSnapshotID, args.AfterSnapshotID, diffRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to compare NQE results: %w", err)
	}

	// Format result as JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to format diff result: %w", err)
	}

	return textResult(string(jsonBytes)), nil
}
