package ports

import "github.com/forward-mcp/internal/domain"

// EmbeddingService turns text into a vector for similarity search.
type EmbeddingService interface {
	GenerateEmbedding(text string) ([]float64, error)
}

// SyntheticEmbeddings is implemented by an EmbeddingService whose vectors carry
// no meaning, such as a test mock. Such vectors must not be saved.
type SyntheticEmbeddings interface {
	Synthetic() bool
}

// IsSynthetic reports whether svc produces meaningless vectors.
func IsSynthetic(svc EmbeddingService) bool {
	s, ok := svc.(SyntheticEmbeddings)
	return ok && s.Synthetic()
}

// ResultCache holds NQE results and finds them again by the meaning of the
// query that produced them, not only by its exact text.
type ResultCache interface {
	Get(query, networkID, snapshotID string) (*NQERunResult, bool)
	// GetExact matches only the exact same query text. Use it for keys built
	// from IDs and parameters, where "similar" means a different result.
	GetExact(query, networkID, snapshotID string) (*NQERunResult, bool)
	Put(query, networkID, snapshotID string, result *NQERunResult) error
	FindSimilarQueries(query string, limit int) ([]*CacheEntry, error)
	GetStats() map[string]interface{}
	// ClearExpired removes entries past their TTL and returns how many.
	ClearExpired() int
	// Clear removes every entry and resets the metrics.
	Clear()
	// Close stops background work.
	Close()
}

// Domain types carried by ResultCache.
type (
	CacheEntry   = domain.CacheEntry
	CacheMetrics = domain.CacheMetrics
)
