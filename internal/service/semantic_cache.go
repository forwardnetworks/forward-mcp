package service

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

// EmbeddingService interface for generating embeddings
type EmbeddingService interface {
	GenerateEmbedding(text string) ([]float64, error)
}

// CacheEntry represents a cached query result with embeddings
type CacheEntry struct {
	Query           string                `json:"query"`
	NetworkID       string                `json:"network_id"`
	SnapshotID      string                `json:"snapshot_id"`
	Embedding       []float64             `json:"embedding"`
	Result          *forward.NQERunResult `json:"result"`
	Timestamp       time.Time             `json:"timestamp"`
	AccessCount     int                   `json:"access_count"`
	LastAccessed    time.Time             `json:"last_accessed"`
	Hash            string                `json:"hash"`
	SimilarityScore float64               `json:"-"` // Used for search results
}

// SemanticCache provides intelligent caching with embedding-based similarity
type SemanticCache struct {
	entries          map[string]*CacheEntry
	embeddingIndex   []*CacheEntry
	mutex            sync.RWMutex
	embeddingService EmbeddingService
	logger           *logger.Logger
	instanceID       string // Unique identifier for this Forward Networks instance

	// Configuration
	maxEntries          int
	ttl                 time.Duration
	similarityThreshold float64

	// Metrics
	hitCount     int64
	missCount    int64
	totalQueries int64
}

// truncateString safely truncates a string for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NewSemanticCache creates a new semantic cache
func NewSemanticCache(embeddingService EmbeddingService, logger *logger.Logger, instanceID string) *SemanticCache {
	return &SemanticCache{
		entries:             make(map[string]*CacheEntry),
		embeddingIndex:      make([]*CacheEntry, 0),
		embeddingService:    embeddingService,
		logger:              logger,
		instanceID:          instanceID,
		maxEntries:          1000,
		ttl:                 24 * time.Hour,
		similarityThreshold: 0.85, // 85% similarity threshold
	}
}

