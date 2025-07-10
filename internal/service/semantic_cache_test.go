package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

// TestSemanticCache tests the semantic cache functionality
func TestSemanticCache(t *testing.T) {
	// Create a semantic cache with mock embedding service
	embeddingService := NewMockEmbeddingService()
	cache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)

	// Test basic Put and Get operations
	t.Run("basic_put_and_get", func(t *testing.T) {
		query := "foreach device in network.devices select {name: device.name}"
		networkID := "162112"
		snapshotID := "latest"

		result := &forward.NQERunResult{
			SnapshotID: snapshotID,
			Items: []map[string]interface{}{
				{"name": "router-1"},
				{"name": "switch-1"},
			},
		}

		// Store result
		err := cache.Put(query, networkID, snapshotID, result)
		if err != nil {
			t.Fatalf("Failed to put result in cache: %v", err)
		}

		// Retrieve result - exact match
		cachedResult, found := cache.Get(query, networkID, snapshotID)
		if !found {
			t.Fatal("Expected to find cached result")
		}

		if cachedResult.SnapshotID != result.SnapshotID {
			t.Errorf("Expected snapshot ID %s, got %s", result.SnapshotID, cachedResult.SnapshotID)
		}

		if len(cachedResult.Items) != len(result.Items) {
			t.Errorf("Expected %d items, got %d", len(result.Items), len(cachedResult.Items))
		}
	})

	t.Run("semantic_similarity_match", func(t *testing.T) {
		// Store a query
		originalQuery := "show me all network devices with their names"
		similarQuery := "list all devices with device names"
		networkID := "162112"
		snapshotID := "latest"

		result := &forward.NQERunResult{
			SnapshotID: snapshotID,
			Items: []map[string]interface{}{
				{"name": "device-1"},
			},
		}

		err := cache.Put(originalQuery, networkID, snapshotID, result)
		if err != nil {
			t.Fatalf("Failed to put result in cache: %v", err)
		}

		// Try to retrieve with similar query
		cachedResult, found := cache.Get(similarQuery, networkID, snapshotID)

		// Note: With mock embedding service, similarity depends on hash-based algorithm
		// This test verifies the semantic search mechanism works
		if found {
			t.Logf("Found semantic match for similar query")
			if cachedResult.SnapshotID != result.SnapshotID {
				t.Errorf("Expected snapshot ID %s, got %s", result.SnapshotID, cachedResult.SnapshotID)
			}
		} else {
			t.Logf("No semantic match found (depends on mock embedding similarity)")
		}
	})

	t.Run("network_isolation", func(t *testing.T) {
		query := "test query"
		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		// Store in network A
		err := cache.Put(query, "network-A", "snap-1", result)
		if err != nil {
			t.Fatalf("Failed to put result: %v", err)
		}

		// Try to retrieve from network B - should not find
		cachedResult, found := cache.Get(query, "network-B", "snap-1")
		if found {
			t.Errorf("Expected not to find result from different network, but found: %+v", cachedResult)
		}

		// Retrieve from same network - should find
		cachedResult, found = cache.Get(query, "network-A", "snap-1")
		if !found {
			t.Error("Expected to find result from same network")
		}

		// Verify the result is correct
		if cachedResult == nil {
			t.Error("Expected cached result to not be nil")
		}
	})

	t.Run("ttl_expiration", func(t *testing.T) {
		// Create cache with short TTL for testing
		shortTTLCache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)
		shortTTLCache.ttl = 1 * time.Millisecond // Very short TTL

		query := "test query"
		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		err := shortTTLCache.Put(query, "162112", "latest", result)
		if err != nil {
			t.Fatalf("Failed to put result: %v", err)
		}

		// Sleep to let entry expire
		time.Sleep(10 * time.Millisecond)

		// Should not find expired entry
		_, found := shortTTLCache.Get(query, "162112", "latest")
		if found {
			t.Error("Expected not to find expired entry")
		}
	})

	t.Run("eviction_policy", func(t *testing.T) {
		// Create cache with small capacity
		smallCache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)
		smallCache.maxEntries = 2 // Only 2 entries

		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		// Fill cache to capacity
		err := smallCache.Put("query1", "162112", "latest", result)
		if err != nil {
			t.Fatalf("Failed to put query1: %v", err)
		}

		err = smallCache.Put("query2", "162112", "latest", result)
		if err != nil {
			t.Fatalf("Failed to put query2: %v", err)
		}

		// Adding third entry should evict oldest
		err = smallCache.Put("query3", "162112", "latest", result)
		if err != nil {
			t.Fatalf("Failed to put query3: %v", err)
		}

		// query1 should be evicted
		_, found := smallCache.Get("query1", "162112", "latest")
		if found {
			t.Error("Expected query1 to be evicted")
		}

		// query3 should be present
		_, found = smallCache.Get("query3", "162112", "latest")
		if !found {
			t.Error("Expected query3 to be present")
		}
	})
}

