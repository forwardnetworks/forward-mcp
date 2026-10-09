package service

import (
	"fmt"
	"testing"
	"time"

	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
)

func TestBloomIndexManager(t *testing.T) {
	// Create logger
	logger := logger.New()

	// Create bloom index manager
	bloomIndexManager := NewBloomIndexManager(logger, "test_data")

	// Test data
	testRows := []map[string]interface{}{
		{
			"device_name":    "router-01",
			"interface_name": "GigabitEthernet0/1",
			"ip_address":     "192.168.1.1",
			"status":         "up",
			"platform":       "Cisco IOS-XE",
		},
		{
			"device_name":    "switch-01",
			"interface_name": "FastEthernet0/1",
			"ip_address":     "192.168.1.2",
			"status":         "up",
			"platform":       "Cisco IOS",
		},
		{
			"device_name":    "router-02",
			"interface_name": "GigabitEthernet0/2",
			"ip_address":     "192.168.1.3",
			"status":         "down",
			"platform":       "Cisco IOS-XE",
		},
	}

	indexedFields := []string{"device_name", "interface_name", "ip_address", "status", "platform"}

	// Test partitioning
	blocks := PartitionRows(testRows, 2)
	if len(blocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d", len(blocks))
	}

	if len(blocks[0]) != 2 {
		t.Errorf("Expected first block to have 2 rows, got %d", len(blocks[0]))
	}

	if len(blocks[1]) != 1 {
		t.Errorf("Expected second block to have 1 row, got %d", len(blocks[1]))
	}

	// Test engine creation
	entityID := "test-entity"
	engine, err := bloomIndexManager.GetOrCreateEngine(entityID)
	if err != nil {
		t.Errorf("Failed to create engine: %v", err)
	}

	if engine == nil {
		t.Error("Engine should not be nil")
	}

	// Test block ingestion
	blockID := "test-block-1"
	err = bloomIndexManager.IngestBlock(entityID, blockID, testRows, indexedFields)
	if err != nil {
		t.Errorf("Failed to ingest block: %v", err)
	}

	// Test search
	query := BloomSearchQuery{
		Queries: []BloomQuery{
			{
				Field:    "device_name",
				Operator: "contains",
				Value:    "router",
			},
		},
		Limit: 10,
	}

	result, err := bloomIndexManager.Search(entityID, query)
	if err != nil {
		t.Errorf("Failed to search: %v", err)
	}

	if result == nil {
		t.Error("Search result should not be nil")
	}

	// Test stats
	stats, err := bloomIndexManager.GetStats(entityID)
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}

	if stats == nil {
		t.Error("Stats should not be nil")
	}

	if stats.TotalBlocks != 1 {
		t.Errorf("Expected 1 block in stats, got %d", stats.TotalBlocks)
	}

	// Test multiple block search
	multiResults, err := bloomIndexManager.SearchMultipleBlocks(entityID, query)
	if err != nil {
		t.Errorf("Failed to search multiple blocks: %v", err)
	}

	if len(multiResults) == 0 {
		t.Error("Should have at least one result from multiple block search")
	}

	// Test close
	err = bloomIndexManager.Close()
	if err != nil {
		t.Errorf("Failed to close: %v", err)
	}
}

func TestBloomIndexManagerCreateSearchableText(t *testing.T) {
	// Create logger
	logger := logger.New()

	// Create bloom search manager (using existing implementation for comparison)
	bsm := NewBloomSearchManager(logger, "test-instance")

	// Test data
	testItem := map[string]interface{}{
		"device_name":    "router-01",
		"interface_name": "GigabitEthernet0/1",
		"ip_address":     "192.168.1.1",
		"status":         "up",
		"platform":       "Cisco IOS-XE",
		"port_count":     48,
		"enabled":        true,
		"tags":           []interface{}{"core", "router", "production"},
		"metadata": map[string]interface{}{
			"location": "datacenter-1",
			"rack":     "A01",
		},
	}

	// Test searchable text creation
	searchableText := bsm.createSearchableText(testItem)

	// Verify key fields are present
	expectedFields := []string{"device_name:router-01", "ip_address:192.168.1.1", "status:up"}
	for _, field := range expectedFields {
		if !containsString(searchableText, field) {
			t.Errorf("Searchable text should contain '%s', got: %s", field, searchableText)
		}
	}

	// Verify array handling
	if !containsString(searchableText, "tags[0]:core") {
		t.Errorf("Searchable text should contain array elements, got: %s", searchableText)
	}

	// Verify nested object handling
	if !containsString(searchableText, "metadata.location:datacenter-1") {
		t.Errorf("Searchable text should contain nested objects, got: %s", searchableText)
	}
}

func TestBloomIndexManagerPerformance(t *testing.T) {
	// Create logger
	logger := logger.New()

	// Create bloom index manager
	bloomIndexManager := NewBloomIndexManager(logger, "test_data")

	// Generate large test dataset
	var testRows []map[string]interface{}
	for i := 0; i < 1000; i++ {
		row := map[string]interface{}{
			"device_name":    fmt.Sprintf("device-%04d", i),
			"interface_name": fmt.Sprintf("interface-%04d", i),
			"ip_address":     fmt.Sprintf("192.168.%d.%d", i/256, i%256),
			"status":         "up",
			"platform":       "Cisco IOS-XE",
		}
		testRows = append(testRows, row)
	}

	indexedFields := []string{"device_name", "interface_name", "ip_address", "status", "platform"}

	// Test partitioning performance
	start := time.Now()
	blocks := PartitionRows(testRows, 100)
	partitionTime := time.Since(start)

	if len(blocks) != 10 {
		t.Errorf("Expected 10 blocks for 1000 rows with block size 100, got %d", len(blocks))
	}

	t.Logf("Partitioned %d rows into %d blocks in %v", len(testRows), len(blocks), partitionTime)

	// Test ingestion performance
	entityID := "perf-test-entity"
	start = time.Now()
	for i, block := range blocks {
		blockID := fmt.Sprintf("block-%d", i)
		err := bloomIndexManager.IngestBlock(entityID, blockID, block, indexedFields)
		if err != nil {
			t.Errorf("Failed to ingest block %d: %v", i, err)
		}
	}
	ingestionTime := time.Since(start)

	t.Logf("Ingested %d blocks in %v", len(blocks), ingestionTime)

	// Test search performance
	query := BloomSearchQuery{
		Queries: []BloomQuery{
			{
				Field:    "device_name",
				Operator: "contains",
				Value:    "device-",
			},
		},
		Limit: 50,
	}

	start = time.Now()
	result, err := bloomIndexManager.SearchMultipleBlocks(entityID, query)
	searchTime := time.Since(start)

	if err != nil {
		t.Errorf("Failed to search: %v", err)
	}

	t.Logf("Searched across %d blocks in %v, found %d results", len(blocks), searchTime, len(result))

	// Clean up
	err = bloomIndexManager.Close()
	if err != nil {
		t.Errorf("Failed to close: %v", err)
	}
}
