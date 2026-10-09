// Package queryindex is the ports.QueryIndex adapter: the NQE query library in
// memory, loaded from the database or spec/nqe-queries.json, searched by
// embedding similarity with keyword fallback.
package queryindex

import (
	"encoding/json"
	"fmt"
	"github.com/forward-mcp/internal/ports"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// NQEQueryIndex manages the searchable index of NQE queries
type NQEQueryIndex struct {
	queries             []*ports.NQEQueryIndexEntry
	embeddings          map[string][]float32
	embeddingService    ports.EmbeddingService
	logger              ports.Logger
	mutex               sync.RWMutex
	indexPath           string
	embeddingsCachePath string // Path to save/load embeddings
	offlineMode         bool   // Whether to work with cached embeddings only
	isLoading           bool   // Whether the index is currently loading
	isReady             bool   // Whether the index is ready for use

	bm25Mu sync.Mutex
	bm25   *bm25Index // built lazily from queries; see getBM25
}

// IsReady returns true if the query index is ready for use
func (idx *NQEQueryIndex) IsReady() bool {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	return idx.isReady && len(idx.queries) > 0
}

// IsLoading returns true if the query index is currently loading
func (idx *NQEQueryIndex) IsLoading() bool {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	return idx.isLoading
}

// SetLoading sets the loading state
func (idx *NQEQueryIndex) SetLoading(loading bool) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()
	idx.isLoading = loading
	if !loading {
		idx.isReady = true
	}
}

// NewNQEQueryIndex creates a new query index
func NewNQEQueryIndex(embeddingService ports.EmbeddingService, logger ports.Logger) *NQEQueryIndex {
	// Try to find the spec file using robust path resolution
	specPath, err := findSpecFile("NQELibrary.json")
	if err != nil {
		logger.Debug("Could not locate spec file during initialization: %v", err)
		specPath = "spec/NQELibrary.json" // fallback to relative path
	}

	// Find embeddings cache path in the same directory as spec file
	embeddingsCachePath := "spec/nqe-embeddings.json"
	if specPath != "spec/NQELibrary.json" {
		// Use the same directory as the spec file for embeddings cache
		specDir := filepath.Dir(specPath)
		embeddingsCachePath = filepath.Join(specDir, "nqe-embeddings.json")
	}

	return &NQEQueryIndex{
		queries:             make([]*ports.NQEQueryIndexEntry, 0),
		embeddings:          make(map[string][]float32),
		embeddingService:    embeddingService,
		logger:              logger,
		indexPath:           specPath,
		embeddingsCachePath: embeddingsCachePath,
		offlineMode:         false,
		isLoading:           false,
		isReady:             false,
	}
}