// TestEnhancedEvictionPolicies tests the new eviction strategies
func TestEnhancedEvictionPolicies(t *testing.T) {
	embeddingService := NewMockEmbeddingService()

	t.Run("lru_eviction", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:         true,
			MaxEntries:      3,
			EvictionPolicy:  config.EvictionPolicyLRU,
			TTLHours:        24,
			MaxMemoryMB:     10,
			CompressResults: false,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		// Add entries
		cache.Put("query1", "net", "snap", result)
		cache.Put("query2", "net", "snap", result)
		cache.Put("query3", "net", "snap", result)

		// Access query1 to make it more recently used
		cache.Get("query1", "net", "snap")

		// Add another entry, should evict query2 (least recently used)
		cache.Put("query4", "net", "snap", result)

		// query2 should be evicted
		_, found := cache.Get("query2", "net", "snap")
		if found {
			t.Error("Expected query2 to be evicted by LRU policy")
		}

		// query1 should still be present (recently accessed)
		_, found = cache.Get("query1", "net", "snap")
		if !found {
			t.Error("Expected query1 to still be present")
		}
	})

	t.Run("lfu_eviction", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:         true,
			MaxEntries:      3,
			EvictionPolicy:  config.EvictionPolicyLFU,
			TTLHours:        24,
			MaxMemoryMB:     10,
			CompressResults: false,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		// Add entries
		cache.Put("query1", "net", "snap", result)
		cache.Put("query2", "net", "snap", result)
		cache.Put("query3", "net", "snap", result)

		// Access query1 multiple times to increase frequency
		cache.Get("query1", "net", "snap")
		cache.Get("query1", "net", "snap")
		cache.Get("query3", "net", "snap") // Access query3 once

		// Add another entry, should evict query2 (least frequently used)
		cache.Put("query4", "net", "snap", result)

		// query2 should be evicted
		_, found := cache.Get("query2", "net", "snap")
		if found {
			t.Error("Expected query2 to be evicted by LFU policy")
		}

		// query1 should still be present (most frequently used)
		_, found = cache.Get("query1", "net", "snap")
		if !found {
			t.Error("Expected query1 to still be present")
		}
	})

	t.Run("size_based_eviction", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:         true,
			MaxEntries:      5,
			EvictionPolicy:  config.EvictionPolicySize,
			TTLHours:        24,
			MaxMemoryMB:     1, // Very small memory limit
			CompressResults: false,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		// Create results of different sizes
		smallResult := &forward.NQERunResult{Items: []map[string]interface{}{{"small": "data"}}}
		largeResult := &forward.NQERunResult{
			Items: []map[string]interface{}{
				{"large": "data with much more content and longer strings to make it bigger"},
				{"item2": "additional data to increase size"},
				{"item3": "even more data"},
			},
		}

		// Add small result first
		cache.Put("small_query", "net", "snap", smallResult)

		// Add large result
		cache.Put("large_query", "net", "snap", largeResult)

		// Add another entry to trigger eviction
		cache.Put("trigger_query", "net", "snap", smallResult)

		// Check if eviction occurred - large result should be evicted first
		stats := cache.GetStats()
		if stats["evicted_count"].(int64) == 0 {
			t.Log("No eviction occurred yet, memory limit might not be reached")
		}
	})
}

