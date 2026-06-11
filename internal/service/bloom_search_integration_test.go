package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

func TestBloomSearchManager(t *testing.T) {
	logger := logger.New()
	instanceID := "test-instance"

	bsm := NewBloomSearchManager(logger, instanceID)

	// Test data
	networkID := "test-network"
	filterType := "device"

	// Create mock NQE result
	result := &forward.NQERunResult{
		SnapshotID: "latest",
		Items: []map[string]interface{}{
			{
				"device_name": "router-1",
				"platform":    "cisco-ios",
				"ip_address":  "192.168.1.1",
				"status":      "up",
			},
			{
				"device_name": "switch-1",
				"platform":    "cisco-ios",
				"ip_address":  "192.168.1.2",
				"status":      "up",
			},
			{
				"device_name": "firewall-1",
				"platform":    "palo-alto",
				"ip_address":  "192.168.1.3",
				"status":      "down",
			},
		},
	}

	// Test building filter
	err := bsm.BuildFilterFromNQEResult(networkID, filterType, result, 200)
	if err != nil {
		t.Fatalf("Failed to build bloom filter: %v", err)
	}

	// Test filter availability
	if !bsm.IsFilterAvailable(networkID, filterType) {
		t.Error("Filter should be available after building")
	}

	// Test search
	searchTerms := []string{"cisco-ios", "up"}
	searchResult, err := bsm.SearchFilter(networkID, filterType, searchTerms, result.Items)
	if err != nil {
		t.Fatalf("Failed to search bloom filter: %v", err)
	}

	// Verify search results
	if searchResult.TotalItems != 3 {
		t.Errorf("Expected 3 total items, got %d", searchResult.TotalItems)
	}

	if searchResult.SearchTime == 0 {
		t.Error("Search time should be greater than 0")
	}

	// Test stats
	stats := bsm.GetFilterStats()
	if len(stats) == 0 {
		t.Error("Should have filter stats after building")
	}

	filterKey := networkID + "-" + filterType
	if metadata, exists := stats[filterKey]; !exists {
		t.Error("Should have metadata for built filter")
	} else {
		if metadata.ItemCount != 3 {
			t.Errorf("Expected 3 items in metadata, got %d", metadata.ItemCount)
		}
		if metadata.FilterType != filterType {
			t.Errorf("Expected filter type %s, got %s", filterType, metadata.FilterType)
		}
	}

	// Test memory usage
	memoryUsage := bsm.GetMemoryUsage()
	if memoryUsage <= 0 {
		t.Error("Memory usage should be greater than 0")
	}

	// Test clearing filter
	bsm.ClearFilter(networkID, filterType)
	if bsm.IsFilterAvailable(networkID, filterType) {
		t.Error("Filter should not be available after clearing")
	}

	// Test clearing all filters
	bsm.BuildFilterFromNQEResult(networkID, filterType, result, 200)
	bsm.ClearAllFilters()
	if bsm.IsFilterAvailable(networkID, filterType) {
		t.Error("Filter should not be available after clearing all")
	}

	stats = bsm.GetFilterStats()
	if len(stats) != 0 {
		t.Error("Should have no stats after clearing all")
	}
}

func TestBloomSearchManagerCreateSearchableText(t *testing.T) {
	logger := logger.New()
	instanceID := "test-instance"

	bsm := NewBloomSearchManager(logger, instanceID)

	// Test item with various data types
	item := map[string]interface{}{
		"string_field": "test_value",
		"int_field":    42,
		"float_field":  3.14,
		"bool_field":   true,
		"array_field":  []interface{}{"item1", "item2"},
		"nested_field": map[string]interface{}{
			"nested_key": "nested_value",
		},
	}

	searchableText := bsm.createSearchableText(item)

	// Verify searchable text contains expected content
	expectedFields := []string{
		"string_field:test_value",
		"int_field:42",
		"float_field:3.14",
		"bool_field:true",
		"array_field[0]:item1",
		"array_field[1]:item2",
		"nested_field.nested_key:nested_value",
	}

	for _, expected := range expectedFields {
		if !containsString(searchableText, expected) {
			t.Errorf("Searchable text should contain '%s', got: %s", expected, searchableText)
		}
	}
}

func TestBloomSearchManagerPerformance(t *testing.T) {
	logger := logger.New()
	instanceID := "test-instance"

	bsm := NewBloomSearchManager(logger, instanceID)

	// Create larger test dataset
	networkID := "test-network"
	filterType := "device"

	items := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = map[string]interface{}{
			"device_name": fmt.Sprintf("device-%d", i),
			"platform":    "cisco-ios",
			"ip_address":  fmt.Sprintf("192.168.1.%d", i+1),
			"status":      "up",
		}
	}

	result := &forward.NQERunResult{
		SnapshotID: "latest",
		Items:      items,
	}

	// Test building filter performance
	start := time.Now()
	err := bsm.BuildFilterFromNQEResult(networkID, filterType, result, 200)
	buildTime := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to build bloom filter: %v", err)
	}

	// Build time should be reasonable (less than 1 second for 1000 items)
	if buildTime > time.Second {
		t.Errorf("Build time too slow: %v for 1000 items", buildTime)
	}

	// Test search performance
	searchTerms := []string{"cisco-ios"}
	start = time.Now()
	searchResult, err := bsm.SearchFilter(networkID, filterType, searchTerms, items)
	searchTime := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to search bloom filter: %v", err)
	}

	// Search time should be very fast (less than 5 milliseconds for bloom filter).
	// Race-detector instrumentation slows execution several-fold, so scale the
	// threshold accordingly when -race is active.
	searchLimit := 5 * time.Millisecond
	if raceEnabled {
		searchLimit = 100 * time.Millisecond
	}
	if searchTime > searchLimit {
		t.Errorf("Search time too slow: %v for bloom filter search", searchTime)
	}

	// Should find all items with "cisco-ios"
	if searchResult.MatchedCount != 1000 {
		t.Errorf("Expected 1000 matches, got %d", searchResult.MatchedCount)
	}

	t.Logf("Performance results - Build: %v, Search: %v", buildTime, searchTime)
}