// LoadFromSpec parses the JSON spec file and extracts query information
func (idx *NQEQueryIndex) LoadFromSpec() error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Try to find the spec file using robust path resolution
	specPath, err := findSpecFile("NQELibrary.json")
	if err != nil {
		return fmt.Errorf("failed to open spec file: %w", err)
	}

	idx.logger.Debug("Loading NQE query index from spec file: %s", specPath)

	file, err := os.Open(specPath)
	if err != nil {
		return fmt.Errorf("failed to open spec file: %w", err)
	}
	defer file.Close()

	// Parse the JSON file
	var nqeLibrary struct {
		Queries []*ports.NQEQueryIndexEntry `json:"queries"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&nqeLibrary); err != nil {
		return fmt.Errorf("failed to parse JSON file: %w", err)
	}

	if len(nqeLibrary.Queries) == 0 {
		idx.logger.Warn("No queries loaded from spec file")
		return fmt.Errorf("no queries found in spec file")
	}

	filtered := make([]*ports.NQEQueryIndexEntry, 0, len(nqeLibrary.Queries))
	for _, query := range nqeLibrary.Queries {
		segments := strings.Split(strings.Trim(query.Path, "/"), "/")
		if len(segments) > 0 {
			query.Category = segments[0]
		}
		if len(segments) > 1 {
			query.Subcategory = segments[1]
		}
		if len(segments) > 0 {
			query.Intent = segments[len(segments)-1]
		}
		// The bundled library has paths but no descriptions. BM25 ranks on the
		// path and intent, so keep those queries; only the ones with real
		// descriptions count as strong metadata.
		intent := strings.TrimSpace(query.Intent)
		desc := strings.TrimSpace(query.Description)
		if len(intent) < 3 {
			continue
		}
		query.IsStrongMeta = len(intent) >= 10 && len(desc) >= 10
		// Exclude generic/test names
		lowerIntent := strings.ToLower(intent)
		if lowerIntent == "test" || lowerIntent == "example" || lowerIntent == "demo" ||
			strings.Contains(lowerIntent, "test") || strings.Contains(lowerIntent, "example") {
			continue
		}
		filtered = append(filtered, query)
	}

	if len(filtered) == 0 {
		return fmt.Errorf("spec file %s had %d queries but none were usable", specPath, len(nqeLibrary.Queries))
	}
	idx.queries = filtered
	idx.logger.Info("Loaded %d NQE queries into search index from spec file", len(filtered))

	// Try to load pre-generated embeddings
	if err := idx.loadEmbeddingsFromCache(); err != nil {
		idx.logger.Debug("Could not load cached embeddings: %v", err)
		idx.logger.Debug("Run 'initialize_query_index' with 'generate_embeddings: true' to create embeddings cache")
	} else {
		embeddedCount := 0
		for _, query := range idx.queries {
			if len(query.Embedding) > 0 {
				embeddedCount++
			}
		}
		idx.logger.Info("Loaded %d cached embeddings for offline AI search", embeddedCount)
	}

	return nil
}

// LoadFromQueries loads queries from a provided slice of NQEQueryDetail
func (idx *NQEQueryIndex) LoadFromQueries(queries []ports.NQEQueryDetail) error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	idx.isLoading = true
	defer func() {
		idx.isLoading = false
		idx.isReady = true
	}()

	var strongMeta, weakMeta []*ports.NQEQueryIndexEntry
	for _, query := range queries {
		segments := strings.Split(strings.Trim(query.Path, "/"), "/")
		category := ""
		subcategory := ""
		intent := strings.TrimSpace(query.Intent)
		desc := strings.TrimSpace(query.Description)
		path := query.Path

		if len(segments) > 0 {
			category = segments[0]
		}
		if len(segments) > 1 {
			subcategory = segments[1]
		}

		// Improved: Use both intent and description if available, else fallback
		embeddingText := ""
		switch {
		case intent != "" && desc != "":
			embeddingText = intent + " | " + desc
		case intent != "":
			embeddingText = intent
		case desc != "":
			embeddingText = desc
		default:
			embeddingText = path
		}

		isStrong := intent != "" && len(intent) >= 10 && desc != "" && len(desc) >= 10
		entry := &ports.NQEQueryIndexEntry{
			QueryID:      query.QueryID,
			Path:         query.Path,
			Intent:       intent,
			Description:  desc,
			Code:         embeddingText, // Use embedding text for embedding
			Category:     category,
			Subcategory:  subcategory,
			Repository:   query.Repository,
			LastUpdated:  time.Now(),
			IsStrongMeta: isStrong,
		}
		if isStrong {
			strongMeta = append(strongMeta, entry)
		} else {
			weakMeta = append(weakMeta, entry)
		}
	}
	filtered := append(strongMeta, weakMeta...)
	idx.queries = filtered
	idx.logger.Info("Loaded %d NQE queries into search index from database (%d strong, %d weak)", len(filtered), len(strongMeta), len(weakMeta))
	return nil
}

// LoadFromMockData loads mock data for testing (bypasses spec file requirement)
func (idx *NQEQueryIndex) LoadFromMockData() error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Create mock queries for testing
	mockQueries := []*ports.NQEQueryIndexEntry{
		{
			QueryID:      "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029",
			Path:         "/L3/Basic/All Devices",
			Intent:       "List all devices in the network",
			Description:  "This query retrieves a list of all devices connected to the network, including their names and platforms.",
			Code:         "SELECT device_name, platform FROM devices",
			Category:     "L3",
			Subcategory:  "Basic",
			Repository:   "ORG",
			LastUpdated:  time.Now(),
			IsStrongMeta: true,
		},
		{
			QueryID:      "FQ_test_hardware_query",
			Path:         "/Hardware/Basic/Device Hardware",
			Intent:       "Show device hardware information",
			Description:  "This query displays detailed hardware information for a specific device, including its name, model, and serial number.",
			Code:         "SELECT device_name, model, serial_number FROM device_hardware",
			Category:     "Hardware",
			Subcategory:  "Basic",
			Repository:   "FWD",
			LastUpdated:  time.Now(),
			IsStrongMeta: false,
		},
		{
			QueryID:      "FQ_test_security_query",
			Path:         "/Security/Basic/ACL Analysis",
			Intent:       "Analyze access control lists",
			Description:  "This query analyzes access control lists (ACLs) across the network to identify potential security vulnerabilities and misconfigurations.",
			Code:         "SELECT device_name, acl_name, rule_count FROM acls",
			Category:     "Security",
			Subcategory:  "Basic",
			Repository:   "ORG",
			LastUpdated:  time.Now(),
			IsStrongMeta: true,
		},
	}

	idx.queries = mockQueries
	idx.isReady = true
	idx.logger.Debug("Loaded %d mock NQE queries for testing", len(mockQueries))

	return nil
}

// loadEmbeddingsFromCache loads pre-generated embeddings from disk
func (idx *NQEQueryIndex) loadEmbeddingsFromCache() error {
	data, err := os.ReadFile(idx.embeddingsCachePath)
	if err != nil {
		return fmt.Errorf("failed to read embeddings cache: %w", err)
	}

	var embeddingsCache map[string][]float32
	if err := json.Unmarshal(data, &embeddingsCache); err != nil {
		return fmt.Errorf("failed to unmarshal embeddings cache: %w", err)
	}

	// Match embeddings to queries by path (more reliable than generated IDs)
	embeddingsLoaded := 0
	for _, query := range idx.queries {
		if embedding, exists := embeddingsCache[query.Path]; exists {
			query.Embedding = embedding
			idx.embeddings[query.QueryID] = embedding
			embeddingsLoaded++
		}
	}

	idx.logger.Debug("Loaded %d embeddings from cache file", embeddingsLoaded)
	return nil
}

// saveEmbeddingsToCache saves generated embeddings to disk for offline use
func (idx *NQEQueryIndex) saveEmbeddingsToCache() error {
	// Create a map of path -> embedding for reliable lookup
	embeddingsCache := make(map[string][]float32)

	for _, query := range idx.queries {
		if len(query.Embedding) > 0 {
			embeddingsCache[query.Path] = query.Embedding
		}
	}

	data, err := json.MarshalIndent(embeddingsCache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal embeddings cache: %w", err)
	}

	if err := os.WriteFile(idx.embeddingsCachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write embeddings cache: %w", err)
	}

	idx.logger.Info("Saved %d embeddings to cache file: %s", len(embeddingsCache), idx.embeddingsCachePath)
	return nil
}

// GenerateEmbeddings creates embeddings for all queries using the embedding service
func (idx *NQEQueryIndex) GenerateEmbeddings() error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Check if we can actually generate embeddings
	if ports.IsSynthetic(idx.embeddingService) {
		return fmt.Errorf("cannot generate real embeddings with mock service - set OPENAI_API_KEY")
	}

	idx.logger.Info("Generating embeddings for %d NQE queries...", len(idx.queries))

	successCount := 0
	for i, query := range idx.queries {
		// Skip if embedding already exists (for resuming)
		if len(query.Embedding) > 0 {
			successCount++
			continue
		}

		// Use the prioritized embedding text we stored in entry.Code
		searchText := query.Code
		if searchText == "" {
			// Fallback to old logic if Code is empty
			searchText = fmt.Sprintf(
				"Query Path: %s\nCategory: %s\nSubcategory: %s\nIntent: %s\nDescription: %s",
				query.Path, query.Category, query.Subcategory, query.Intent, query.Description,
			)
		}

		embedding, err := idx.embeddingService.GenerateEmbedding(searchText)
		if err != nil {
			idx.logger.Debug("Failed to generate embedding for query %s: %v", query.Path, err)
			continue
		}

		// Convert []float64 to []float32
		embedding32 := make([]float32, len(embedding))
		for j, v := range embedding {
			embedding32[j] = float32(v)
		}

		query.Embedding = embedding32
		idx.embeddings[query.QueryID] = embedding32
		successCount++

		// Log progress every 50 queries (more frequent updates)
		if (i+1)%50 == 0 {
			idx.logger.Info("Generated embeddings for %d/%d queries (%.1f%%)", i+1, len(idx.queries), float64(i+1)/float64(len(idx.queries))*100)
		}

		// Save progress incrementally every 100 queries to avoid losing work
		if successCount%100 == 0 {
			idx.logger.Info("Saving incremental progress (%d embeddings)...", successCount)
			if err := idx.saveEmbeddingsToCache(); err != nil {
				idx.logger.Error("Failed to save incremental cache: %v", err)
			} else {
				idx.logger.Info("Incremental cache saved successfully")
			}
		}
	}

	idx.logger.Info("Successfully generated embeddings for %d queries", successCount)

	// Save final embeddings to cache
	if err := idx.saveEmbeddingsToCache(); err != nil {
		idx.logger.Error("Failed to save embeddings cache: %v", err)
		return err
	}

	return nil
}

// calculateCosineSimilarity computes the cosine similarity between two vectors
func calculateCosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64

	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// GetQueryByID retrieves a specific query by its ID
func (idx *NQEQueryIndex) GetQueryByID(queryID string) (*ports.NQEQueryIndexEntry, error) {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	for _, query := range idx.queries {
		if query.QueryID == queryID {
			return query, nil
		}
	}

	return nil, fmt.Errorf("query with ID %s not found", queryID)
}

// GetStatistics returns statistics about the query index
func (idx *NQEQueryIndex) GetStatistics() map[string]interface{} {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	categories := make(map[string]int)
	subcategories := make(map[string]map[string]int)
	embeddedCount := 0

	// Initialize known categories
	knownCategories := []string{"L2", "L3", "Security", "Cloud", "Interfaces", "Hosts", "External", "Discovery", "Time", "Other"}
	for _, cat := range knownCategories {
		categories[cat] = 0
		subcategories[cat] = make(map[string]int)
	}

	// Count queries by category and subcategory
	for _, query := range idx.queries {
		if query.Category != "" {
			categories[query.Category]++
			if query.Subcategory != "" {
				if _, exists := subcategories[query.Category]; !exists {
					subcategories[query.Category] = make(map[string]int)
				}
				subcategories[query.Category][query.Subcategory]++
			}
		}
		if len(query.Embedding) > 0 {
			embeddedCount++
		}
	}

	// Remove empty categories
	for cat := range categories {
		if categories[cat] == 0 {
			delete(categories, cat)
			delete(subcategories, cat)
		}
	}

	return map[string]interface{}{
		"total_queries":    len(idx.queries),
		"embedded_queries": embeddedCount,
		"categories":       categories,
		"subcategories":    subcategories,
		"embedding_coverage": func() float64 {
			if len(idx.queries) == 0 {
				return 0.0
			}
			return float64(embeddedCount) / float64(len(idx.queries))
		}(),
	}
}

// SaveIndex saves the query index to a JSON file for faster loading
func (idx *NQEQueryIndex) SaveIndex(filename string) error {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	data, err := json.MarshalIndent(idx.queries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	idx.logger.Info("Saved NQE query index to %s", filename)
	return nil
}

// LoadIndex loads the query index from a JSON file
func (idx *NQEQueryIndex) LoadIndex(filename string) error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read index file: %w", err)
	}

	var queries []*ports.NQEQueryIndexEntry
	if err := json.Unmarshal(data, &queries); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	idx.queries = queries

	// Rebuild embeddings map
	idx.embeddings = make(map[string][]float32)
	for _, query := range queries {
		if len(query.Embedding) > 0 {
			idx.embeddings[query.QueryID] = query.Embedding
		}
	}

	idx.logger.Info("Loaded NQE query index from %s (%d queries)", filename, len(queries))
	return nil
}

// findSpecFile tries to locate the spec file in various possible locations
func findSpecFile(filename string) (string, error) {
	// Try multiple possible locations
	possiblePaths := []string{
		filename,                                       // Relative to current directory
		filepath.Join("spec", filename),                // In spec subdirectory
		filepath.Join("..", "spec", filename),          // One level up
		filepath.Join("forward-mcp", "spec", filename), // If we're in parent directory
	}

	// Also try to find it relative to the executable location
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		possiblePaths = append(possiblePaths,
			filepath.Join(execDir, "spec", filename),
			filepath.Join(execDir, "..", "spec", filename),
		)
	}

	// Try to find the project root by looking for go.mod
	if projectRoot, err := findProjectRoot(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(projectRoot, "spec", filename))
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath, nil
		}
	}

	return "", fmt.Errorf("spec file %s not found in any of the expected locations: %v", filename, possiblePaths)
}

// findProjectRoot locates the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found")
}

// Queries returns the list of NQE queries in the index (read-only)
func (idx *NQEQueryIndex) Queries() []*ports.NQEQueryIndexEntry {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	return idx.queries
}

// FilterQueriesByDirectory returns queries that match the specified directory path
func (idx *NQEQueryIndex) FilterQueriesByDirectory(directory string) []*ports.NQEQueryIndexEntry {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	if directory == "" {
		// Return all queries if no directory filter
		return idx.queries
	}

	// Normalize the directory path
	normalizedDir := strings.Trim(directory, "/")
	var filteredQueries []*ports.NQEQueryIndexEntry

	for _, query := range idx.queries {
		// Normalize the query path for comparison
		normalizedPath := strings.Trim(query.Path, "/")

		// Check if the query path starts with the directory
		if strings.HasPrefix(normalizedPath, normalizedDir) {
			// Additional check: ensure it's actually in this directory level
			// (not just a path that starts with the same string)
			remaining := strings.TrimPrefix(normalizedPath, normalizedDir)
			if remaining == "" || strings.HasPrefix(remaining, "/") {
				filteredQueries = append(filteredQueries, query)
			}
		}
	}

	return filteredQueries
}

// SpecPath locates the query specification that LoadFromSpec reads.
func (idx *NQEQueryIndex) SpecPath() (string, error) {
	return findSpecFile("NQELibrary.json")
}

// UsesSyntheticEmbeddings reports whether the embedding service produces
// meaningless vectors, in which case GenerateEmbeddings refuses to run.
func (idx *NQEQueryIndex) UsesSyntheticEmbeddings() bool {
	return ports.IsSynthetic(idx.embeddingService)
}

// NQEQueryIndex implements the QueryIndex port.
var _ ports.QueryIndex = (*NQEQueryIndex)(nil)
