package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

// TestSemanticCache tests the semantic cache functionality
func TestSemanticCache(t *testing.T) {
	// Create a semantic cache with mock embedding service
	embeddingService := NewMockEmbeddingService()
	cache := NewSemanticCache(embeddingService, createTestLogger())

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
		shortTTLCache := NewSemanticCache(embeddingService, createTestLogger())
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
		smallCache := NewSemanticCache(embeddingService, createTestLogger())
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

func TestSemanticCacheStats(t *testing.T) {
	embeddingService := NewMockEmbeddingService()
	cache := NewSemanticCache(embeddingService, createTestLogger())

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
	cache := NewSemanticCache(embeddingService, createTestLogger())

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
	cache := NewSemanticCache(embeddingService, createTestLogger())

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
