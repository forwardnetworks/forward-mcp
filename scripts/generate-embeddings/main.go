package main

import (
	"fmt"
	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/ports"
	"os"
	"time"

	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/service"
)

func main() {
	fmt.Println("🤖 Forward Networks MCP - Embedding Generation")
	fmt.Println("==============================================")

	// Initialize logger
	logger := stderrlog.New()

	// Check configuration
	provider := os.Getenv("FORWARD_EMBEDDING_PROVIDER")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	fmt.Printf("🎛️  Configuration:\n")
	fmt.Printf("   📋 Provider: %s\n", provider)

	// Initialize embedding service based on provider
	var embeddingService ports.EmbeddingService
	var serviceName, costInfo string

	switch provider {
	case "keyword":
		embeddingService = embeddings.NewKeywordEmbeddingService()
		serviceName = "Keyword-based Embeddings"
		costInfo = "💰 Cost: $0.00 (free!)"
	case "openai":
		if openaiKey == "" {
			fmt.Printf("❌ Error: OPENAI_API_KEY environment variable not set\n")
			fmt.Printf("💡 Set it with: export OPENAI_API_KEY=your-key-here\n")
			os.Exit(1)
		}
		embeddingService = embeddings.NewOpenAIEmbeddingService(openaiKey)
		serviceName = "OpenAI API Embeddings"
		costInfo = "💰 Estimated cost: $1-5 for 6000+ queries"
	default:
		fmt.Printf("❌ Error: Invalid FORWARD_EMBEDDING_PROVIDER: %s\n", provider)
		fmt.Printf("💡 Valid options: 'keyword' or 'openai'\n")
		fmt.Printf("💡 Example: export FORWARD_EMBEDDING_PROVIDER=keyword\n")
		os.Exit(1)
	}

	fmt.Printf("   🤖 Service: %s\n", serviceName)
	fmt.Printf("   %s\n", costInfo)

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

	// Get pre-generation statistics
	stats := queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	embeddedQueries := stats["embedded_queries"].(int)
	coverage := stats["embedding_coverage"].(float64)

	fmt.Printf("\n📊 Current Statistics:\n")
	fmt.Printf("   📋 Total Queries: %d\n", totalQueries)
	fmt.Printf("   🧠 Already Embedded: %d\n", embeddedQueries)
	fmt.Printf("   📈 Coverage: %.1f%%\n", coverage*100)

	if coverage >= 0.95 {
		fmt.Printf("\n✅ Embeddings already at excellent coverage (%.1f%%)!\n", coverage*100)
		fmt.Printf("💡 No generation needed. Use 'make embedding-clean' first if you want to regenerate.\n")
		return
	}

	remaining := totalQueries - embeddedQueries
	fmt.Printf("   🔄 To Generate: %d queries\n", remaining)

	// Time estimation
	var estimatedTime time.Duration
	switch provider {
	case "keyword":
		// Keyword embeddings are very fast
		estimatedTime = time.Duration(remaining) * time.Microsecond * 100 // ~100µs per embedding
		fmt.Printf("   ⚡ Estimated time: %v (very fast!)\n", estimatedTime)
	case "openai":
		// OpenAI API is slower due to network calls
		estimatedTime = time.Duration(remaining) * time.Millisecond * 200 // ~200ms per embedding
		fmt.Printf("   🐌 Estimated time: %v (API limited)\n", estimatedTime)
	}

	// Confirm before proceeding
	fmt.Printf("\n⚠️  Ready to generate embeddings?\n")
	if provider == "openai" {
		fmt.Printf("💰 This will make %d API calls to OpenAI\n", remaining)
		fmt.Printf("💸 Estimated cost: $%.2f\n", float64(remaining)*0.0001) // Rough estimate
	}

	fmt.Printf("Continue? (y/N): ")
	var confirm string
	_, err := fmt.Scanln(&confirm)
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}

	if confirm != "y" && confirm != "Y" {
		fmt.Printf("❌ Operation cancelled\n")
		return
	}

	// Generate embeddings
	fmt.Printf("\n🚀 Starting embedding generation...\n")
	fmt.Printf("📊 Progress will be logged as we go...\n")

	generationStart := time.Now()

	if err := queryIndex.GenerateEmbeddings(); err != nil {
		fmt.Printf("❌ Failed to generate embeddings: %v\n", err)
		os.Exit(1)
	}

	generationTime := time.Since(generationStart)

	// Get post-generation statistics
	finalStats := queryIndex.GetStatistics()
	finalEmbedded := finalStats["embedded_queries"].(int)
	finalCoverage := finalStats["embedding_coverage"].(float64)

	// Success report
	fmt.Printf("\n🎉 Embedding Generation Complete!\n")
	fmt.Printf("================================\n")
	fmt.Printf("   ⏱️  Total time: %v\n", generationTime)
	fmt.Printf("   📈 Final coverage: %.1f%% (%d/%d queries)\n", finalCoverage*100, finalEmbedded, totalQueries)
	fmt.Printf("   🆕 Generated: %d new embeddings\n", finalEmbedded-embeddedQueries)

	// Print the first embedding vector for verification
	for _, query := range queryIndex.Queries() {
		if len(query.Embedding) > 0 {
			fmt.Printf("\n🔬 Example embedding vector for query: %s\n", query.Path)
			fmt.Printf("[ ")
			for i, v := range query.Embedding {
				if i > 0 && i < 10 {
					fmt.Printf(", ")
				}
				if i < 10 {
					fmt.Printf("%.6f", v)
				}
			}
			if len(query.Embedding) > 10 {
				fmt.Printf(", ... (total %d values)", len(query.Embedding))
			}
			fmt.Printf(" ]\n")
			break
		}
	}

	if provider == "keyword" {
		fmt.Printf("   ⚡ Performance: %.0f embeddings/second\n", float64(finalEmbedded-embeddedQueries)/generationTime.Seconds())
	}

	// Performance test
	fmt.Printf("\n🔍 Testing search performance...\n")
	searchStart := time.Now()
	results, err := queryIndex.SearchQueries("device inventory analysis", 5)
	searchTime := time.Since(searchStart)

	if err != nil {
		fmt.Printf("   ❌ Search test failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Search test passed: Found %d results in %v\n", len(results), searchTime)
		if len(results) > 0 {
			fmt.Printf("   📋 Top result: %s\n", results[0].Path)
		}
	}

	// Cache information
	cacheFile := "spec/nqe-embeddings.json"
	if info, err := os.Stat(cacheFile); err == nil {
		fmt.Printf("\n💾 Cache saved: %s (%.2f MB)\n", cacheFile, float64(info.Size())/(1024*1024))
	}

	fmt.Printf("\n✨ Ready for production use!\n")
	fmt.Printf("💡 Run 'make embedding-status' to see detailed statistics\n")
	fmt.Printf("🚀 Your AI-powered query search is now optimized!\n")
}
