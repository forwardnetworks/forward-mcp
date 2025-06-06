package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
)

func main() {
	fmt.Println("⚡ Forward Networks MCP - Search Performance Benchmark")
	fmt.Println("=====================================================")

	// Initialize logger
	logger := logger.New()

	// Initialize keyword embedding service for fast benchmarking
	embeddingService := service.NewKeywordEmbeddingService()
	queryIndex := service.NewNQEQueryIndex(embeddingService, logger)

	// Load queries
	fmt.Printf("📖 Loading queries...")
	if err := queryIndex.LoadFromSpec(); err != nil {
		fmt.Printf(" ❌ Failed: %v\n", err)
		return
	}
	fmt.Printf(" ✅ Done\n")

	// Benchmark queries
	benchmarkQueries := []string{
		"device inventory",
		"bgp routing analysis",
		"security configuration",
		"interface status",
		"network topology",
		"hardware information",
		"protocol analysis",
		"configuration management",
		"device monitoring",
		"route analysis",
	}

	fmt.Printf("\n🔍 Running search benchmarks...\n")
	fmt.Printf("Query: 'search term' → results (time)\n")
	fmt.Printf("=====================================\n")

	var times []time.Duration
	totalResults := 0

	for _, query := range benchmarkQueries {
		start := time.Now()
		results, err := queryIndex.SearchQueries(query, 10)
		duration := time.Since(start)
		times = append(times, duration)

		if err != nil {
			fmt.Printf("❌ '%s' → Error: %v\n", query, err)
		} else {
			totalResults += len(results)
			fmt.Printf("✅ '%s' → %d results (%v)\n", query, len(results), duration)
		}
	}

	// Calculate statistics
	fmt.Printf("\n📊 Performance Statistics:\n")
	fmt.Printf("=========================\n")

	if len(times) > 0 {
		// Calculate average
		var total time.Duration
		for _, t := range times {
			total += t
		}
		average := total / time.Duration(len(times))

		// Sort for median and percentiles
		sort.Slice(times, func(i, j int) bool {
			return times[i] < times[j]
		})

		median := times[len(times)/2]
		p95 := times[int(float64(len(times))*0.95)]
		min := times[0]
		max := times[len(times)-1]

		fmt.Printf("📈 Search Times:\n")
		fmt.Printf("   ⚡ Minimum: %v\n", min)
		fmt.Printf("   📊 Average: %v\n", average)
		fmt.Printf("   📊 Median:  %v\n", median)
		fmt.Printf("   📊 95th percentile: %v\n", p95)
		fmt.Printf("   📊 Maximum: %v\n", max)

		fmt.Printf("\n📋 Results:\n")
		fmt.Printf("   📊 Total results found: %d\n", totalResults)
		fmt.Printf("   📊 Average results per query: %.1f\n", float64(totalResults)/float64(len(benchmarkQueries)))

		// Performance assessment
		fmt.Printf("\n🎯 Performance Assessment:\n")
		if average < time.Millisecond {
			fmt.Printf("   🏆 Excellent! Sub-millisecond average (%.0fµs)\n", float64(average.Nanoseconds())/1000)
			fmt.Printf("   🚀 This meets the ACHIEVEMENTS.md performance target!\n")
		} else if average < 10*time.Millisecond {
			fmt.Printf("   ✅ Good performance (%.1fms average)\n", float64(average.Nanoseconds())/1000000)
		} else if average < 100*time.Millisecond {
			fmt.Printf("   🟡 Acceptable performance (%.1fms average)\n", float64(average.Nanoseconds())/1000000)
		} else {
			fmt.Printf("   🔴 Slow performance (%.1fms average)\n", float64(average.Nanoseconds())/1000000)
			fmt.Printf("   💡 Consider optimizing embeddings or search algorithm\n")
		}

		// Throughput calculation
		queriesPerSecond := float64(len(benchmarkQueries)) / total.Seconds()
		fmt.Printf("   📊 Throughput: %.0f queries/second\n", queriesPerSecond)

		// Consistency check
		coefficient := calculateCoefficient(times)
		if coefficient < 0.2 {
			fmt.Printf("   ✅ Very consistent performance (CV: %.3f)\n", coefficient)
		} else if coefficient < 0.5 {
			fmt.Printf("   🟡 Moderately consistent performance (CV: %.3f)\n", coefficient)
		} else {
			fmt.Printf("   🔴 Inconsistent performance (CV: %.3f)\n", coefficient)
		}
	}

	// Test with different result limits
	fmt.Printf("\n🔍 Limit Impact Test:\n")
	testQuery := "device configuration analysis"
	limits := []int{1, 5, 10, 25, 50}

	for _, limit := range limits {
		start := time.Now()
		results, err := queryIndex.SearchQueries(testQuery, limit)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Limit %d: Error - %v\n", limit, err)
		} else {
			fmt.Printf("   📊 Limit %d: %d results in %v\n", limit, len(results), duration)
		}
	}

	fmt.Printf("\n🎉 Benchmark complete!\n")
	fmt.Printf("💡 Run 'make embedding-status' for overall system health\n")
}

// calculateCoefficient calculates the coefficient of variation for consistency measurement
func calculateCoefficient(times []time.Duration) float64 {
	if len(times) == 0 {
		return 0
	}

	// Convert to float64 nanoseconds for calculation
	values := make([]float64, len(times))
	var sum float64
	for i, t := range times {
		values[i] = float64(t.Nanoseconds())
		sum += values[i]
	}

	mean := sum / float64(len(values))

	// Calculate standard deviation
	var sumSquaredDiff float64
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}

	variance := sumSquaredDiff / float64(len(values))
	stdDev := math.Sqrt(variance)

	// Coefficient of variation = stdDev / mean
	if mean == 0 {
		return 0
	}
	return stdDev / mean
}
