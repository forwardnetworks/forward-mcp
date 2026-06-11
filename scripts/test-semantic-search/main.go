package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
)

func main() {
	fmt.Println("🔍 Testing Forward Networks MCP Semantic Search")
	fmt.Println("==============================================")

	// Load config
	cfg := config.LoadConfig()
	if cfg.Forward.APIKey == "" {
		fmt.Println("❌ No API key found. Make sure your .env file is configured.")
		return
	}

	// Initialize logger
	appLogger := logger.New()

	// Initialize Forward client (for potential future use)
	_ = forward.NewClient(&cfg.Forward)

	// Initialize embedding service (will use keyword fallback if no OpenAI key)
	var embeddingService service.EmbeddingService
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		embeddingService = service.NewOpenAIEmbeddingService(openaiKey)
		fmt.Println("🧠 Using OpenAI embedding service for AI semantic search")
	} else {
		embeddingService = service.NewKeywordEmbeddingService()
		fmt.Println("🔤 Using keyword embedding service (no OpenAI key found)")
	}

	// Initialize database
	database, err := service.NewNQEDatabase(appLogger, "default")
	if err != nil {
		fmt.Printf("❌ Failed to create database: %v\n", err)
		return
	}
	defer database.Close()

	// Initialize NQE query index
	queryIndex := service.NewNQEQueryIndex(embeddingService, appLogger)

	fmt.Println("📊 Testing query index loading...")

	// Test 1: Load queries from database/API using smart caching
	fmt.Println("\n1️⃣ Loading queries from database/API...")
	// Try loading from database first
	existingQueries, err := database.LoadQueries()
	if err != nil || len(existingQueries) < 3000 {
		// Fallback to spec file loading
		fmt.Println("📄 Loading from spec file...")
		if err := queryIndex.LoadFromSpec(); err != nil {
			fmt.Printf("❌ Failed to load queries: %v\n", err)
			return
		}
	} else {
		// Load queries into the index
		if err := queryIndex.LoadFromQueries(existingQueries); err != nil {
			fmt.Printf("❌ Failed to load queries into index: %v\n", err)
			return
		}
	}

	// Get statistics
	stats := queryIndex.GetStatistics()
	fmt.Printf("✅ Loaded %v queries\n", stats["total_queries"])
	fmt.Printf("📊 Embedded queries: %v\n", stats["embedded_queries"])
	fmt.Printf("📈 Embedding coverage: %.2f%%\n", stats["embedding_coverage"].(float64)*100)

	// Test 2: Test semantic search
	fmt.Println("\n2️⃣ Testing semantic search...")

	testQueries := []string{
		"AWS security issues",
		"BGP routing problems",
		"interface utilization",
		"network devices",
		"security vulnerabilities",
		"cloud infrastructure",
		"OSPF configuration",
		"VLAN configuration",
		"firewall rules",
		"load balancing",
	}

	successfulSearches := 0
	totalResults := 0

	for i, query := range testQueries {
		fmt.Printf("\n🔍 Test %d: Searching for '%s'\n", i+1, query)

		results, err := queryIndex.SearchQueries(query, 3) // Limit to 3 for cleaner output
		if err != nil {
			fmt.Printf("❌ Search failed: %v\n", err)
			continue
		}

		if len(results) == 0 {
			fmt.Printf("⚠️  No results found\n")
			continue
		}

		successfulSearches++
		totalResults += len(results)

		fmt.Printf("✅ Found %d results:\n", len(results))
		for j, result := range results {
			fmt.Printf("   %d. %s (score: %.3f, type: %s)\n",
				j+1, result.Path, result.SimilarityScore, result.MatchType)
			if result.Intent != "" && result.Intent != result.Path {
				fmt.Printf("      Intent: %s\n", result.Intent)
			}
		}
	}

	// Test 3: Test specific query lookup
	fmt.Println("\n3️⃣ Testing query lookup by ID...")

	queries := queryIndex.Queries()
	if len(queries) > 0 {
		firstQuery := queries[0]
		fmt.Printf("🔍 Looking up query ID: %s\n", firstQuery.QueryID)

		found, err := queryIndex.GetQueryByID(firstQuery.QueryID)
		if err != nil {
			fmt.Printf("❌ Lookup failed: %v\n", err)
		} else {
			fmt.Printf("✅ Found query: %s\n", found.Path)
			fmt.Printf("   Intent: %s\n", found.Intent)
			fmt.Printf("   Category: %s\n", found.Category)
			// Repository info not available in NQEQueryIndexEntry
		}
	}

	// Test 4: Database statistics
	fmt.Println("\n4️⃣ Database statistics...")
	dbStats, err := database.GetStatistics()
	if err != nil {
		fmt.Printf("❌ Failed to get database stats: %v\n", err)
	} else {
		dbStatsJSON, _ := json.MarshalIndent(dbStats, "", "  ")
		fmt.Printf("📊 Database stats:\n%s\n", string(dbStatsJSON))
	}

	// Summary
	fmt.Println("\n📋 TEST SUMMARY")
	fmt.Println("===============")
	fmt.Printf("✅ Successful searches: %d/%d\n", successfulSearches, len(testQueries))
	fmt.Printf("📊 Total results found: %d\n", totalResults)
	fmt.Printf("🗄️  Database queries: %v\n", stats["total_queries"])
	fmt.Printf("🧠 Embedding coverage: %.1f%%\n", stats["embedding_coverage"].(float64)*100)

	if stats["embedded_queries"].(int) == 0 {
		fmt.Println("\n🔧 To enable AI semantic search:")
		fmt.Println("   Set OPENAI_API_KEY in your .env file and run:")
		fmt.Println("   make embedding-generate-openai")
	} else {
		fmt.Println("\n🎉 AI semantic search is enabled and working!")
	}

	fmt.Println("\n✅ Semantic search test completed successfully!")
}
