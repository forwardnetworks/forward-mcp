package service

import (
	"testing"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

func TestAPIMemoryTracker_TrackNetworkQuery(t *testing.T) {
	// Create test memory system
	logger := logger.New()
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	// Create API tracker
	tracker := NewAPIMemoryTracker(memorySystem, logger, "test-instance")

	// Mock query result
	result := &forward.NQERunResult{
		SnapshotID: "test-snapshot",
		Items: []map[string]interface{}{
			{"device": "router1", "type": "router"},
			{"device": "switch1", "type": "switch"},
		},
	}

	// Track a query execution
	err := tracker.TrackNetworkQuery("test-query", "test-network", "test-snapshot", result, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to track network query: %v", err)
	}

	// Verify entities were created
	queryEntity, err := memorySystem.GetEntity("test-query")
	if err != nil {
		t.Fatalf("Query entity not found: %v", err)
	}

	if queryEntity.Type != "query" {
		t.Errorf("Expected query entity type 'query', got '%s'", queryEntity.Type)
	}

	networkEntity, err := memorySystem.GetEntity("test-network")
	if err != nil {
		t.Fatalf("Network entity not found: %v", err)
	}

	if networkEntity.Type != "network" {
		t.Errorf("Expected network entity type 'network', got '%s'", networkEntity.Type)
	}

	// Verify relations were created
	relations, err := memorySystem.GetRelations(queryEntity.ID, "executed_on")
	if err != nil {
		t.Fatalf("Failed to get relations: %v", err)
	}

	if len(relations) != 1 {
		t.Errorf("Expected 1 'executed_on' relation, got %d", len(relations))
	}

	if relations[0].ToID != networkEntity.ID {
		t.Errorf("Expected relation to network entity %s, got %s", networkEntity.ID, relations[0].ToID)
	}

	// Verify observations were added
	observations, err := memorySystem.GetObservations(queryEntity.ID, "performance")
	if err != nil {
		t.Fatalf("Failed to get observations: %v", err)
	}

	if len(observations) != 1 {
		t.Errorf("Expected 1 performance observation, got %d", len(observations))
	}

	if !contains(observations[0].Content, "150ms") {
		t.Errorf("Expected observation to contain execution time, got: %s", observations[0].Content)
	}
}

func TestAPIMemoryTracker_TrackDeviceDiscovery(t *testing.T) {
	logger := logger.New()
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	tracker := NewAPIMemoryTracker(memorySystem, logger, "test-instance")

	// Mock devices
	devices := []forward.Device{
		{
			Name:          "router1",
			Type:          "ROUTER",
			Vendor:        "CISCO",
			Model:         "ISR4331",
			Platform:      "cisco_ios",
			OSVersion:     "16.9.04",
			ManagementIPs: []string{"192.168.1.1"},
		},
		{
			Name:     "switch1",
			Type:     "SWITCH",
			Vendor:   "CISCO",
			Model:    "N9K-C93180YC-EX",
			Platform: "cisco_nxos",
		},
	}

	// Track device discovery
	err := tracker.TrackDeviceDiscovery("test-network", devices)
	if err != nil {
		t.Fatalf("Failed to track device discovery: %v", err)
	}

	// Verify network entity was created
	networkEntity, err := memorySystem.GetEntity("test-network")
	if err != nil {
		t.Fatalf("Network entity not found: %v", err)
	}

	// Verify device entities were created
	router1Entity, err := memorySystem.GetEntity("router1")
	if err != nil {
		t.Fatalf("Router1 entity not found: %v", err)
	}

	if router1Entity.Type != "device" {
		t.Errorf("Expected device entity type 'device', got '%s'", router1Entity.Type)
	}

	if router1Entity.Metadata["vendor"] != "CISCO" {
		t.Errorf("Expected vendor 'CISCO', got '%v'", router1Entity.Metadata["vendor"])
	}

	// Verify device-network relations
	relations, err := memorySystem.GetRelations(router1Entity.ID, "belongs_to")
	if err != nil {
		t.Fatalf("Failed to get device relations: %v", err)
	}

	if len(relations) != 1 {
		t.Errorf("Expected 1 'belongs_to' relation, got %d", len(relations))
	}

	if relations[0].ToID != networkEntity.ID {
		t.Errorf("Expected device to belong to network %s, got %s", networkEntity.ID, relations[0].ToID)
	}

	// Verify network observation about device discovery
	observations, err := memorySystem.GetObservations(networkEntity.ID, "device_discovery")
	if err != nil {
		t.Fatalf("Failed to get network observations: %v", err)
	}

	if len(observations) != 1 {
		t.Errorf("Expected 1 device discovery observation, got %d", len(observations))
	}

	if !contains(observations[0].Content, "2 devices") {
		t.Errorf("Expected observation to mention 2 devices, got: %s", observations[0].Content)
	}
}

func TestAPIMemoryTracker_TrackPathSearch(t *testing.T) {
	logger := logger.New()
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	tracker := NewAPIMemoryTracker(memorySystem, logger, "test-instance")

	// Mock path search response
	response := &forward.PathSearchResponse{
		Paths: []forward.Path{
			{
				Hops: []forward.Hop{
					{Device: "router1", Action: "forward"},
					{Device: "switch1", Action: "deliver"},
				},
				Outcome:     "delivered",
				OutcomeType: "success",
			},
		},
		SnapshotID:         "test-snapshot",
		SearchTimeMs:       45,
		NumCandidatesFound: 1,
	}

	// Track path search
	err := tracker.TrackPathSearch("test-network", "10.1.1.1", "10.2.2.2", response)
	if err != nil {
		t.Fatalf("Failed to track path search: %v", err)
	}

	// Verify path search entity was created
	entities, err := memorySystem.SearchEntities("path_search", "path_search", 10)
	if err != nil {
		t.Fatalf("Failed to search for path search entities: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("Expected 1 path search entity, got %d", len(entities))
	}

	searchEntity := entities[0]
	if searchEntity.Metadata["src_ip"] != "10.1.1.1" {
		t.Errorf("Expected src_ip '10.1.1.1', got '%v'", searchEntity.Metadata["src_ip"])
	}

	if searchEntity.Metadata["dst_ip"] != "10.2.2.2" {
		t.Errorf("Expected dst_ip '10.2.2.2', got '%v'", searchEntity.Metadata["dst_ip"])
	}

	if pathCountFloat, ok := searchEntity.Metadata["path_count"].(float64); !ok || int(pathCountFloat) != 1 {
		t.Errorf("Expected path_count 1, got %v", searchEntity.Metadata["path_count"])
	}

	// Verify observations were added
	observations, err := memorySystem.GetObservations(searchEntity.ID, "search_result")
	if err != nil {
		t.Fatalf("Failed to get search observations: %v", err)
	}

	if len(observations) != 1 {
		t.Errorf("Expected 1 search result observation, got %d", len(observations))
	}

	if !contains(observations[0].Content, "successful") {
		t.Errorf("Expected observation to mention successful search, got: %s", observations[0].Content)
	}
}

func TestAPIMemoryTracker_GetQueryAnalytics(t *testing.T) {
	logger := logger.New()
	memorySystem := createTestMemorySystem(t)
	defer memorySystem.Close()

	tracker := NewAPIMemoryTracker(memorySystem, logger, "test-instance")

	// Track multiple queries
	result1 := &forward.NQERunResult{
		Items: []map[string]interface{}{{"device": "router1"}},
	}
	result2 := &forward.NQERunResult{
		Items: []map[string]interface{}{{"device": "router1"}, {"device": "switch1"}},
	}

	tracker.TrackNetworkQuery("query1", "test-network", "snapshot1", result1, 100*time.Millisecond)
	tracker.TrackNetworkQuery("query2", "test-network", "snapshot1", result2, 200*time.Millisecond)

	// Get analytics
	analytics, err := tracker.GetQueryAnalytics("test-network")
	if err != nil {
		t.Fatalf("Failed to get query analytics: %v", err)
	}

	if analytics["query_count"] != 2 {
		t.Errorf("Expected query count 2, got %v", analytics["query_count"])
	}

	if analytics["avg_execution_time_ms"] != int64(150) {
		t.Errorf("Expected avg execution time 150ms, got %v", analytics["avg_execution_time_ms"])
	}

	if analytics["total_results"] != 3 {
		t.Errorf("Expected total results 3, got %v", analytics["total_results"])
	}

	if analytics["avg_result_count"] != 1 { // (1+2)/2 = 1.5, but integer division gives 1
		t.Errorf("Expected avg result count 1, got %v", analytics["avg_result_count"])
	}
}

func TestAPIMemoryTracker_NilMemorySystem(t *testing.T) {
	logger := logger.New()
	tracker := NewAPIMemoryTracker(nil, logger, "test-instance")

	// All tracking methods should handle nil memory system gracefully
	result := &forward.NQERunResult{Items: []map[string]interface{}{}}

	err := tracker.TrackNetworkQuery("query", "network", "snapshot", result, 100*time.Millisecond)
	if err != nil {
		t.Errorf("TrackNetworkQuery should handle nil memory system, got error: %v", err)
	}

	err = tracker.TrackDeviceDiscovery("network", []forward.Device{})
	if err != nil {
		t.Errorf("TrackDeviceDiscovery should handle nil memory system, got error: %v", err)
	}

	response := &forward.PathSearchResponse{}
	err = tracker.TrackPathSearch("network", "1.1.1.1", "2.2.2.2", response)
	if err != nil {
		t.Errorf("TrackPathSearch should handle nil memory system, got error: %v", err)
	}

	_, err = tracker.GetQueryAnalytics("network")
	if err == nil {
		t.Error("GetQueryAnalytics should return error for nil memory system")
	}
}