// generateCacheKey creates a consistent cache key including instance partitioning
func (sc *SemanticCache) generateCacheKey(query, networkID, snapshotID string) string {
	hasher := md5.New()
	hasher.Write([]byte(fmt.Sprintf("%s|%s|%s|%s", sc.instanceID, query, networkID, snapshotID)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// cosineSimilarity calculates cosine similarity between two embeddings
func (sc *SemanticCache) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Get attempts to retrieve a cached result using semantic similarity
func (sc *SemanticCache) Get(query, networkID, snapshotID string) (*forward.NQERunResult, bool) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	sc.totalQueries++

	// First try exact match
	key := sc.generateCacheKey(query, networkID, snapshotID)
	if entry, exists := sc.entries[key]; exists && !sc.isExpired(entry) {
		entry.AccessCount++
		entry.LastAccessed = time.Now()
		sc.hitCount++
		sc.logger.Debug("CACHE HIT: Exact match for query: %s", truncateString(query, 50))
		return entry.Result, true
	}

	// Generate embedding for semantic search
	embedding, err := sc.embeddingService.GenerateEmbedding(query)
	if err != nil {
		sc.logger.Error("CACHE ERROR: Failed to generate embedding: %v", err)
		sc.missCount++
		return nil, false
	}

	// Search for semantically similar queries
	bestMatch := sc.findBestMatch(embedding, networkID, snapshotID)
	if bestMatch != nil && bestMatch.SimilarityScore >= sc.similarityThreshold {
		bestMatch.AccessCount++
		bestMatch.LastAccessed = time.Now()
		sc.hitCount++
		sc.logger.Debug("CACHE HIT: Semantic match (%.3f similarity) for query: %s",
			bestMatch.SimilarityScore, truncateString(query, 50))
		return bestMatch.Result, true
	}

	sc.missCount++
	return nil, false
}

// Put stores a query result in the cache with its embedding
func (sc *SemanticCache) Put(query, networkID, snapshotID string, result *forward.NQERunResult) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	// Generate embedding
	embedding, err := sc.embeddingService.GenerateEmbedding(query)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	key := sc.generateCacheKey(query, networkID, snapshotID)
	entry := &CacheEntry{
		Query:        query,
		NetworkID:    networkID,
		SnapshotID:   snapshotID,
		Embedding:    embedding,
		Result:       result,
		Timestamp:    time.Now(),
		AccessCount:  1,
		LastAccessed: time.Now(),
		Hash:         key,
	}

	// Check if we need to evict entries
	if len(sc.entries) >= sc.maxEntries {
		sc.evictOldest()
	}

	sc.entries[key] = entry
	sc.embeddingIndex = append(sc.embeddingIndex, entry)

	sc.logger.Debug("CACHE PUT: Stored result for query: %s", truncateString(query, 50))
	return nil
}

// findBestMatch finds the most similar cached query
func (sc *SemanticCache) findBestMatch(embedding []float64, networkID, snapshotID string) *CacheEntry {
	var bestMatch *CacheEntry
	var bestSimilarity float64

	for _, entry := range sc.embeddingIndex {
		// Skip expired entries and different networks/snapshots
		if sc.isExpired(entry) ||
			(networkID != "" && entry.NetworkID != networkID) ||
			(snapshotID != "" && entry.SnapshotID != snapshotID) {
			continue
		}

		similarity := sc.cosineSimilarity(embedding, entry.Embedding)
		if similarity > bestSimilarity {
			bestSimilarity = similarity
			bestMatch = entry
		}
	}

	if bestMatch != nil {
		bestMatch.SimilarityScore = bestSimilarity
	}

	return bestMatch
}

// isExpired checks if a cache entry has expired
func (sc *SemanticCache) isExpired(entry *CacheEntry) bool {
	return time.Since(entry.Timestamp) > sc.ttl
}

// evictOldest removes the oldest cache entry
func (sc *SemanticCache) evictOldest() {
	if len(sc.entries) == 0 {
		return
	}

	// Find oldest entry
	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, entry := range sc.entries {
		if entry.LastAccessed.Before(oldestTime) {
			oldestTime = entry.LastAccessed
			oldestKey = key
		}
	}

	// Remove from both maps
	if entry, exists := sc.entries[oldestKey]; exists {
		delete(sc.entries, oldestKey)

		// Remove from embedding index
		for i, indexEntry := range sc.embeddingIndex {
			if indexEntry.Hash == oldestKey {
				sc.embeddingIndex = append(sc.embeddingIndex[:i], sc.embeddingIndex[i+1:]...)
				break
			}
		}

		sc.logger.Debug("CACHE EVICT: Removed entry for query: %s", truncateString(entry.Query, 50))
	}
}

// GetStats returns cache performance statistics
func (sc *SemanticCache) GetStats() map[string]interface{} {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	hitRate := float64(0)
	if sc.totalQueries > 0 {
		hitRate = float64(sc.hitCount) / float64(sc.totalQueries) * 100
	}

	return map[string]interface{}{
		"total_entries":    len(sc.entries),
		"total_queries":    sc.totalQueries,
		"cache_hits":       sc.hitCount,
		"cache_misses":     sc.missCount,
		"hit_rate_percent": fmt.Sprintf("%.2f", hitRate),
		"threshold":        sc.similarityThreshold,
		"max_entries":      sc.maxEntries,
		"ttl_hours":        sc.ttl.Hours(),
	}
}

// FindSimilarQueries returns similar cached queries for query suggestion
func (sc *SemanticCache) FindSimilarQueries(query string, limit int) ([]*CacheEntry, error) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	embedding, err := sc.embeddingService.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	var similarEntries []*CacheEntry

	for _, entry := range sc.embeddingIndex {
		if sc.isExpired(entry) {
			continue
		}

		similarity := sc.cosineSimilarity(embedding, entry.Embedding)
		if similarity > 0.5 { // Lower threshold for suggestions
			entryCopy := *entry
			entryCopy.SimilarityScore = similarity
			similarEntries = append(similarEntries, &entryCopy)
		}
	}

	// Sort by similarity
	sort.Slice(similarEntries, func(i, j int) bool {
		return similarEntries[i].SimilarityScore > similarEntries[j].SimilarityScore
	})

	// Limit results
	if len(similarEntries) > limit {
		similarEntries = similarEntries[:limit]
	}

	return similarEntries, nil
}

// ClearExpired removes all expired entries
func (sc *SemanticCache) ClearExpired() int {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	var removed int
	var validEntries []*CacheEntry

	for key, entry := range sc.entries {
		if sc.isExpired(entry) {
			delete(sc.entries, key)
			removed++
		} else {
			validEntries = append(validEntries, entry)
		}
	}

	sc.embeddingIndex = validEntries
	sc.logger.Debug("CACHE CLEANUP: Removed %d expired entries", removed)

	return removed
}
