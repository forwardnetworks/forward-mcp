package ports

import "github.com/forward-mcp/internal/domain"

// BloomFilters pre-filter large NQE results, so a search scans only the items
// that may match.
type BloomFilters interface {
	BuildFilterFromNQEResult(networkID, filterType string, result *NQERunResult, chunkSize int) error
	SearchFilter(networkID, filterType string, searchTerms []string, allItems []map[string]interface{}) (*BloomSearchResult, error)
	IsFilterAvailable(networkID, filterType string) bool
	GetFilterStats() map[string]*FilterMetadata
	GetMemoryUsage() int64
}

// Domain types carried by BloomFilters.
type (
	FilterMetadata    = domain.FilterMetadata
	BloomSearchResult = domain.BloomSearchResult
)
