package main

import (
	"fmt"
	"os"
	"time"

	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
)

func main() {
	fmt.Println("🔍 Forward Networks MCP - Embedding Status Report")
	fmt.Println("==================================================")

	// Initialize logger
	logger := logger.New()

	// Check which embedding service would be used
	provider := os.Getenv("FORWARD_EMBEDDING_PROVIDER")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	fmt.Printf("🎛️  Current Configuration:\n")
	if provider != "" {
		fmt.Printf("   📋 FORWARD_EMBEDDING_PROVIDER: %s\n", provider)
	} else {
		fmt.Printf("   📋 FORWARD_EMBEDDING_PROVIDER: (not set - will auto-detect)\n")
	}

	if openaiKey != "" {
		fmt.Printf("   🔑 OPENAI_API_KEY: Set (***%s)\n", openaiKey[len(openaiKey)-4:])
	} else {
		fmt.Printf("   🔑 OPENAI_API_KEY: Not set\n")
	}

	// Initialize embedding service based on configuration
	var embeddingService service.EmbeddingService
	var serviceName string

	switch provider {
	case "keyword":
		embeddingService = service.NewKeywordEmbeddingService()
		serviceName = "Keyword-based (fast, free, offline)"
	case "openai":
		if openaiKey == "" {
			fmt.Printf("\n❌ Error: OPENAI_API_KEY required for OpenAI provider\n")
			os.Exit(1)
		}
		embeddingService = service.NewOpenAIEmbeddingService(openaiKey)
		serviceName = "OpenAI API (high quality, costs money)"
	default:
		// Auto-detect
		if openaiKey != "" {
			embeddingService = service.NewOpenAIEmbeddingService(openaiKey)
			serviceName = "OpenAI API (auto-detected from OPENAI_API_KEY)"
		} else {
			embeddingService = service.NewKeywordEmbeddingService()
			serviceName = "Keyword-based (auto-detected, no OpenAI key)"
		}
	}

	fmt.Printf("   🤖 Active Service: %s\n", serviceName)

	// Initialize query index
	queryIndex := service.NewNQEQueryIndex(embeddingService, logger)

	// Load queries
	fmt.Printf("\n📖 Loading NQE Queries:\n")
	startTime := time.Now()

	if err := queryIndex.LoadFromSpec(); err != nil {
		fmt.Printf("❌ Failed to load query index: %v\n", err)
		os.Exit(1)
	}

	loadTime := time.Since(startTime)
	fmt.Printf("   ✅ Loaded in %v\n", loadTime)

	// Get statistics
	stats := queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	embeddedQueries := stats["embedded_queries"].(int)
	coverage := stats["embedding_coverage"].(float64)
	categories := stats["categories"].(map[string]int)

	fmt.Printf("\n📊 Query Statistics:\n")
	fmt.Printf("   📋 Total Queries: %d\n", totalQueries)
	fmt.Printf("   🧠 Embedded Queries: %d\n", embeddedQueries)
	fmt.Printf("   📈 Coverage: %.1f%%\n", coverage*100)

	// Coverage assessment
	fmt.Printf("\n🎯 Coverage Assessment:\n")
	if coverage >= 0.95 {
		fmt.Printf("   ✅ Excellent coverage (%.1f%%) - ready for production\n", coverage*100)
	} else if coverage >= 0.80 {
		fmt.Printf("   🟡 Good coverage (%.1f%%) - mostly ready\n", coverage*100)
	} else if coverage >= 0.50 {
		fmt.Printf("   🟠 Moderate coverage (%.1f%%) - consider regenerating\n", coverage*100)
	} else {
		fmt.Printf("   ❌ Low coverage (%.1f%%) - embeddings need generation\n", coverage*100)
	}

	// Show category breakdown
	fmt.Printf("\n📂 Categories:\n")
	for category, count := range categories {
		if category == "" {
			category = "(uncategorized)"
		}
		fmt.Printf("   📁 %s: %d queries\n", category, count)
	}

	// Test search functionality
	fmt.Printf("\n🔍 Search Test:\n")
	testQueries := []string{"device inventory", "bgp routing", "security"}

	for _, testQuery := range testQueries {
		results, err := queryIndex.SearchQueries(testQuery, 3)
		if err != nil {
			fmt.Printf("   ❌ '%s': Error - %v\n", testQuery, err)
		} else {
			fmt.Printf("   ✅ '%s': Found %d results\n", testQuery, len(results))
		}
	}

	// Performance benchmark
	fmt.Printf("\n⚡ Performance Test:\n")
	searchStart := time.Now()
	_, err := queryIndex.SearchQueries("network device configuration analysis", 10)
	searchTime := time.Since(searchStart)

	if err != nil {
		fmt.Printf("   ❌ Search failed: %v\n", err)
	} else {
		fmt.Printf("   ⚡ Search time: %v", searchTime)
		if searchTime < time.Millisecond {
			fmt.Printf(" (excellent! sub-millisecond)\n")
		} else if searchTime < 10*time.Millisecond {
			fmt.Printf(" (good)\n")
		} else {
			fmt.Printf(" (slow - consider optimization)\n")
		}
	}

	// Cache file info
	fmt.Printf("\n💾 Cache Information:\n")
	cacheFile := "spec/nqe-embeddings.json"
	if info, err := os.Stat(cacheFile); err == nil {
		fmt.Printf("   ✅ Cache file exists: %s\n", cacheFile)
		fmt.Printf("   📁 Size: %.2f MB\n", float64(info.Size())/(1024*1024))
		fmt.Printf("   📅 Last modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("   ❌ No cache file found\n")
		fmt.Printf("   💡 Run 'make embedding-generate-keyword' to create embeddings\n")
	}

	// Recommendations
	fmt.Printf("\n💡 Recommendations:\n")
	if coverage < 0.8 {
		fmt.Printf("   🔧 Run 'make embedding-generate-keyword' for fast, free embeddings\n")
		fmt.Printf("   🧠 Or 'make embedding-generate-openai' for higher quality (costs money)\n")
	} else {
		fmt.Printf("   ✅ Embeddings look good! No action needed.\n")
		fmt.Printf("   🚀 Your search system is ready for production use.\n")
	}

	fmt.Printf("\n🎉 Status check complete!\n")
}
