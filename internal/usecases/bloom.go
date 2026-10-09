package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/forward-mcp/internal/domain"
)

// BuildBloomFilter builds a bloom filter from NQE query results
func (s *Service) BuildBloomFilter(ctx context.Context, args BuildBloomFilterArgs) (*Result, error) {
	s.logToolCall("build_bloom_filter", args, nil)

	if s.bloomManager == nil {
		return nil, fmt.Errorf("bloom search manager is not available")
	}

	// Use defaults if not specified
	networkID := s.getNetworkID(ctx, args.NetworkID)
	chunkSize := args.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 200 // Default chunk size
	}

	// Run the NQE query to get data for building the filter
	params := &domain.NQEQueryParams{
		NetworkID:  networkID,
		QueryID:    args.QueryID,
		SnapshotID: s.getSnapshotID(""),
		Options: &domain.NQEQueryOptions{
			Limit: 1000, // Reasonable limit for filter building
		},
	}

	result, err := s.forwardClient.RunNQEQueryByID(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to run NQE query for filter building: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no data found for building bloom filter")
	}

	// Build the bloom filter
	err = s.bloomManager.BuildFilterFromNQEResult(networkID, args.FilterType, result, chunkSize)
	if err != nil {
		return nil, fmt.Errorf("failed to build bloom filter: %w", err)
	}

	// Get filter stats
	stats := s.bloomManager.GetFilterStats()
	filterKey := fmt.Sprintf("%s-%s", networkID, args.FilterType)
	metadata := stats[filterKey]

	response := fmt.Sprintf("✅ Bloom filter built successfully!\n\n"+
		"**Filter Details:**\n"+
		"- Network ID: %s\n"+
		"- Filter Type: %s\n"+
		"- Items Processed: %d\n"+
		"- Memory Usage: %d bytes\n"+
		"- False Positive Rate: %.2f%%\n"+
		"- Chunks: %d\n\n"+
		"**Next Steps:**\n"+
		"Use `search_bloom_filter` to efficiently search this dataset with sub-millisecond performance.",
		networkID, args.FilterType, metadata.ItemCount, metadata.MemoryUsage,
		metadata.FalsePositiveRate*100, metadata.ChunkCount)

	return textResult(response), nil
}

// SearchBloomFilter searches a bloom filter for matching items
func (s *Service) SearchBloomFilter(ctx context.Context, args SearchBloomFilterArgs) (*Result, error) {
	s.logToolCall("search_bloom_filter", args, nil)

	if s.bloomManager == nil {
		return nil, fmt.Errorf("bloom search manager is not available")
	}

	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Use defaults if not specified
	networkID := s.getNetworkID(ctx, args.NetworkID)

	// Check if filter exists
	if !s.bloomManager.IsFilterAvailable(networkID, args.FilterType) {
		return nil, fmt.Errorf("no bloom filter found for %s (network: %s). Use build_bloom_filter first.", args.FilterType, networkID)
	}

	// Get the full dataset from memory system
	chunks, err := s.memorySystem.GetNQEResultChunks(args.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve result chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no data found for entity %s", args.EntityID)
	}

	// Parse all rows from all chunks
	var allItems []map[string]interface{}
	for _, chunk := range chunks {
		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(chunk), &rows); err != nil {
			return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
		}
		allItems = append(allItems, rows...)
	}

	// Search the bloom filter
	searchResult, err := s.bloomManager.SearchFilter(networkID, args.FilterType, args.SearchTerms, allItems)
	if err != nil {
		return nil, fmt.Errorf("failed to search bloom filter: %w", err)
	}

	// Format response
	response := fmt.Sprintf("🔍 Bloom Search Results\n\n"+
		"**Search Performance:**\n"+
		"- Search Time: %v\n"+
		"- Total Items: %d\n"+
		"- Matched Items: %d\n"+
		"- Search Terms: %v\n\n"+
		"**Filter Stats:**\n"+
		"- Network ID: %s\n"+
		"- Filter Type: %s\n"+
		"- Memory Usage: %d bytes\n"+
		"- False Positive Rate: %.2f%%\n\n"+
		"**Matched Items (%d):**\n",
		searchResult.SearchTime, searchResult.TotalItems, searchResult.MatchedCount,
		args.SearchTerms, searchResult.FilterStats.NetworkID, searchResult.FilterStats.FilterType,
		searchResult.FilterStats.MemoryUsage, searchResult.FilterStats.FalsePositiveRate*100,
		len(searchResult.MatchedItems))

	// Add matched items (limit to first 10 for display)
	displayLimit := 10
	if len(searchResult.MatchedItems) < displayLimit {
		displayLimit = len(searchResult.MatchedItems)
	}

	for i := 0; i < displayLimit; i++ {
		itemJSON, _ := json.MarshalIndent(searchResult.MatchedItems[i], "", "  ")
		response += fmt.Sprintf("%d. %s\n", i+1, string(itemJSON))
	}

	if len(searchResult.MatchedItems) > displayLimit {
		response += fmt.Sprintf("\n... and %d more items (use analyze_nqe_result_sql for full analysis)\n",
			len(searchResult.MatchedItems)-displayLimit)
	}

	return textResult(response), nil
}

