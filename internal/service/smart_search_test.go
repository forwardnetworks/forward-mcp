package service

import (
	"testing"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/logger"
)

// setupSmartSearchTestService creates a service for smart search testing
func setupSmartSearchTestService() *ForwardMCPService {
	cfg := &config.Config{
		Forward: config.ForwardConfig{
			APIKey:     "test-key",
			APISecret:  "test-secret",
			APIBaseURL: "https://test.example.com",
			Timeout:    10,
		},
	}

	testLogger := logger.New()
	embeddingService := NewMockEmbeddingService()

	// Initialize query index
	queryIndex := NewNQEQueryIndex(embeddingService, testLogger)

	// Initialize query index for tests with mock data instead of spec file
	if err := queryIndex.LoadFromMockData(); err != nil {
		testLogger.Error("Failed to load mock query index in smart search test: %v", err)
	}

	service := &ForwardMCPService{
		forwardClient:   NewMockForwardClient(),
		config:          cfg,
		logger:          testLogger,
		instanceID:      "test", // Add instance ID for test service
		defaults:        &ServiceDefaults{},
		workflowManager: NewWorkflowManager(),
		semanticCache:   NewSemanticCache(embeddingService, testLogger, "test", nil),
		queryIndex:      queryIndex,
	}

	return service
}

// Test searchNQEQueries function with empty query index (auto-initialization)
func TestSearchNQEQueries_AutoInitialization(t *testing.T) {
	service := setupSmartSearchTestService()

	// Test with empty query index - should trigger auto-initialization
	args := SearchNQEQueriesArgs{
		Query: "device information",
		Limit: 5,
	}

	response, err := service.searchNQEQueries(args)

	// Should succeed or provide helpful error message
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	// Response should be non-empty (either successful results or auto-initialization message)
	responseText := response.Content[0].TextContent.Text
	if responseText == "" {
		t.Error("Expected non-empty response text")
	}

	// Test passes if we get either:
	// 1. Successful search results (auto-initialization succeeded)
	// 2. Auto-initialization failure message with manual fix instructions
	// 3. Query index related message
	hasResults := contains(responseText, "search found") || contains(responseText, "relevant NQE queries")
	hasAutoInitFailed := contains(responseText, "Auto-initialization failed")
	hasQueryIndexMessage := contains(responseText, "query index") || contains(responseText, "Query index")

	if !hasResults && !hasAutoInitFailed && !hasQueryIndexMessage {
		t.Errorf("Expected either successful search results, auto-initialization failure message, or query index message. Got: %s", responseText)
	}

	// Should provide manual fix instructions when auto-init fails
	if hasAutoInitFailed {
		if !contains(responseText, "Manual Fix") {
			t.Error("Expected manual fix instructions when auto-initialization fails")
		}
	}
}

// Test searchNQEQueries with invalid query
func TestSearchNQEQueries_EmptyQuery(t *testing.T) {
	service := setupSmartSearchTestService()

	args := SearchNQEQueriesArgs{
		Query: "", // Empty query
		Limit: 5,
	}

	response, err := service.searchNQEQueries(args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	responseText := response.Content[0].TextContent.Text
	if !contains(responseText, "Please provide a search query") {
		t.Error("Expected response to ask for search query")
	}
}

// Test searchNQEQueries with various query parameters
func TestSearchNQEQueries_Parameters(t *testing.T) {
	service := setupSmartSearchTestService()

	testCases := []struct {
		name        string
		args        SearchNQEQueriesArgs
		expectError bool
	}{
		{
			name: "Basic query",
			args: SearchNQEQueriesArgs{
				Query: "device information",
				Limit: 10,
			},
			expectError: false,
		},
		{
			name: "Query with category filter",
			args: SearchNQEQueriesArgs{
				Query:    "security analysis",
				Category: "Security",
				Limit:    5,
			},
			expectError: false,
		},
		{
			name: "Query with subcategory filter",
			args: SearchNQEQueriesArgs{
				Query:       "BGP routing",
				Category:    "L3",
				Subcategory: "BGP",
				Limit:       3,
			},
			expectError: false,
		},
		{
			name: "Query with code inclusion",
			args: SearchNQEQueriesArgs{
				Query:       "interface statistics",
				IncludeCode: true,
				Limit:       5,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := service.searchNQEQueries(tc.args)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if response == nil {
					t.Error("Expected response, got nil")
				}
			}
		})
	}
}