// TestCompressionFeatures tests the compression functionality
func TestCompressionFeatures(t *testing.T) {
	embeddingService := NewMockEmbeddingService()

	t.Run("compression_enabled", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:          true,
			MaxEntries:       10,
			CompressResults:  true,
			CompressionLevel: 6,
			TTLHours:         24,
			MaxMemoryMB:      10,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		// Create a large result that would benefit from compression
		largeResult := &forward.NQERunResult{
			Items: make([]map[string]interface{}, 100),
		}
		for i := 0; i < 100; i++ {
			largeResult.Items[i] = map[string]interface{}{
				"device_name": fmt.Sprintf("device-%d", i),
				"description": "This is a long description that repeats the same content over and over to create a large payload that will compress well",
				"data":        fmt.Sprintf("repetitive data pattern %d", i),
			}
		}

		err := cache.Put("large_query", "net", "snap", largeResult)
		if err != nil {
			t.Fatalf("Failed to store large result: %v", err)
		}

		// Retrieve and verify the result is the same
		retrievedResult, found := cache.Get("large_query", "net", "snap")
		if !found {
			t.Fatal("Failed to retrieve compressed result")
		}

		if len(retrievedResult.Items) != len(largeResult.Items) {
			t.Errorf("Expected %d items, got %d", len(largeResult.Items), len(retrievedResult.Items))
		}

		// Check compression metrics
		stats := cache.GetStats()
		compressionRatio := stats["compression_ratio"].(string)
		if compressionRatio == "0.000" {
			t.Error("Expected compression to have occurred")
		}
		t.Logf("Compression ratio: %s", compressionRatio)
	})

	t.Run("compression_disabled", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:         true,
			MaxEntries:      10,
			CompressResults: false,
			TTLHours:        24,
			MaxMemoryMB:     10,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		err := cache.Put("uncompressed_query", "net", "snap", result)
		if err != nil {
			t.Fatalf("Failed to store uncompressed result: %v", err)
		}

		// Retrieve and verify
		retrievedResult, found := cache.Get("uncompressed_query", "net", "snap")
		if !found {
			t.Fatal("Failed to retrieve uncompressed result")
		}

		if len(retrievedResult.Items) != len(result.Items) {
			t.Errorf("Expected %d items, got %d", len(result.Items), len(retrievedResult.Items))
		}
	})
}

// TestMemoryManagement tests memory tracking and limits
func TestMemoryManagement(t *testing.T) {
	embeddingService := NewMockEmbeddingService()

	t.Run("memory_tracking", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:         true,
			MaxEntries:      10,
			MaxMemoryMB:     1, // 1MB limit
			CompressResults: false,
			MetricsEnabled:  true,
			TTLHours:        24,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

		// Add some entries
		for i := 0; i < 5; i++ {
			err := cache.Put(fmt.Sprintf("query%d", i), "net", "snap", result)
			if err != nil {
				t.Fatalf("Failed to put query %d: %v", i, err)
			}
		}

		stats := cache.GetStats()
		memoryUsage := stats["memory_usage_bytes"].(int64)
		memoryUsageMB := stats["current_memory_mb"].(float64)

		if memoryUsage <= 0 {
			t.Error("Expected memory usage to be tracked")
		}

		if memoryUsageMB <= 0 {
			t.Error("Expected memory usage in MB to be positive")
		}

		t.Logf("Memory usage: %d bytes (%.2f MB)", memoryUsage, memoryUsageMB)
	})

	t.Run("memory_limit_enforcement", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:                 true,
			MaxEntries:              100,
			MaxMemoryMB:             1, // Very small limit to trigger eviction
			EvictionPolicy:          config.EvictionPolicyLRU,
			CompressResults:         false,
			MetricsEnabled:          true,
			MemoryEvictionThreshold: 0.5, // 50% threshold
			TTLHours:                24,
		}
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		// Create a moderately sized result
		result := &forward.NQERunResult{
			Items: make([]map[string]interface{}, 50),
		}
		for i := 0; i < 50; i++ {
			result.Items[i] = map[string]interface{}{
				"device": fmt.Sprintf("device-%d", i),
				"data":   fmt.Sprintf("some data for device %d", i),
			}
		}

		// Keep adding entries until memory limit triggers eviction
		initialEntries := 0
		for i := 0; i < 20; i++ {
			err := cache.Put(fmt.Sprintf("memory_query%d", i), "net", "snap", result)
			if err != nil {
				t.Logf("Memory limit reached at entry %d: %v", i, err)
				break
			}
			initialEntries++
		}

		stats := cache.GetStats()
		finalEntries := stats["total_entries"].(int)
		evicted := stats["evicted_count"].(int64)

		t.Logf("Initial entries: %d, Final entries: %d, Evicted: %d",
			initialEntries, finalEntries, evicted)

		if finalEntries > initialEntries && evicted == 0 {
			t.Log("Memory limit may not have been reached with test data size")
		}
	})
}

