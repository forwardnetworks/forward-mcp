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

func TestSearchQueries_EmbeddingPreferred(t *testing.T) {
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
	if len(results) != 1 {
		t.Errorf("expected 1 result with embedding, got %d", len(results))
	}
	if len(results) > 0 && results[0].QueryID != "1" {
		t.Errorf("expected QueryID 1, got %s", results[0].QueryID)
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