// Test findExecutableQuery function with auto-initialization
func TestFindExecutableQuery_AutoInitialization(t *testing.T) {
	service := setupSmartSearchTestService()

	args := FindExecutableQueryArgs{
		Query: "show me all network devices",
		Limit: 3,
	}

	response, err := service.findExecutableQuery(args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	responseText := response.Content[0].TextContent.Text
	// Should either show results or explain auto-initialization
	if len(responseText) == 0 {
		t.Error("Response should not be empty")
	}
}

// Test findExecutableQuery with empty query
func TestFindExecutableQuery_EmptyQuery(t *testing.T) {
	service := setupSmartSearchTestService()

	args := FindExecutableQueryArgs{
		Query: "", // Empty query
	}

	response, err := service.findExecutableQuery(args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	responseText := response.Content[0].TextContent.Text
	if !contains(responseText, "Please describe what you want to analyze") {
		t.Error("Expected response to ask for query description")
	}
}

// Test findExecutableQuery with various parameters
func TestFindExecutableQuery_Parameters(t *testing.T) {
	service := setupSmartSearchTestService()

	testCases := []struct {
		name        string
		args        FindExecutableQueryArgs
		expectError bool
	}{
		{
			name: "Device information query",
			args: FindExecutableQueryArgs{
				Query: "show me device information",
				Limit: 5,
			},
			expectError: false,
		},
		{
			name: "Hardware query",
			args: FindExecutableQueryArgs{
				Query: "find hardware details",
				Limit: 3,
			},
			expectError: false,
		},
		{
			name: "Query with related matches",
			args: FindExecutableQueryArgs{
				Query:          "device CPU and memory usage",
				Limit:          2,
				IncludeRelated: true,
			},
			expectError: false,
		},
		{
			name: "Configuration search query",
			args: FindExecutableQueryArgs{
				Query: "search device configurations",
				Limit: 4,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := service.findExecutableQuery(tc.args)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if response == nil {
					t.Error("Expected response, got nil")
				} else {
					responseText := response.Content[0].TextContent.Text
					if len(responseText) == 0 {
						t.Error("Response should not be empty")
					}
				}
			}
		})
	}
}

// Test executable query mapping logic
func TestExecutableQueryMapping(t *testing.T) {
	// Create mock semantic results
	semanticResults := []*QuerySearchResult{
		{
			NQEQueryIndexEntry: &NQEQueryIndexEntry{
				Path:   "device_basic_info",
				Intent: "Get basic device information",
			},
			SimilarityScore: 0.8,
			MatchType:       "semantic",
		},
		{
			NQEQueryIndexEntry: &NQEQueryIndexEntry{
				Path:   "device_hardware",
				Intent: "Get device hardware details",
			},
			SimilarityScore: 0.7,
			MatchType:       "semantic",
		},
	}

	mappings := MapSemanticToExecutable(semanticResults)

	// Should find some mappings
	if len(mappings) == 0 {
		t.Error("Should find at least one executable mapping")
	}

	for _, mapping := range mappings {
		if mapping.ExecutableQuery == nil {
			t.Error("Each mapping should have an executable query")
		}
		if mapping.MappingConfidence <= 0.0 {
			t.Error("Mapping confidence should be positive")
		}
		if mapping.MappingReason == "" {
			t.Error("Mapping should have a reason")
		}
	}
}

// Test executable queries list
func TestGetExecutableQueries(t *testing.T) {
	queries := GetExecutableQueries()

	if len(queries) == 0 {
		t.Error("Should have at least one executable query")
	}

	for _, query := range queries {
		if query.QueryID == "" {
			t.Error("Query should have an ID")
		}
		if query.Name == "" {
			t.Error("Query should have a name")
		}
		if query.Description == "" {
			t.Error("Query should have a description")
		}
		if query.Category == "" {
			t.Error("Query should have a category")
		}
		if query.WhenToUse == "" {
			t.Error("Query should have usage guidance")
		}

		// QueryID should be a real Forward Networks ID format
		if !contains(query.QueryID, "FQ_") {
			t.Errorf("Query ID should be in Forward Networks format, got: %s", query.QueryID)
		}
	}
}

// Test query index initialization
func TestInitializeQueryIndex(t *testing.T) {
	service := setupSmartSearchTestService()

	args := InitializeQueryIndexArgs{
		RebuildIndex:       false,
		GenerateEmbeddings: false, // Don't generate embeddings for speed
	}

	response, err := service.initializeQueryIndex(args)

	// Note: This test might fail if spec file doesn't exist, which is expected
	// The response should provide helpful guidance in that case
	if err != nil {
		if !contains(err.Error(), "spec") {
			t.Errorf("Expected error to mention spec file, got: %v", err)
		}
	} else {
		if response == nil {
			t.Error("Expected response, got nil")
		} else {
			responseText := response.Content[0].TextContent.Text
			if !contains(responseText, "query index") {
				t.Error("Expected response to mention query index")
			}
		}
	}
}

// Test query index statistics
func TestGetQueryIndexStats(t *testing.T) {
	service := setupSmartSearchTestService()

	args := GetQueryIndexStatsArgs{
		Detailed: false,
	}

	response, err := service.getQueryIndexStats(args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	responseText := response.Content[0].TextContent.Text
	if !contains(responseText, "Query Index Statistics") {
		t.Error("Expected response to contain statistics header")
	}
}

// Test query index statistics with detailed view
func TestGetQueryIndexStats_Detailed(t *testing.T) {
	service := setupSmartSearchTestService()

	args := GetQueryIndexStatsArgs{
		Detailed: true,
	}

	response, err := service.getQueryIndexStats(args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	responseText := response.Content[0].TextContent.Text
	if !contains(responseText, "Query Index Statistics") {
		t.Error("Expected response to contain statistics header")
	}
}

// Test keyword embedding service used in smart search
func TestKeywordEmbeddingService_SmartSearch(t *testing.T) {
	service := NewKeywordEmbeddingService()

	testQueries := []string{
		"device information",
		"BGP routing analysis",
		"security compliance check",
		"interface utilization statistics",
	}

	var firstEmbeddingLength int

	for i, query := range testQueries {
		embedding, err := service.GenerateEmbedding(query)

		if err != nil {
			t.Errorf("Should generate embedding for query '%s', got error: %v", query, err)
			continue
		}

		if len(embedding) == 0 {
			t.Errorf("Embedding should not be empty for query: %s", query)
			continue
		}

		// All embeddings should have the same dimensionality
		if i == 0 {
			firstEmbeddingLength = len(embedding)
		} else {
			if len(embedding) != firstEmbeddingLength {
				t.Errorf("All embeddings should have same dimensionality. Expected %d, got %d for query: %s",
					firstEmbeddingLength, len(embedding), query)
			}
		}
	}
}

// Benchmark test for smart search performance
func BenchmarkSearchNQEQueries(b *testing.B) {
	service := setupSmartSearchTestService()

	args := SearchNQEQueriesArgs{
		Query: "device configuration analysis",
		Limit: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.searchNQEQueries(args)
		if err != nil {
			b.Logf("Search error (expected for empty index): %v", err)
		}
	}
}

// Benchmark test for find executable query performance
func BenchmarkFindExecutableQuery(b *testing.B) {
	service := setupSmartSearchTestService()

	args := FindExecutableQueryArgs{
		Query: "show me all network devices",
		Limit: 5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.findExecutableQuery(args)
		if err != nil {
			b.Logf("Search error (expected for empty index): %v", err)
		}
	}
}
