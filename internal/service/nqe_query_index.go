package service

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

// NQEQueryIndex represents a query in the NQE library with AI-powered search capabilities
type NQEQueryIndexEntry struct {
	QueryID     string    `json:"queryId"`
	Path        string    `json:"path"`
	Intent      string    `json:"intent"`
	Code        string    `json:"code"`
	Category    string    `json:"category"`
	Subcategory string    `json:"subcategory"`
	Repository  string    `json:"repository"` // Track which repository this query comes from
	Embedding   []float32 `json:"embedding,omitempty"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// NQEQueryIndex manages the searchable index of NQE queries
type NQEQueryIndex struct {
	queries             []*NQEQueryIndexEntry
	embeddings          map[string][]float32
	embeddingService    EmbeddingService
	logger              *logger.Logger
	mutex               sync.RWMutex
	indexPath           string
	embeddingsCachePath string // Path to save/load embeddings
	offlineMode         bool   // Whether to work with cached embeddings only
	isLoading           bool   // Whether the index is currently loading
	isReady             bool   // Whether the index is ready for use
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

// QuerySearchResult represents a search result with similarity score
type QuerySearchResult struct {
	*NQEQueryIndexEntry
	SimilarityScore float64 `json:"similarityScore"`
	MatchType       string  `json:"matchType"` // "intent", "path", "code"
}

// NewNQEQueryIndex creates a new query index
func NewNQEQueryIndex(embeddingService EmbeddingService, logger *logger.Logger) *NQEQueryIndex {
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
		queries:             make([]*NQEQueryIndexEntry, 0),
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
		Queries []*NQEQueryIndexEntry `json:"queries"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&nqeLibrary); err != nil {
		return fmt.Errorf("failed to parse JSON file: %w", err)
	}

	if len(nqeLibrary.Queries) == 0 {
		idx.logger.Warn("No queries loaded from spec file")
		return fmt.Errorf("no queries found in spec file")
	}

	// Parse path into category, subcategory, and intent for each query
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
	}

	idx.queries = nqeLibrary.Queries
	idx.logger.Info("Loaded %d NQE queries into search index", len(nqeLibrary.Queries))

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
func (idx *NQEQueryIndex) LoadFromQueries(queries []forward.NQEQueryDetail) error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	idx.isLoading = true
	defer func() {
		idx.isLoading = false
		idx.isReady = true
	}()

	// Convert NQEQueryDetail to NQEQueryIndexEntry
	idx.queries = make([]*NQEQueryIndexEntry, 0, len(queries))

	for _, query := range queries {
		// Parse path into category, subcategory, and intent
		segments := strings.Split(strings.Trim(query.Path, "/"), "/")
		category := ""
		subcategory := ""
		intent := ""

		if len(segments) > 0 {
			category = segments[0]
		}
		if len(segments) > 1 {
			subcategory = segments[1]
		}
		if len(segments) > 0 {
			intent = segments[len(segments)-1]
		}

		entry := &NQEQueryIndexEntry{
			QueryID:     query.QueryID,
			Path:        query.Path,
			Intent:      intent,
			Code:        query.SourceCode,
			Category:    category,
			Subcategory: subcategory,
			Repository:  query.Repository, // Use the actual repository from API
			LastUpdated: time.Now(),
		}

		idx.queries = append(idx.queries, entry)
	}

	idx.logger.Info("Loaded %d NQE queries into search index from database", len(idx.queries))

	// Skip loading old embeddings cache - we're using database data now
	// Embeddings can be generated on-demand if needed for semantic search
	idx.logger.Debug("Using database-first approach - embeddings will be generated on-demand if needed")

	return nil
}

