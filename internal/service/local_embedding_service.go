package service

import (
	"fmt"
	"math"
	"strings"
)

// LocalEmbeddingService implements simple TF-IDF based embeddings
type LocalEmbeddingService struct {
	vocabulary map[string]int
	idfScores  map[string]float64
	documents  []string
}

// NewLocalEmbeddingService creates a simple local embedding service
func NewLocalEmbeddingService() *LocalEmbeddingService {
	return &LocalEmbeddingService{
		vocabulary: make(map[string]int),
		idfScores:  make(map[string]float64),
		documents:  make([]string, 0),
	}
}

// GenerateEmbedding creates a simple TF-IDF vector for the input text
func (les *LocalEmbeddingService) GenerateEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text provided")
	}

	// Tokenize and normalize text
	tokens := les.tokenize(text)

	// Create term frequency map
	tf := make(map[string]float64)
	for _, token := range tokens {
		tf[token]++
	}

	// Normalize by document length
	for token := range tf {
		tf[token] = tf[token] / float64(len(tokens))
	}

	// Create a fixed-size embedding vector (dimension 100 for simplicity)
	embeddingDim := 100
	embedding := make([]float64, embeddingDim)

	// Use hash-based mapping to convert tokens to vector positions
	for token, tfScore := range tf {
		// Simple hash to map token to embedding dimensions
		positions := les.hashToken(token, embeddingDim)
		weight := tfScore * les.getIDF(token)

		for _, pos := range positions {
			embedding[pos] += weight
		}
	}

	// Normalize the embedding vector
	embedding = les.normalizeVector(embedding)

	return embedding, nil
}

// tokenize splits text into lowercase tokens
func (les *LocalEmbeddingService) tokenize(text string) []string {
	// Simple tokenization: lowercase, split on whitespace and punctuation
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "/", " ")
	text = strings.ReplaceAll(text, "-", " ")
	text = strings.ReplaceAll(text, "_", " ")

	tokens := strings.Fields(text)

	// Filter out very short tokens
	var filtered []string
	for _, token := range tokens {
		if len(token) >= 2 {
			filtered = append(filtered, token)
		}
	}

	return filtered
}

// hashToken maps a token to multiple positions in the embedding vector
func (les *LocalEmbeddingService) hashToken(token string, dim int) []int {
	// Simple hash function to map tokens to 2-3 positions
	positions := make([]int, 0, 3)

	// Primary hash
	hash1 := 0
	for _, char := range token {
		hash1 = (hash1*31 + int(char)) % dim
	}
	positions = append(positions, hash1)

	// Secondary hash (different seed)
	hash2 := 17
	for _, char := range token {
		hash2 = (hash2*37 + int(char)) % dim
	}
	positions = append(positions, hash2)

	// Tertiary hash for longer tokens
	if len(token) > 4 {
		hash3 := 23
		for _, char := range token {
			hash3 = (hash3*41 + int(char)) % dim
		}
		positions = append(positions, hash3)
	}

	return positions
}

// getIDF returns a simple IDF score (can be enhanced with corpus statistics)
func (les *LocalEmbeddingService) getIDF(token string) float64 {
	// Simple IDF approximation based on token length and common words
	commonWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
		"are": true, "was": true, "were": true, "be": true, "been": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
	}

	if commonWords[token] {
		return 0.1 // Low weight for common words
	}

	// Higher weight for longer, more specific terms
	if len(token) >= 6 {
		return 2.0
	} else if len(token) >= 4 {
		return 1.5
	} else {
		return 1.0
	}
}

// normalizeVector normalizes the embedding vector to unit length
func (les *LocalEmbeddingService) normalizeVector(vector []float64) []float64 {
	var norm float64
	for _, val := range vector {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return vector
	}

	normalized := make([]float64, len(vector))
	for i, val := range vector {
		normalized[i] = val / norm
	}

	return normalized
}
