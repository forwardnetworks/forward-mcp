package domain

import "time"

// CacheEntry represents a cached query result with embeddings and metadata
type CacheEntry struct {
	Query           string        `json:"query"`
	NetworkID       string        `json:"network_id"`
	SnapshotID      string        `json:"snapshot_id"`
	Embedding       []float64     `json:"embedding"`
	Result          *NQERunResult `json:"result"`
	Timestamp       time.Time     `json:"timestamp"`
	AccessCount     int64         `json:"access_count"`
	LastAccessed    time.Time     `json:"last_accessed"`
	Hash            string        `json:"hash"`
	SimilarityScore float64       `json:"-"` // Used for search results

	// Enhanced fields for large result management
	CompressedSize   int64  `json:"compressed_size"`
	UncompressedSize int64  `json:"uncompressed_size"`
	IsCompressed     bool   `json:"is_compressed"`
	CompressedData   []byte `json:"-"`                   // Compressed result data
	DiskPath         string `json:"disk_path,omitempty"` // Path if stored on disk
}

// CacheMetrics holds detailed cache performance metrics
type CacheMetrics struct {
	HitCount          int64            `json:"hit_count"`
	MissCount         int64            `json:"miss_count"`
	TotalQueries      int64            `json:"total_queries"`
	EvictedCount      int64            `json:"evicted_count"`
	CurrentEntries    int              `json:"current_entries"`
	MemoryUsageBytes  int64            `json:"memory_usage_bytes"`
	MemoryUsageMB     float64          `json:"memory_usage_mb"`
	CompressionRatio  float64          `json:"compression_ratio"`
	AvgResponseTimeMs float64          `json:"avg_response_time_ms"`
	HitRate           float64          `json:"hit_rate"`
	EvictionsByPolicy map[string]int64 `json:"evictions_by_policy"`
	LastCleanup       time.Time        `json:"last_cleanup"`
}