// LoadFromMockData loads mock data for testing (bypasses spec file requirement)
func (idx *NQEQueryIndex) LoadFromMockData() error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Create mock queries for testing
	mockQueries := []*NQEQueryIndexEntry{
		{
			QueryID:     "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029",
			Path:        "/L3/Basic/All Devices",
			Intent:      "List all devices in the network",
			Code:        "SELECT device_name, platform FROM devices",
			Category:    "L3",
			Subcategory: "Basic",
			Repository:  "ORG",
			LastUpdated: time.Now(),
		},
		{
			QueryID:     "FQ_test_hardware_query",
			Path:        "/Hardware/Basic/Device Hardware",
			Intent:      "Show device hardware information",
			Code:        "SELECT device_name, model, serial_number FROM device_hardware",
			Category:    "Hardware",
			Subcategory: "Basic",
			Repository:  "FWD",
			LastUpdated: time.Now(),
		},
		{
			QueryID:     "FQ_test_security_query",
			Path:        "/Security/Basic/ACL Analysis",
			Intent:      "Analyze access control lists",
			Code:        "SELECT device_name, acl_name, rule_count FROM acls",
			Category:    "Security",
			Subcategory: "Basic",
			Repository:  "ORG",
			LastUpdated: time.Now(),
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
	if _, ok := idx.embeddingService.(*MockEmbeddingService); ok {
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

		// Use all parsed fields for richer context
		searchText := fmt.Sprintf(
			"Query Path: %s\nCategory: %s\nSubcategory: %s\nIntent: %s",
			query.Path, query.Category, query.Subcategory, query.Intent,
		)

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

// SearchQueries performs semantic search on the query index
func (idx *NQEQueryIndex) SearchQueries(searchText string, limit int) ([]*QuerySearchResult, error) {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	if len(idx.queries) == 0 {
		return nil, fmt.Errorf("query index is empty - run LoadFromSpec() first")
	}

	// Count queries with embeddings
	embeddedCount := 0
	for _, query := range idx.queries {
		if len(query.Embedding) > 0 {
			embeddedCount++
		}
	}

	// For offline mode or when OpenAI is not available, use cached embeddings with keyword fallback
	var searchEmbedding []float32

	// Check if we should use keyword-based search directly
	_, isMock := idx.embeddingService.(*MockEmbeddingService)
	_, isKeyword := idx.embeddingService.(*KeywordEmbeddingService)

	if isMock || isKeyword || embeddedCount == 0 {
		// Use keyword-based matching for better accuracy with these services
		idx.logger.Debug("Using keyword-based search (service type: %T)", idx.embeddingService)
		return idx.searchWithKeywords(searchText, limit)
	}

	// Try to generate embedding for search text
	searchEmbedding64, err := idx.embeddingService.GenerateEmbedding(searchText)
	if err != nil {
		idx.logger.Debug("Failed to generate search embedding, falling back to keyword search: %v", err)
		return idx.searchWithKeywords(searchText, limit)
	}

	// Convert to float32
	searchEmbedding = make([]float32, len(searchEmbedding64))
	for i, v := range searchEmbedding64 {
		searchEmbedding[i] = float32(v)
	}

	var results []*QuerySearchResult

	// Calculate similarity scores using cached embeddings
	for _, query := range idx.queries {
		if len(query.Embedding) == 0 {
			continue
		}

		similarity := calculateCosineSimilarity(searchEmbedding, query.Embedding)

		// Lower threshold to be more lenient (was 0.05)
		if similarity > 0.01 {
			result := &QuerySearchResult{
				NQEQueryIndexEntry: query,
				SimilarityScore:    similarity,
				MatchType:          "semantic",
			}
			results = append(results, result)
		}
	}

	// Sort by similarity score (highest first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].SimilarityScore < results[j].SimilarityScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Debug printout: log the top 10 matches
	maxDebug := 10
	if len(results) < maxDebug {
		maxDebug = len(results)
	}
	idx.logger.Debug("Top %d semantic matches for search '%s':", maxDebug, searchText)
	for i := 0; i < maxDebug; i++ {
		q := results[i]
		idx.logger.Debug("  [%d] QueryID: %s | Path: %s | Intent: %s | Similarity: %.4f", i+1, q.QueryID, q.Path, q.Intent, q.SimilarityScore)
	}

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// searchWithKeywords provides keyword-based search as fallback when embeddings are not available
func (idx *NQEQueryIndex) searchWithKeywords(searchText string, limit int) ([]*QuerySearchResult, error) {
	searchTerms := strings.Fields(strings.ToLower(searchText))
	var results []*QuerySearchResult

	for _, query := range idx.queries {
		score := idx.calculateKeywordScore(query, searchTerms)

		if score > 0 {
			result := &QuerySearchResult{
				NQEQueryIndexEntry: query,
				SimilarityScore:    score,
				MatchType:          "keyword",
			}
			results = append(results, result)
		}
	}

	// Sort by keyword score (highest first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].SimilarityScore < results[j].SimilarityScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// calculateKeywordScore calculates a keyword-based similarity score
func (idx *NQEQueryIndex) calculateKeywordScore(query *NQEQueryIndexEntry, searchTerms []string) float64 {
	searchableText := strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s",
		query.Path,
		query.Intent,
		query.Category,
		query.Subcategory,
		query.Code,
		query.QueryID, // Allow searching by queryId
	))

	// For long queries, split into key concepts and require fewer matches
	var keyTerms []string
	if len(searchTerms) > 4 {
		// Group related terms together
		keyTerms = idx.extractKeyTerms(searchTerms)
	} else {
		keyTerms = searchTerms
	}

	score := 0.0
	matchedTerms := 0
	matchedKeyTerms := 0

	// First check for key term matches (more important)
	for _, term := range keyTerms {
		if strings.Contains(searchableText, term) {
			matchedKeyTerms++
			// Boost score for exact matches in important fields
			if strings.Contains(strings.ToLower(query.Intent), term) {
				score += 4.0 // Intent (last path segment) is most valuable
			} else if strings.Contains(strings.ToLower(query.QueryID), term) {
				score += 3.0 // QueryID match is very valuable
			} else if strings.Contains(strings.ToLower(query.Path), term) {
				score += 2.0 // Path matches are valuable
			} else if strings.Contains(strings.ToLower(query.Category), term) {
				score += 1.5 // Category matches are valuable
			} else if strings.Contains(strings.ToLower(query.Code), term) {
				score += 1.0 // Code matches are valuable
			} else {
				score += 0.5 // General matches
			}
		}
	}

	// Then check for individual term matches (less important)
	for _, term := range searchTerms {
		if strings.Contains(searchableText, term) {
			matchedTerms++
			score += 0.2 // Small boost for any match
		}
	}

	// Return a minimum score if we matched anything
	if matchedKeyTerms > 0 || matchedTerms > 0 {
		// Calculate final score with more weight on key term matches
		keyTermRatio := float64(matchedKeyTerms) / float64(len(keyTerms))
		termRatio := float64(matchedTerms) / float64(len(searchTerms))
		avgScore := score / float64(matchedKeyTerms+matchedTerms)

		// Scale from 0.05 to 1.0 with more emphasis on key term matches
		finalScore := 0.05 + (keyTermRatio * 0.4) + (termRatio * 0.3) + (avgScore * 0.25)

		// Cap at 1.0
		if finalScore > 1.0 {
			finalScore = 1.0
		}

		return finalScore
	}

	return 0.0
}

// GetQueryByID retrieves a specific query by its ID
func (idx *NQEQueryIndex) GetQueryByID(queryID string) (*NQEQueryIndexEntry, error) {
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
		"total_queries":      len(idx.queries),
		"embedded_queries":   embeddedCount,
		"categories":         categories,
		"subcategories":      subcategories,
		"embedding_coverage": float64(embeddedCount) / float64(len(idx.queries)),
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

	var queries []*NQEQueryIndexEntry
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

// extractKeyTerms groups related search terms into key concepts
func (idx *NQEQueryIndex) extractKeyTerms(searchTerms []string) []string {
	// Define groups of related terms
	securityTerms := []string{"security", "vulnerabilities", "vulnerability", "secure", "insecure"}
	accessTerms := []string{"access", "authentication", "authorization", "control", "permission"}
	cryptoTerms := []string{"encryption", "encrypted", "decrypt", "crypto", "certificate", "tls", "ssl"}
	weaknessTerms := []string{"weak", "password", "credentials", "default", "misconfiguration"}
	protocolTerms := []string{"protocol", "protocols", "http", "https", "ssh", "telnet", "ftp"}
	networkTerms := []string{"network", "routing", "bgp", "ospf", "interface", "vlan"}
	complianceTerms := []string{"compliance", "compliant", "policy", "policies", "standard", "requirement"}

	// Map to track which terms we've already used
	usedTerms := make(map[string]bool)
	var keyTerms []string

	// Helper to check if a term belongs to a group
	belongsToGroup := func(term string, group []string) bool {
		for _, g := range group {
			if strings.Contains(term, g) || strings.Contains(g, term) {
				return true
			}
		}
		return false
	}

	// Helper to add a representative term for a group
	addGroupTerm := func(term string, group []string, representative string) {
		if belongsToGroup(term, group) {
			if !usedTerms[representative] {
				keyTerms = append(keyTerms, representative)
				usedTerms[representative] = true
			}
		}
	}

	// Process each search term
	for _, term := range searchTerms {
		if usedTerms[term] {
			continue
		}

		// Check each group and add representative terms
		addGroupTerm(term, securityTerms, "security")
		addGroupTerm(term, accessTerms, "access-control")
		addGroupTerm(term, cryptoTerms, "encryption")
		addGroupTerm(term, weaknessTerms, "weak-credentials")
		addGroupTerm(term, protocolTerms, "protocols")
		addGroupTerm(term, networkTerms, "network")
		addGroupTerm(term, complianceTerms, "compliance")

		// If term doesn't belong to any group, add it as is
		matched := false
		for _, group := range [][]string{
			securityTerms, accessTerms, cryptoTerms, weaknessTerms,
			protocolTerms, networkTerms, complianceTerms,
		} {
			if belongsToGroup(term, group) {
				matched = true
				break
			}
		}
		if !matched {
			keyTerms = append(keyTerms, term)
			usedTerms[term] = true
		}
	}

	return keyTerms
}

// Queries returns the list of NQE queries in the index (read-only)
func (idx *NQEQueryIndex) Queries() []*NQEQueryIndexEntry {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	return idx.queries
}

// FilterQueriesByDirectory returns queries that match the specified directory path
func (idx *NQEQueryIndex) FilterQueriesByDirectory(directory string) []*NQEQueryIndexEntry {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	if directory == "" {
		// Return all queries if no directory filter
		return idx.queries
	}

	// Normalize the directory path
	normalizedDir := strings.Trim(directory, "/")
	var filteredQueries []*NQEQueryIndexEntry

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

// ConvertToNQEQuery converts NQEQueryIndexEntry to forward.NQEQuery for compatibility
func (entry *NQEQueryIndexEntry) ConvertToNQEQuery() forward.NQEQuery {
	// Use the actual repository information from the API instead of inferring from path
	return forward.NQEQuery{
		QueryID:    entry.QueryID,
		Path:       entry.Path,
		Intent:     entry.Intent,
		Repository: entry.Repository, // Use the stored repository from API
	}
}
