package embeddings

import (
	"testing"
)

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