// GetBloomFilterStats returns statistics for all bloom filters
func (s *Service) GetBloomFilterStats(ctx context.Context, args GetBloomFilterStatsArgs) (*Result, error) {
	s.logToolCall("get_bloom_filter_stats", args, nil)

	if s.bloomManager == nil {
		return nil, fmt.Errorf("bloom search manager is not available")
	}

	stats := s.bloomManager.GetFilterStats()
	totalMemory := s.bloomManager.GetMemoryUsage()

	if len(stats) == 0 {
		return textResult("No bloom filters found. Use `build_bloom_filter` to create filters for efficient searching."), nil
	}

	response := fmt.Sprintf("📊 Bloom Filter Statistics\n\n"+
		"**Overall Stats:**\n"+
		"- Total Filters: %d\n"+
		"- Total Memory Usage: %d bytes (%.2f MB)\n\n"+
		"**Filter Details:**\n",
		len(stats), totalMemory, float64(totalMemory)/(1024*1024))

	for key, metadata := range stats {
		response += fmt.Sprintf("**%s**\n"+
			"- Network ID: %s\n"+
			"- Filter Type: %s\n"+
			"- Items: %d\n"+
			"- Memory: %d bytes\n"+
			"- False Positive Rate: %.2f%%\n"+
			"- Last Updated: %s\n"+
			"- Chunks: %d\n\n",
			key, metadata.NetworkID, metadata.FilterType, metadata.ItemCount,
			metadata.MemoryUsage, metadata.FalsePositiveRate*100,
			metadata.LastUpdated.Format("2006-01-02 15:04:05"), metadata.ChunkCount)
	}

	response += "**Performance Benefits:**\n" +
		"- Sub-millisecond search performance\n" +
		"- Memory-efficient filtering\n" +
		"- Reduced API calls for large datasets\n" +
		"- Pre-filtering before SQL analysis"

	return textResult(response), nil
}

// determineFilterType determines the appropriate filter type based on query ID and data content
func (s *Service) determineFilterType(queryID string, items []map[string]interface{}) string {
	// Check query ID patterns first
	if strings.Contains(queryID, "device") || strings.Contains(queryID, "devices") {
		return "device"
	}
	if strings.Contains(queryID, "interface") || strings.Contains(queryID, "interfaces") {
		return "interface"
	}
	if strings.Contains(queryID, "config") || strings.Contains(queryID, "configuration") {
		return "config"
	}
	if strings.Contains(queryID, "route") || strings.Contains(queryID, "routing") {
		return "route"
	}
	if strings.Contains(queryID, "vlan") {
		return "vlan"
	}
	if strings.Contains(queryID, "acl") || strings.Contains(queryID, "firewall") {
		return "security"
	}

	// Fallback: analyze the actual data structure
	if len(items) > 0 {
		item := items[0]
		if _, hasDevice := item["device_name"]; hasDevice {
			return "device"
		}
		if _, hasInterface := item["interface_name"]; hasInterface {
			return "interface"
		}
		if _, hasConfig := item["configuration"]; hasConfig {
			return "config"
		}
	}

	// Default to generic type
	return "data"
}

// extractSearchTerms extracts meaningful search terms from a query string
func (s *Service) extractSearchTerms(query string) []string {
	if query == "" {
		return nil
	}

	// Split by common delimiters and clean up
	terms := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '|' || r == '&'
	})

	var cleanTerms []string
	for _, term := range terms {
		// Remove common stop words and short terms
		term = strings.TrimSpace(term)
		if len(term) > 2 && !s.isStopWord(term) {
			cleanTerms = append(cleanTerms, strings.ToLower(term))
		}
	}

	return cleanTerms
}

// isStopWord checks if a word is a common stop word
func (s *Service) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true,
		"to": true, "for": true, "of": true, "with": true, "by": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
		"a": true, "an": true, "this": true, "that": true, "these": true, "those": true,
		"all": true, "any": true, "some": true, "no": true, "not": true, "only": true, "just": true,
	}
	return stopWords[strings.ToLower(word)]
}
