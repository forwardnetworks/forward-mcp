package service

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

// EmbeddingService interface for generating embeddings
type EmbeddingService interface {
	GenerateEmbedding(text string) ([]float64, error)
}

// CacheEntry represents a cached query result with embeddings and metadata
type CacheEntry struct {
	Query           string                `json:"query"`
	NetworkID       string                `json:"network_id"`
	SnapshotID      string                `json:"snapshot_id"`
	Embedding       []float64             `json:"embedding"`
	Result          *forward.NQERunResult `json:"result"`
	Timestamp       time.Time             `json:"timestamp"`
	AccessCount     int64                 `json:"access_count"`
	LastAccessed    time.Time             `json:"last_accessed"`
	Hash            string                `json:"hash"`
	SimilarityScore float64               `json:"-"` // Used for search results

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

// SemanticCache provides intelligent caching with embedding-based similarity and configurable eviction
type SemanticCache struct {
	entries          map[string]*CacheEntry
	embeddingIndex   []*CacheEntry
	mutex            sync.RWMutex
	embeddingService EmbeddingService
	logger           *logger.Logger
	instanceID       string // Unique identifier for this Forward Networks instance
	config           *config.SemanticCacheConfig

	// Enhanced configuration
	maxEntries          int
	maxMemoryBytes      int64
	ttl                 time.Duration
	similarityThreshold float64
	evictionPolicy      config.CacheEvictionPolicy
	compressionEnabled  bool
	compressionLevel    int
	persistToDisk       bool
	diskCachePath       string
	metricsEnabled      bool
	memoryThreshold     float64
	cleanupInterval     time.Duration

	// Metrics
	metrics *CacheMetrics

	// Cleanup management
	stopCleanup   chan bool
	cleanupTicker *time.Ticker

	// Memory tracking
	currentMemoryUsage int64
}

// truncateString safely truncates a string for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NewSemanticCache creates a new semantic cache with enhanced configuration
func NewSemanticCache(embeddingService EmbeddingService, logger *logger.Logger, instanceID string, cfg *config.SemanticCacheConfig) *SemanticCache {
	if cfg == nil {
		// Use default configuration
		cfg = &config.SemanticCacheConfig{
			Enabled:                 true,
			MaxEntries:              1000,
			TTLHours:                24,
			SimilarityThreshold:     0.85,
			MaxMemoryMB:             512,
			EvictionPolicy:          config.EvictionPolicyLRU,
			CompressResults:         true,
			CompressionLevel:        6,
			PersistToDisk:           false,
			MetricsEnabled:          true,
			MemoryEvictionThreshold: 0.8,
			CleanupIntervalMinutes:  30,
		}
	}

	sc := &SemanticCache{
		entries:             make(map[string]*CacheEntry),
		embeddingIndex:      make([]*CacheEntry, 0),
		embeddingService:    embeddingService,
		logger:              logger,
		instanceID:          instanceID,
		config:              cfg,
		maxEntries:          cfg.MaxEntries,
		maxMemoryBytes:      int64(cfg.MaxMemoryMB) * 1024 * 1024,
		ttl:                 time.Duration(cfg.TTLHours) * time.Hour,
		similarityThreshold: cfg.SimilarityThreshold,
		evictionPolicy:      cfg.EvictionPolicy,
		compressionEnabled:  cfg.CompressResults,
		compressionLevel:    cfg.CompressionLevel,
		persistToDisk:       cfg.PersistToDisk,
		diskCachePath:       cfg.DiskCachePath,
		metricsEnabled:      cfg.MetricsEnabled,
		memoryThreshold:     cfg.MemoryEvictionThreshold,
		cleanupInterval:     time.Duration(cfg.CleanupIntervalMinutes) * time.Minute,
		stopCleanup:         make(chan bool, 1),
		metrics: &CacheMetrics{
			EvictionsByPolicy: make(map[string]int64),
		},
	}

	// Initialize disk cache directory if needed
	if sc.persistToDisk && sc.diskCachePath != "" {
		if err := os.MkdirAll(sc.diskCachePath, 0755); err != nil {
			logger.Warn("Failed to create disk cache directory %s: %v", sc.diskCachePath, err)
			sc.persistToDisk = false
		}
	}

	// Start background cleanup routine
	if sc.cleanupInterval > 0 {
		sc.startCleanupRoutine()
	}

	logger.Info("Enhanced semantic cache initialized - Policy: %s, MaxMemory: %dMB, Compression: %v",
		cfg.EvictionPolicy, cfg.MaxMemoryMB, cfg.CompressResults)

	return sc
}

// generateCacheKey creates a consistent cache key including instance partitioning
func (sc *SemanticCache) generateCacheKey(query, networkID, snapshotID string) string {
	hasher := md5.New()
	hasher.Write([]byte(fmt.Sprintf("%s|%s|%s|%s", sc.instanceID, query, networkID, snapshotID)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// estimateMemoryUsage estimates the memory usage of a cache entry
func (sc *SemanticCache) estimateMemoryUsage(entry *CacheEntry) int64 {
	var size int64

	// Basic fields
	size += int64(len(entry.Query))
	size += int64(len(entry.NetworkID))
	size += int64(len(entry.SnapshotID))
	size += int64(len(entry.Hash))
	size += int64(len(entry.DiskPath))

	// Embedding (float64 slice)
	size += int64(len(entry.Embedding) * 8)

	// Result size (either compressed or uncompressed)
	if entry.IsCompressed {
		size += entry.CompressedSize
	} else {
		size += entry.UncompressedSize
	}

	// Metadata overhead
	size += 200 // approximate overhead for timestamps, counters, etc.

	return size
}

// compressResult compresses the NQE result using gzip
func (sc *SemanticCache) compressResult(result *forward.NQERunResult) ([]byte, int64, error) {
	if !sc.compressionEnabled {
		return nil, 0, nil
	}

	// Serialize result to JSON first
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal result: %w", err)
	}

	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, sc.compressionLevel)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := writer.Write(jsonData); err != nil {
		writer.Close()
		return nil, 0, fmt.Errorf("failed to compress data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	compressedData := buf.Bytes()
	uncompressedSize := int64(len(jsonData))

	return compressedData, uncompressedSize, nil
}

// decompressResult decompresses the cached result
func (sc *SemanticCache) decompressResult(compressedData []byte) (*forward.NQERunResult, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	decompressedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	var result forward.NQERunResult
	if err := json.Unmarshal(decompressedData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
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
	start := time.Now()
	defer func() {
		if sc.metricsEnabled {
			sc.metrics.AvgResponseTimeMs = (sc.metrics.AvgResponseTimeMs + float64(time.Since(start).Nanoseconds())/1e6) / 2
		}
	}()

	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	sc.metrics.TotalQueries++

	// First try exact match
	key := sc.generateCacheKey(query, networkID, snapshotID)
	if entry, exists := sc.entries[key]; exists && !sc.isExpired(entry) {
		result, err := sc.getResultFromEntry(entry)
		if err != nil {
			sc.logger.Error("Failed to retrieve result from cache entry: %v", err)
			sc.metrics.MissCount++
			return nil, false
		}

		// Update access metrics
		entry.AccessCount++
		entry.LastAccessed = time.Now()
		sc.metrics.HitCount++

		sc.logger.Debug("CACHE HIT: Exact match for query: %s (compression: %v, size: %d bytes)",
			truncateString(query, 50), entry.IsCompressed, entry.CompressedSize)
		return result, true
	}

	// Generate embedding for semantic search if embedding service available
	if sc.embeddingService != nil {
		embedding, err := sc.embeddingService.GenerateEmbedding(query)
		if err != nil {
			sc.logger.Debug("Failed to generate embedding for semantic search: %v", err)
		} else {
			// Search for semantically similar queries
			bestMatch := sc.findBestMatch(embedding, networkID, snapshotID)
			if bestMatch != nil && bestMatch.SimilarityScore >= sc.similarityThreshold {
				result, err := sc.getResultFromEntry(bestMatch)
				if err != nil {
					sc.logger.Error("Failed to retrieve result from best match: %v", err)
					sc.metrics.MissCount++
					return nil, false
				}

				bestMatch.AccessCount++
				bestMatch.LastAccessed = time.Now()
				sc.metrics.HitCount++

				sc.logger.Debug("CACHE HIT: Semantic match (%.3f similarity) for query: %s",
					bestMatch.SimilarityScore, truncateString(query, 50))
				return result, true
			}
		}
	}

	sc.metrics.MissCount++
	return nil, false
}

// getResultFromEntry retrieves the result from a cache entry, handling compression and disk storage
func (sc *SemanticCache) getResultFromEntry(entry *CacheEntry) (*forward.NQERunResult, error) {
	if entry.DiskPath != "" && sc.persistToDisk {
		// Load from disk
		return sc.loadFromDisk(entry.DiskPath)
	}

	if entry.IsCompressed && len(entry.CompressedData) > 0 {
		// Decompress in-memory data
		return sc.decompressResult(entry.CompressedData)
	}

	// Return uncompressed in-memory data
	return entry.Result, nil
}

// Put stores a query result in the cache with its embedding
func (sc *SemanticCache) Put(query, networkID, snapshotID string, result *forward.NQERunResult) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	// Generate embedding if service available
	var embedding []float64
	var err error
	if sc.embeddingService != nil {
		embedding, err = sc.embeddingService.GenerateEmbedding(query)
		if err != nil {
			sc.logger.Debug("Failed to generate embedding, storing without semantic search capability: %v", err)
		}
	}

	key := sc.generateCacheKey(query, networkID, snapshotID)

	// Estimate uncompressed size
	resultBytes, _ := json.Marshal(result)
	uncompressedSize := int64(len(resultBytes))

	entry := &CacheEntry{
		Query:            query,
		NetworkID:        networkID,
		SnapshotID:       snapshotID,
		Embedding:        embedding,
		Result:           result,
		Timestamp:        time.Now(),
		AccessCount:      1,
		LastAccessed:     time.Now(),
		Hash:             key,
		UncompressedSize: uncompressedSize,
	}

	// Apply compression if enabled
	if sc.compressionEnabled {
		compressedData, originalSize, err := sc.compressResult(result)
		if err != nil {
			sc.logger.Warn("Failed to compress result, storing uncompressed: %v", err)
		} else {
			entry.CompressedData = compressedData
			entry.CompressedSize = int64(len(compressedData))
			entry.IsCompressed = true
			entry.UncompressedSize = originalSize
			entry.Result = nil // Clear uncompressed data to save memory

			// Update compression metrics
			if sc.metricsEnabled && originalSize > 0 {
				ratio := float64(entry.CompressedSize) / float64(originalSize)
				sc.metrics.CompressionRatio = (sc.metrics.CompressionRatio + ratio) / 2
			}
		}
	}

	// Check if we should persist to disk for very large results
	entrySize := sc.estimateMemoryUsage(entry)
	if sc.persistToDisk && entrySize > (sc.maxMemoryBytes/20) { // > 5% of max memory
		if err := sc.saveToDisk(entry); err != nil {
			sc.logger.Warn("Failed to save large entry to disk: %v", err)
		}
	}

	// Check memory and entry limits before adding
	if err := sc.ensureCapacity(entrySize); err != nil {
		return fmt.Errorf("failed to ensure cache capacity: %w", err)
	}

	// Store the entry
	sc.entries[key] = entry
	if len(embedding) > 0 {
		sc.embeddingIndex = append(sc.embeddingIndex, entry)
	}

	// Update memory tracking
	sc.currentMemoryUsage += entrySize

	sc.logger.Debug("CACHE PUT: Stored result for query: %s (compressed: %v, size: %dB->%dB)",
		truncateString(query, 50), entry.IsCompressed, entry.UncompressedSize, entry.CompressedSize)

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
		// Update memory usage before deleting
		entrySize := sc.estimateMemoryUsage(entry)
		sc.currentMemoryUsage -= entrySize

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

// evictEntriesByPolicy evicts entries based on the configured eviction policy
func (sc *SemanticCache) evictEntriesByPolicy(maxToEvict int) int {
	if len(sc.entries) == 0 {
		return 0
	}

	evicted := 0
	policy := sc.config.EvictionPolicy

	switch policy {
	case config.EvictionPolicyLRU:
		// Evict based on Least Recently Used
		for evicted < maxToEvict && len(sc.entries) > 0 {
			sc.evictOldest()
			evicted++
		}
	case config.EvictionPolicyLFU:
		// Evict based on Least Frequently Used
		for evicted < maxToEvict && len(sc.entries) > 0 {
			sc.evictLeastFrequent()
			evicted++
		}
	case config.EvictionPolicySize:
		// Evict largest entries first
		for evicted < maxToEvict && len(sc.entries) > 0 {
			sc.evictLargest()
			evicted++
		}
	default:
		// Default to oldest
		for evicted < maxToEvict && len(sc.entries) > 0 {
			sc.evictOldest()
			evicted++
		}
	}

	sc.metrics.EvictedCount += int64(evicted)
	sc.metrics.EvictionsByPolicy[string(policy)] += int64(evicted)

	return evicted
}

// evictLeastFrequent removes the least frequently used cache entry
func (sc *SemanticCache) evictLeastFrequent() {
	if len(sc.entries) == 0 {
		return
	}

	var lfuKey string
	var minAccessCount int64 = -1

	for key, entry := range sc.entries {
		if minAccessCount == -1 || entry.AccessCount < minAccessCount {
			minAccessCount = entry.AccessCount
			lfuKey = key
		}
	}

	if entry, exists := sc.entries[lfuKey]; exists {
		// Update memory usage before deleting
		entrySize := sc.estimateMemoryUsage(entry)
		sc.currentMemoryUsage -= entrySize

		delete(sc.entries, lfuKey)

		// Remove from embedding index
		for i, indexEntry := range sc.embeddingIndex {
			if indexEntry.Hash == lfuKey {
				sc.embeddingIndex = append(sc.embeddingIndex[:i], sc.embeddingIndex[i+1:]...)
				break
			}
		}

		sc.logger.Debug("CACHE EVICT (LFU): Removed entry for query: %s (access count: %d)",
			truncateString(entry.Query, 50), entry.AccessCount)
	}
}

// evictLargest removes the largest cache entry by memory usage
func (sc *SemanticCache) evictLargest() {
	if len(sc.entries) == 0 {
		return
	}

	var largestKey string
	var maxSize int64 = 0

	for key, entry := range sc.entries {
		size := sc.estimateMemoryUsage(entry)
		if size > maxSize {
			maxSize = size
			largestKey = key
		}
	}

	if entry, exists := sc.entries[largestKey]; exists {
		// Update memory usage before deleting
		entrySize := sc.estimateMemoryUsage(entry)
		sc.currentMemoryUsage -= entrySize

		delete(sc.entries, largestKey)

		// Remove from embedding index
		for i, indexEntry := range sc.embeddingIndex {
			if indexEntry.Hash == largestKey {
				sc.embeddingIndex = append(sc.embeddingIndex[:i], sc.embeddingIndex[i+1:]...)
				break
			}
		}

		sc.logger.Debug("CACHE EVICT (Size): Removed entry for query: %s (size: %d bytes)",
			truncateString(entry.Query, 50), maxSize)
	}
}

// GetStats returns cache performance statistics
func (sc *SemanticCache) GetStats() map[string]interface{} {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	hitRate := float64(0)
	if sc.metrics.TotalQueries > 0 {
		hitRate = float64(sc.metrics.HitCount) / float64(sc.metrics.TotalQueries) * 100
	}

	// Update current metrics
	sc.metrics.CurrentEntries = len(sc.entries)
	sc.metrics.MemoryUsageBytes = sc.currentMemoryUsage
	sc.metrics.MemoryUsageMB = float64(sc.currentMemoryUsage) / (1024 * 1024)
	sc.metrics.HitRate = hitRate

	return map[string]interface{}{
		"total_entries":        len(sc.entries),
		"total_queries":        sc.metrics.TotalQueries,
		"cache_hits":           sc.metrics.HitCount,
		"cache_misses":         sc.metrics.MissCount,
		"hit_rate_percent":     fmt.Sprintf("%.2f", hitRate),
		"threshold":            sc.similarityThreshold,
		"max_entries":          sc.maxEntries,
		"max_memory_mb":        float64(sc.maxMemoryBytes) / (1024 * 1024),
		"current_memory_mb":    sc.metrics.MemoryUsageMB,
		"memory_usage_bytes":   sc.metrics.MemoryUsageBytes,
		"memory_utilization_%": fmt.Sprintf("%.2f", (sc.metrics.MemoryUsageMB/(float64(sc.maxMemoryBytes)/(1024*1024)))*100),
		"ttl_hours":            sc.ttl.Hours(),
		"compression_enabled":  sc.compressionEnabled,
		"compression_ratio":    fmt.Sprintf("%.3f", sc.metrics.CompressionRatio),
		"avg_response_time_ms": fmt.Sprintf("%.2f", sc.metrics.AvgResponseTimeMs),
		"evicted_count":        sc.metrics.EvictedCount,
		"evictions_by_policy":  sc.metrics.EvictionsByPolicy,
		"last_cleanup":         sc.metrics.LastCleanup,
		"persistence_enabled":  sc.persistToDisk,
		"disk_cache_path":      sc.diskCachePath,
		"eviction_policy":      string(sc.evictionPolicy),
		"cleanup_interval_min": sc.cleanupInterval.Minutes(),
		"memory_threshold_%":   sc.memoryThreshold * 100,
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

	return sc.clearExpiredInternal()
}

// clearExpiredInternal removes all expired entries (assumes mutex is already locked)
func (sc *SemanticCache) clearExpiredInternal() int {
	var removed int
	var validEntries []*CacheEntry

	for key, entry := range sc.entries {
		if sc.isExpired(entry) {
			// Update memory usage before deleting
			entrySize := sc.estimateMemoryUsage(entry)
			sc.currentMemoryUsage -= entrySize

			delete(sc.entries, key)
			removed++
		} else {
			validEntries = append(validEntries, entry)
		}
	}

	sc.embeddingIndex = validEntries

	// Update cleanup metrics
	if sc.metricsEnabled {
		sc.metrics.LastCleanup = time.Now()
	}

	sc.logger.Debug("CACHE CLEANUP: Removed %d expired entries", removed)

	return removed
}

// startCleanupRoutine starts a background routine to periodically clean up expired entries
func (sc *SemanticCache) startCleanupRoutine() {
	sc.cleanupTicker = time.NewTicker(sc.cleanupInterval)
	go func() {
		for {
			select {
			case <-sc.stopCleanup:
				sc.cleanupTicker.Stop()
				return
			case <-sc.cleanupTicker.C:
				sc.ClearExpired()
				sc.logger.Debug("Background cleanup routine triggered. Current entries: %d", len(sc.entries))
			}
		}
	}()
}

// stopCleanupRoutine stops the background cleanup routine
func (sc *SemanticCache) stopCleanupRoutine() {
	sc.stopCleanup <- true
}

// ensureCapacity ensures the cache has enough capacity for a new entry.
func (sc *SemanticCache) ensureCapacity(entrySize int64) error {
	// Check entry count limit first
	if len(sc.entries) >= sc.maxEntries {
		sc.logger.Debug("Cache entry limit reached (%d/%d). Evicting entries.", len(sc.entries), sc.maxEntries)
		evictedCount := sc.clearExpiredInternal() // Clear expired entries first
		sc.logger.Debug("Evicted %d expired entries. Current entries: %d", evictedCount, len(sc.entries))

		// If still at limit, evict based on policy
		if len(sc.entries) >= sc.maxEntries {
			evictionCount := sc.evictEntriesByPolicy(1) // Evict one entry to make room
			sc.logger.Debug("Evicted %d entries by policy. Current entries: %d", evictionCount, len(sc.entries))
		}
	}

	// Check memory limit
	if sc.currentMemoryUsage+entrySize > sc.maxMemoryBytes {
		sc.logger.Warn("Cache memory limit reached. Evicting entries.")
		evictedCount := sc.clearExpiredInternal() // Clear expired entries first
		sc.logger.Debug("Evicted %d expired entries to free memory. Current memory: %d bytes", evictedCount, sc.currentMemoryUsage)

		// If still not enough, evict based on policy
		if sc.currentMemoryUsage+entrySize > sc.maxMemoryBytes {
			evictionCount := sc.evictEntriesByPolicy(10) // Evict up to 10 entries
			sc.logger.Debug("Evicted %d entries to free memory. Current memory: %d bytes", evictionCount, sc.currentMemoryUsage)

			// Final check
			if sc.currentMemoryUsage+entrySize > sc.maxMemoryBytes {
				sc.logger.Error("Could not free enough memory for new entry. Memory limit: %d bytes, Current: %d bytes", sc.maxMemoryBytes, sc.currentMemoryUsage)
				return fmt.Errorf("memory limit reached and could not free enough space")
			}
		}
	}
	return nil
}

// saveToDisk saves a cache entry to disk
func (sc *SemanticCache) saveToDisk(entry *CacheEntry) error {
	if !sc.persistToDisk || entry.DiskPath == "" {
		return fmt.Errorf("persistence not enabled or disk path not set")
	}

	filePath := filepath.Join(sc.diskCachePath, entry.Hash)

	// Serialize entry to JSON
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry to JSON: %w", err)
	}

	// Compress if enabled
	var compressedData []byte
	var uncompressedSize int64
	if sc.compressionEnabled {
		compressedData, uncompressedSize, err = sc.compressResult(entry.Result) // Compress the result
		if err != nil {
			sc.logger.Warn("Failed to compress entry result for disk storage: %v", err)
			// Fallback to uncompressed if compression fails
			compressedData = entryBytes
			uncompressedSize = int64(len(entryBytes))
		}
	} else {
		compressedData = entryBytes
		uncompressedSize = int64(len(entryBytes))
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for disk cache: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, compressedData, 0644); err != nil {
		return fmt.Errorf("failed to write to disk cache file %s: %w", filePath, err)
	}

	entry.DiskPath = filePath
	entry.CompressedSize = int64(len(compressedData))
	entry.UncompressedSize = uncompressedSize
	entry.IsCompressed = sc.compressionEnabled
	entry.CompressedData = compressedData

	sc.logger.Debug("Saved entry to disk: %s (compressed: %v, size: %dB->%dB)",
		truncateString(entry.Query, 50), entry.IsCompressed, entry.UncompressedSize, entry.CompressedSize)

	return nil
}

// loadFromDisk loads a cache entry from disk
func (sc *SemanticCache) loadFromDisk(filePath string) (*forward.NQERunResult, error) {
	filePath = filepath.Clean(filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache file not found at %s", filePath)
	}

	// Read file
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read from disk cache file %s: %w", filePath, err)
	}

	// Deserialize the stored entry
	var entry CacheEntry
	if err := json.Unmarshal(fileBytes, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entry from disk: %w", err)
	}

	// Get the result from the loaded entry
	if entry.IsCompressed && len(entry.CompressedData) > 0 {
		// Decompress the result
		result, err := sc.decompressResult(entry.CompressedData)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress result from disk: %w", err)
		}

		sc.logger.Debug("Loaded compressed entry from disk: %s (size: %dB->%dB)",
			truncateString(entry.Query, 50), entry.CompressedSize, entry.UncompressedSize)

		return result, nil
	}

	// Return uncompressed result
	sc.logger.Debug("Loaded uncompressed entry from disk: %s (size: %dB)",
		truncateString(entry.Query, 50), entry.UncompressedSize)

	return entry.Result, nil
}
