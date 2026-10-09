package domain

import "time"

// NQEQueryIndexEntry represents a query in the NQE library with AI-powered search capabilities
type NQEQueryIndexEntry struct {
	QueryID      string    `json:"queryId"`
	Path         string    `json:"path"`
	Intent       string    `json:"intent"`
	Description  string    `json:"description"` // Add this field for extracted @description
	Code         string    `json:"code"`
	Category     string    `json:"category"`
	Subcategory  string    `json:"subcategory"`
	Repository   string    `json:"repository"` // Track which repository this query comes from
	Embedding    []float32 `json:"embedding,omitempty"`
	LastUpdated  time.Time `json:"lastUpdated"`
	IsStrongMeta bool      `json:"isStrongMeta"` // New: flag for strong metadata
}

// QuerySearchResult represents a search result with similarity score
type QuerySearchResult struct {
	*NQEQueryIndexEntry
	SimilarityScore float64 `json:"similarityScore"`
	MatchType       string  `json:"matchType"` // "intent", "path", "code"
}

// ConvertToNQEQuery converts an index entry to the API query type for compatibility
func (entry *NQEQueryIndexEntry) ConvertToNQEQuery() NQEQuery {
	// Use the actual repository information from the API instead of inferring from path
	return NQEQuery{
		QueryID:    entry.QueryID,
		Path:       entry.Path,
		Intent:     entry.Intent,
		Repository: entry.Repository, // Use the stored repository from API
	}
}