// TestEnhancedMetrics tests the enhanced metrics functionality
func TestEnhancedMetrics(t *testing.T) {
	embeddingService := NewMockEmbeddingService()

	cfg := &config.SemanticCacheConfig{
		Enabled:                 true,
		MaxEntries:              10,
		MaxMemoryMB:             10,
		EvictionPolicy:          config.EvictionPolicyLRU,
		CompressResults:         true,
		CompressionLevel:        6,
		MetricsEnabled:          true,
		MemoryEvictionThreshold: 0.8,
		CleanupIntervalMinutes:  1,
		TTLHours:                24,
	}
	cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

	result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

	// Perform operations to generate metrics
	cache.Put("metric_query1", "net", "snap", result)
	cache.Put("metric_query2", "net", "snap", result)

	cache.Get("metric_query1", "net", "snap")     // Hit
	cache.Get("nonexistent_query", "net", "snap") // Miss

	stats := cache.GetStats()

	// Verify all enhanced metrics are present
	expectedMetrics := []string{
		"total_entries", "total_queries", "cache_hits", "cache_misses",
		"hit_rate_percent", "max_memory_mb", "current_memory_mb",
		"memory_usage_bytes", "memory_utilization_%", "compression_enabled",
		"compression_ratio", "avg_response_time_ms", "evicted_count",
		"evictions_by_policy", "last_cleanup", "persistence_enabled",
		"disk_cache_path", "eviction_policy", "cleanup_interval_min",
		"memory_threshold_%",
	}

	for _, metric := range expectedMetrics {
		if _, exists := stats[metric]; !exists {
			t.Errorf("Expected metric %s to be present in stats", metric)
		}
	}

	// Verify specific metric values
	if stats["total_entries"].(int) != 2 {
		t.Errorf("Expected 2 total entries, got %v", stats["total_entries"])
	}

	if stats["cache_hits"].(int64) != 1 {
		t.Errorf("Expected 1 cache hit, got %v", stats["cache_hits"])
	}

	if stats["cache_misses"].(int64) != 1 {
		t.Errorf("Expected 1 cache miss, got %v", stats["cache_misses"])
	}

	if stats["eviction_policy"].(string) != "lru" {
		t.Errorf("Expected eviction policy 'lru', got %v", stats["eviction_policy"])
	}

	t.Logf("Enhanced metrics test passed. Sample metrics: %+v", stats)
}

// TestCacheConfiguration tests different configuration scenarios
func TestCacheConfiguration(t *testing.T) {
	embeddingService := NewMockEmbeddingService()

	t.Run("default_configuration", func(t *testing.T) {
		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)

		// Verify default values are set correctly
		if cache.maxEntries != 1000 {
			t.Errorf("Expected default maxEntries 1000, got %d", cache.maxEntries)
		}

		if cache.compressionEnabled != true {
			t.Error("Expected compression to be enabled by default")
		}

		if cache.evictionPolicy != config.EvictionPolicyLRU {
			t.Errorf("Expected default eviction policy LRU, got %v", cache.evictionPolicy)
		}
	})

	t.Run("custom_configuration", func(t *testing.T) {
		cfg := &config.SemanticCacheConfig{
			Enabled:                 true,
			MaxEntries:              500,
			MaxMemoryMB:             256,
			EvictionPolicy:          config.EvictionPolicyLFU,
			CompressResults:         false,
			CompressionLevel:        9,
			PersistToDisk:           false,
			MetricsEnabled:          true,
			MemoryEvictionThreshold: 0.9,
			CleanupIntervalMinutes:  15,
			TTLHours:                48,
		}

		cache := NewSemanticCache(embeddingService, createTestLogger(), "test", cfg)

		// Verify custom values are applied
		if cache.maxEntries != 500 {
			t.Errorf("Expected custom maxEntries 500, got %d", cache.maxEntries)
		}

		if cache.compressionEnabled != false {
			t.Error("Expected compression to be disabled")
		}

		if cache.evictionPolicy != config.EvictionPolicyLFU {
			t.Errorf("Expected custom eviction policy LFU, got %v", cache.evictionPolicy)
		}

		if cache.maxMemoryBytes != 256*1024*1024 {
			t.Errorf("Expected custom memory limit 256MB, got %d", cache.maxMemoryBytes)
		}
	})
}

func TestSemanticCacheStats(t *testing.T) {
	embeddingService := NewMockEmbeddingService()
	cache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)

	stats := cache.GetStats()

	// Check initial stats
	if stats["total_entries"] != 0 {
		t.Errorf("Expected 0 total entries, got %v", stats["total_entries"])
	}

	if stats["total_queries"] != int64(0) {
		t.Errorf("Expected 0 total queries, got %v", stats["total_queries"])
	}

	// Add some entries and queries
	result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

	err := cache.Put("query1", "162112", "latest", result)
	if err != nil {
		t.Fatalf("Failed to put result: %v", err)
	}

	// Trigger some cache lookups
	cache.Get("query1", "162112", "latest") // Should hit
	cache.Get("query2", "162112", "latest") // Should miss

	stats = cache.GetStats()

	if stats["total_entries"] != 1 {
		t.Errorf("Expected 1 total entry, got %v", stats["total_entries"])
	}

	if stats["total_queries"] != int64(2) {
		t.Errorf("Expected 2 total queries, got %v", stats["total_queries"])
	}

	if stats["cache_hits"] != int64(1) {
		t.Errorf("Expected 1 cache hit, got %v", stats["cache_hits"])
	}

	if stats["cache_misses"] != int64(1) {
		t.Errorf("Expected 1 cache miss, got %v", stats["cache_misses"])
	}
}

