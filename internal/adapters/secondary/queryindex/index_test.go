package queryindex

import (
	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"testing"

	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/ports"
)

func TestSearchQueries_MetadataFiltering(t *testing.T) {
	// Create a mock embedding service for testing
	mockEmbeddingService := embeddings.NewMockEmbeddingService()
	log := logger.New()

	idx := NewNQEQueryIndex(mockEmbeddingService, log)

	// Generate embeddings that will have high similarity with the mock service
	// The mock service returns embeddings based on text hash, so we'll use the same text
	searchEmbedding, _ := mockEmbeddingService.GenerateEmbedding("routes")

	// Convert to float32 for the test queries
	embedding1 := make([]float32, len(searchEmbedding))
	embedding2 := make([]float32, len(searchEmbedding))
	embedding3 := make([]float32, len(searchEmbedding))
	embedding4 := make([]float32, len(searchEmbedding))
	embedding5 := make([]float32, len(searchEmbedding))

	for i, v := range searchEmbedding {
		embedding1[i] = float32(v)
		embedding2[i] = float32(v) * 0.9  // Slightly different
		embedding3[i] = float32(v) * 0.8  // More different
		embedding4[i] = float32(v) * 0.95 // Very similar
		embedding5[i] = float32(v) * 0.85 // Somewhat different
	}

	idx.queries = []*ports.NQEQueryIndexEntry{
		{QueryID: "1", Intent: "Show all routes", Description: "Returns all routes in the routing table for each device.", Embedding: embedding1},
		{QueryID: "2", Intent: "", Description: "", Embedding: embedding2},      // Should be ignored
		{QueryID: "3", Intent: "Short", Description: "", Embedding: embedding3}, // Should be ignored
		{QueryID: "4", Intent: "Count routes", Description: "Count the number of routes per device.", Embedding: embedding4},
		{QueryID: "5", Intent: "", Description: "A valid description with enough length.", Embedding: embedding5},
	}

	results, err := idx.SearchQueries("routes", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results (all queries with embeddings), got %d", len(results))
	}
	// Verify all queries with embeddings are returned (metadata filtering was removed)
	expectedQueryIDs := map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true}
	for _, r := range results {
		if !expectedQueryIDs[r.QueryID] {
			t.Errorf("unexpected query ID in results: %s", r.QueryID)
		}
	}
}

// A query without an embedding is still found by BM25; the query that matches
// on both rankings comes first.
func TestSearchQueries_HybridKeepsUnembeddedQueries(t *testing.T) {
	mockEmbeddingService := embeddings.NewMockEmbeddingService()
	log := logger.New()

	idx := NewNQEQueryIndex(mockEmbeddingService, log)

	// Generate compatible embeddings
	searchEmbedding, _ := mockEmbeddingService.GenerateEmbedding("routes")
	embedding1 := make([]float32, len(searchEmbedding))
	for i, v := range searchEmbedding {
		embedding1[i] = float32(v)
	}

	idx.queries = []*ports.NQEQueryIndexEntry{
		{QueryID: "1", Intent: "Show all routes", Description: "Returns all routes in the routing table for each device.", Embedding: embedding1},
		{QueryID: "2", Intent: "Show all routes", Description: "Returns all routes in the routing table for each device.", Embedding: nil}, // No embedding
	}

	results, err := idx.SearchQueries("routes", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected both queries, got %d", len(results))
	}
	if results[0].QueryID != "1" || results[0].MatchType != "hybrid" {
		t.Errorf("first = %s/%s, want 1/hybrid", results[0].QueryID, results[0].MatchType)
	}
	if results[1].QueryID != "2" || results[1].MatchType != "bm25" {
		t.Errorf("second = %s/%s, want 2/bm25", results[1].QueryID, results[1].MatchType)
	}
}

func TestSearchQueries_KeywordFallback(t *testing.T) {
	mockEmbeddingService := embeddings.NewMockEmbeddingService()
	log := logger.New()

	idx := NewNQEQueryIndex(mockEmbeddingService, log)
	idx.queries = []*ports.NQEQueryIndexEntry{
		{QueryID: "1", Intent: "Show all routes", Description: "Returns all routes in the routing table for each device.", Embedding: nil},
		{QueryID: "2", Intent: "Count routes", Description: "Count the number of routes per device.", Embedding: nil},
	}

	results, err := idx.SearchQueries("routes", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results from keyword fallback, got %d", len(results))
	}
}

