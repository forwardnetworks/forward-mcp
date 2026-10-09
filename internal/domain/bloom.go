package domain

import "time"

// FilterMetadata tracks bloom filter statistics and configuration
type FilterMetadata struct {
	NetworkID         string    `json:"network_id"`
	FilterType        string    `json:"filter_type"`
	ItemCount         int64     `json:"item_count"`
	FalsePositiveRate float64   `json:"false_positive_rate"`
	MemoryUsage       int64     `json:"memory_usage_bytes"`
	LastUpdated       time.Time `json:"last_updated"`
	ChunkCount        int       `json:"chunk_count"`
}

// BloomSearchResult represents a filtered result from bloomsearch
type BloomSearchResult struct {
	MatchedItems []map[string]interface{} `json:"matched_items"`
	FilterStats  *FilterMetadata          `json:"filter_stats"`
	SearchTime   time.Duration            `json:"search_time"`
	TotalItems   int                      `json:"total_items"`
	MatchedCount int                      `json:"matched_count"`
}