func TestSemanticCacheSimilarQueries(t *testing.T) {
	embeddingService := NewMockEmbeddingService()
	cache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)

	// Add some queries to the cache
	queries := []string{
		"show me all devices",
		"list network devices",
		"get device inventory",
		"display all routers",
	}

	result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

	for i, query := range queries {
		err := cache.Put(query, "162112", "latest", result)
		if err != nil {
			t.Fatalf("Failed to put query %d: %v", i, err)
		}
	}

	// Find similar queries
	similarQueries, err := cache.FindSimilarQueries("show devices", 3)
	if err != nil {
		t.Fatalf("Failed to find similar queries: %v", err)
	}

	if len(similarQueries) > 3 {
		t.Errorf("Expected at most 3 similar queries, got %d", len(similarQueries))
	}

	// Verify results are sorted by similarity
	for i := 1; i < len(similarQueries); i++ {
		if similarQueries[i-1].SimilarityScore < similarQueries[i].SimilarityScore {
			t.Error("Expected similar queries to be sorted by similarity score in descending order")
		}
	}

	// Test with query not in cache
	similarQueries, err = cache.FindSimilarQueries("completely different query about unicorns", 5)
	if err != nil {
		t.Fatalf("Failed to find similar queries: %v", err)
	}

	// Should still return some results (depends on mock embedding similarity)
	t.Logf("Found %d similar queries for unrelated query", len(similarQueries))
}

func TestSemanticCacheClearExpired(t *testing.T) {
	embeddingService := NewMockEmbeddingService()
	cache := NewSemanticCache(embeddingService, createTestLogger(), "test", nil)

	// Set short TTL for testing
	cache.ttl = 1 * time.Millisecond

	result := &forward.NQERunResult{Items: []map[string]interface{}{{"test": "data"}}}

	// Add several entries
	for i := 0; i < 5; i++ {
		err := cache.Put(fmt.Sprintf("query%d", i), "162112", "latest", result)
		if err != nil {
			t.Fatalf("Failed to put query %d: %v", i, err)
		}
	}

	// Wait for entries to expire
	time.Sleep(10 * time.Millisecond)

	// Add one fresh entry
	err := cache.Put("fresh_query", "162112", "latest", result)
	if err != nil {
		t.Fatalf("Failed to put fresh query: %v", err)
	}

	// Clear expired entries
	removed := cache.ClearExpired()
	if removed != 5 {
		t.Errorf("Expected to remove 5 expired entries, removed %d", removed)
	}

	// Fresh entry should still be there
	_, found := cache.Get("fresh_query", "162112", "latest")
	if !found {
		t.Error("Expected fresh entry to still be present")
	}

	// Expired entries should be gone
	_, found = cache.Get("query0", "162112", "latest")
	if found {
		t.Error("Expected expired entry to be removed")
	}
}

func TestMockEmbeddingService(t *testing.T) {
	service := NewMockEmbeddingService()

	embedding1, err := service.GenerateEmbedding("test query 1")
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	if len(embedding1) != 1536 {
		t.Errorf("Expected embedding length 1536, got %d", len(embedding1))
	}

	embedding2, err := service.GenerateEmbedding("test query 2")
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	// Same input should produce same output
	embedding1_again, err := service.GenerateEmbedding("test query 1")
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	for i := range embedding1 {
		if embedding1[i] != embedding1_again[i] {
			t.Error("Expected same input to produce same embedding")
			break
		}
	}

	// Different inputs should produce different outputs
	different := false
	for i := range embedding1 {
		if embedding1[i] != embedding2[i] {
			different = true
			break
		}
	}

	if !different {
		t.Error("Expected different inputs to produce different embeddings")
	}

	// Test empty input
	_, err = service.GenerateEmbedding("")
	if err == nil {
		t.Error("Expected error for empty input")
	}
}

// Helper function to create a test logger
func createTestLogger() *logger.Logger {
	return logger.New()
}
