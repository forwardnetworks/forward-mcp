package main

import (
	"fmt"

	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
)

func main() {
	logger := logger.New()

	// Test with keyword-based search (current recommended)
	embeddingService := service.NewKeywordEmbeddingService()
	queryIndex := service.NewNQEQueryIndex(embeddingService, logger)

	// Load the query index
	if err := queryIndex.LoadFromSpec(); err != nil {
		fmt.Printf("Error loading spec: %v\n", err)
		return
	}

	// Show available query categories and examples
	stats := queryIndex.GetStatistics()
	fmt.Printf("📚 Available Query Library:\n")
	fmt.Printf("   Total queries: %d\n", stats["total_queries"])
	fmt.Printf("   Embedded queries: %d\n", stats["embedded_queries"])
	fmt.Printf("   Coverage: %.1f%%\n\n", stats["embedding_coverage"].(float64)*100)

	// Test searches with broader terms that might match
	searchTerms := []string{
		"interface",
		"device",
		"route",
		"security",
		"AWS",
		"L3",
	}

	fmt.Println("🔍 Testing Search Functionality:")
	for _, term := range searchTerms {
		fmt.Printf("\n  Searching: '%s'\n", term)
		results, err := queryIndex.SearchQueries(term, 3)
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
			continue
		}

		if len(results) == 0 {
			fmt.Printf("    No matches found\n")
			continue
		}

		for i, result := range results {
			fmt.Printf("    %d. %s (%.1f%% match)\n", i+1, result.Path, result.SimilarityScore*100)
			if result.Intent != "" && len(result.Intent) > 0 {
				intent := result.Intent
				if len(intent) > 80 {
					intent = intent[:80] + "..."
				}
				fmt.Printf("       → %s\n", intent)
			}
		}
	}
}