// BenchmarkGenerateEmbeddings measures keyword embedding generation performance
func BenchmarkGenerateEmbeddings(b *testing.B) {
	// Use keyword embedder (what auto-hydration actually uses)
	embedder := embeddings.New("keyword", "", logger.New())
	log := logger.New()

	// Create index with queries without embeddings
	idx := NewNQEQueryIndex(embedder, log)
	idx.queries = make([]*ports.NQEQueryIndexEntry, 100)
	for i := 0; i < 100; i++ {
		idx.queries[i] = &ports.NQEQueryIndexEntry{
			QueryID:     string(rune('A' + i)),
			Intent:      "Test query intent for benchmarking",
			Description: "Test query description for benchmarking performance of embedding generation",
			Embedding:   nil, // No embedding yet
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.GenerateEmbeddings(); err != nil {
			b.Fatalf("Failed to generate embeddings: %v", err)
		}
	}
}

// BenchmarkGenerateEmbeddings_Large measures embedding generation for a large query set (~1800 queries)
func BenchmarkGenerateEmbeddings_Large(b *testing.B) {
	// Use keyword embedder (what auto-hydration actually uses)
	embedder := embeddings.New("keyword", "", logger.New())
	log := logger.New()

	// Create index with realistic number of queries
	idx := NewNQEQueryIndex(embedder, log)
	idx.queries = make([]*ports.NQEQueryIndexEntry, 1800)
	for i := 0; i < 1800; i++ {
		idx.queries[i] = &ports.NQEQueryIndexEntry{
			QueryID:     string(rune(i)),
			Intent:      "Network query intent for device configuration analysis",
			Description: "Detailed description of network query for benchmarking embedding generation performance with realistic data",
			Embedding:   nil,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.GenerateEmbeddings(); err != nil {
			b.Fatalf("Failed to generate embeddings: %v", err)
		}
	}
}

// BenchmarkLoadFromQueries measures the time to load queries into the index
func BenchmarkLoadFromQueries(b *testing.B) {
	mockEmbeddingService := embeddings.NewMockEmbeddingService()
	log := logger.New()

	// Create test queries
	queries := make([]ports.NQEQueryDetail, 1800)
	for i := 0; i < 1800; i++ {
		queries[i] = ports.NQEQueryDetail{
			QueryID: string(rune(i)),
			Path:    "/Test/Query/Path",
			Intent:  "Network query intent",
			LastCommit: ports.NQECommitInfo{
				ID: "abc123",
			},
		}
	}

	idx := NewNQEQueryIndex(mockEmbeddingService, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.LoadFromQueries(queries); err != nil {
			b.Fatalf("Failed to load queries: %v", err)
		}
	}
}

// BenchmarkSearchQueries_BM25 measures BM25 search performance
func BenchmarkSearchQueries_BM25(b *testing.B) {
	mockEmbeddingService := embeddings.NewMockEmbeddingService()
	log := logger.New()

	idx := NewNQEQueryIndex(mockEmbeddingService, log)

	// Create realistic query set without embeddings (BM25 only)
	idx.queries = make([]*ports.NQEQueryIndexEntry, 1800)
	for i := 0; i < 1800; i++ {
		idx.queries[i] = &ports.NQEQueryIndexEntry{
			QueryID:     string(rune(i)),
			Intent:      "Show BGP routing information for network devices",
			Description: "Returns detailed BGP routing table information including routes, neighbors, and peer status",
			Embedding:   nil,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := idx.SearchQueries("BGP routing", 10)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// BenchmarkSearchQueries_Hybrid measures hybrid (BM25 + embedding) search performance
func BenchmarkSearchQueries_Hybrid(b *testing.B) {
	mockEmbeddingService := embeddings.NewMockEmbeddingService()
	log := logger.New()

	idx := NewNQEQueryIndex(mockEmbeddingService, log)

	// Create realistic query set with embeddings
	idx.queries = make([]*ports.NQEQueryIndexEntry, 1800)
	for i := 0; i < 1800; i++ {
		embedding, _ := mockEmbeddingService.GenerateEmbedding("BGP routing information")
		embeddingFloat32 := make([]float32, len(embedding))
		for j, v := range embedding {
			embeddingFloat32[j] = float32(v)
		}

		idx.queries[i] = &ports.NQEQueryIndexEntry{
			QueryID:     string(rune(i)),
			Intent:      "Show BGP routing information for network devices",
			Description: "Returns detailed BGP routing table information including routes, neighbors, and peer status",
			Embedding:   embeddingFloat32,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := idx.SearchQueries("BGP routing", 10)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}
