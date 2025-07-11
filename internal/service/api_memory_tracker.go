package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
)

// APIMemoryTracker integrates the memory system with API result tracking
type APIMemoryTracker struct {
	memorySystem *MemorySystem
	logger       *logger.Logger
	instanceID   string
}

// NewAPIMemoryTracker creates a new API memory tracker
func NewAPIMemoryTracker(memorySystem *MemorySystem, logger *logger.Logger, instanceID string) *APIMemoryTracker {
	return &APIMemoryTracker{
		memorySystem: memorySystem,
		logger:       logger,
		instanceID:   instanceID,
	}
}

// TrackNetworkQuery tracks when a query is executed on a network
func (amt *APIMemoryTracker) TrackNetworkQuery(queryID, networkID, snapshotID string, result *forward.NQERunResult, executionTime time.Duration) error {
	if amt.memorySystem == nil {
		return nil // Memory system not available
	}

	// Create or get network entity
	networkEntity, err := amt.ensureNetworkEntity(networkID)
	if err != nil {
		amt.logger.Warn("Failed to create network entity: %v", err)
		return err
	}

	// Create or get query entity
	queryEntity, err := amt.ensureQueryEntity(queryID)
	if err != nil {
		amt.logger.Warn("Failed to create query entity: %v", err)
		return err
	}

	// Create or get snapshot entity if provided
	var snapshotEntity *Entity
	if snapshotID != "" {
		snapshotEntity, err = amt.ensureSnapshotEntity(snapshotID, networkID)
		if err != nil {
			amt.logger.Warn("Failed to create snapshot entity: %v", err)
		}
	}

	// Create query execution result entity
	resultEntity, err := amt.createQueryResultEntity(queryID, networkID, snapshotID, result, executionTime)
	if err != nil {
		amt.logger.Warn("Failed to create result entity: %v", err)
		return err
	}

	// Create relationships
	relations := []struct {
		fromID, toID, relationType string
		properties                 map[string]interface{}
	}{
		{queryEntity.ID, networkEntity.ID, "executed_on", map[string]interface{}{
			"timestamp":      time.Now().Unix(),
			"execution_time": executionTime.Milliseconds(),
		}},
		{queryEntity.ID, resultEntity.ID, "produced", map[string]interface{}{
			"result_count": len(result.Items),
			"timestamp":    time.Now().Unix(),
		}},
	}

	if snapshotEntity != nil {
		relations = append(relations, struct {
			fromID, toID, relationType string
			properties                 map[string]interface{}
		}{
			queryEntity.ID, snapshotEntity.ID, "executed_at_snapshot", map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
		})
	}

	// Create all relationships
	for _, rel := range relations {
		_, err := amt.memorySystem.CreateRelation(rel.fromID, rel.toID, rel.relationType, rel.properties)
		if err != nil {
			amt.logger.Debug("Failed to create relation %s->%s (%s): %v", rel.fromID, rel.toID, rel.relationType, err)
		}
	}

	// Add performance observation
	perfMetadata := map[string]interface{}{
		"execution_time_ms": executionTime.Milliseconds(),
		"result_count":      len(result.Items),
		"snapshot_id":       snapshotID,
		"network_id":        networkID,
		"timestamp":         time.Now().Unix(),
	}

	_, err = amt.memorySystem.AddObservation(
		queryEntity.ID,
		fmt.Sprintf("Query executed in %dms, returned %d items", executionTime.Milliseconds(), len(result.Items)),
		"performance",
		perfMetadata,
	)

	if err != nil {
		amt.logger.Debug("Failed to add performance observation: %v", err)
	}

	amt.logger.Debug("Tracked query execution: %s on network %s (results: %d, time: %dms)",
		queryID, networkID, len(result.Items), executionTime.Milliseconds())

	return nil
}

// TrackDeviceDiscovery tracks when devices are discovered in a network
func (amt *APIMemoryTracker) TrackDeviceDiscovery(networkID string, devices []forward.Device) error {
	if amt.memorySystem == nil || len(devices) == 0 {
		return nil
	}

	// Ensure network entity exists
	networkEntity, err := amt.ensureNetworkEntity(networkID)
	if err != nil {
		return err
	}

	deviceCount := 0
	for _, device := range devices {
		if device.Name == "" {
			continue
		}

		// Create device entity
		deviceMetadata := map[string]interface{}{
			"network_id": networkID,
			"type":       device.Type,
			"vendor":     device.Vendor,
			"model":      device.Model,
			"platform":   device.Platform,
			"os_version": device.OSVersion,
		}

		if len(device.ManagementIPs) > 0 {
			deviceMetadata["management_ip"] = device.ManagementIPs[0]
		}

		deviceEntity, err := amt.memorySystem.CreateEntity(device.Name, "device", deviceMetadata)
		if err != nil {
			// Entity might already exist, try to get it
			deviceEntity, err = amt.memorySystem.GetEntity(device.Name)
			if err != nil {
				amt.logger.Debug("Failed to create/get device entity %s: %v", device.Name, err)
				continue
			}
		}

		// Create relationship: device belongs to network
		_, err = amt.memorySystem.CreateRelation(deviceEntity.ID, networkEntity.ID, "belongs_to", map[string]interface{}{
			"discovered_at": time.Now().Unix(),
		})
		if err != nil {
			amt.logger.Debug("Failed to create device-network relation: %v", err)
		}

		deviceCount++
	}

	// Add observation about device discovery
	if deviceCount > 0 {
		_, err = amt.memorySystem.AddObservation(
			networkEntity.ID,
			fmt.Sprintf("Discovered %d devices in network", deviceCount),
			"device_discovery",
			map[string]interface{}{
				"device_count": deviceCount,
				"timestamp":    time.Now().Unix(),
			},
		)
		if err != nil {
			amt.logger.Debug("Failed to add device discovery observation: %v", err)
		}

		amt.logger.Debug("Tracked device discovery: %d devices in network %s", deviceCount, networkID)
	}

	return nil
}

// TrackPathSearch tracks path search results
func (amt *APIMemoryTracker) TrackPathSearch(networkID, srcIP, dstIP string, result *forward.PathSearchResponse) error {
	if amt.memorySystem == nil {
		return nil
	}

	// Create path search entity
	searchMetadata := map[string]interface{}{
		"network_id":       networkID,
		"src_ip":           srcIP,
		"dst_ip":           dstIP,
		"path_count":       len(result.Paths),
		"search_time_ms":   result.SearchTimeMs,
		"candidates_found": result.NumCandidatesFound,
		"snapshot_id":      result.SnapshotID,
		"timestamp":        time.Now().Unix(),
	}

	searchEntity, err := amt.memorySystem.CreateEntity(
		fmt.Sprintf("path_search_%s_to_%s", srcIP, dstIP),
		"path_search",
		searchMetadata,
	)
	if err != nil {
		amt.logger.Debug("Failed to create path search entity: %v", err)
		return err
	}

	// Ensure network entity exists and relate to search
	networkEntity, err := amt.ensureNetworkEntity(networkID)
	if err == nil {
		_, err = amt.memorySystem.CreateRelation(searchEntity.ID, networkEntity.ID, "performed_on", map[string]interface{}{
			"timestamp": time.Now().Unix(),
		})
		if err != nil {
			amt.logger.Debug("Failed to create search-network relation: %v", err)
		}
	}

	// Add observation about path search results
	var outcome string
	if len(result.Paths) > 0 {
		outcome = "successful"
	} else {
		outcome = "no_paths_found"
	}

	_, err = amt.memorySystem.AddObservation(
		searchEntity.ID,
		fmt.Sprintf("Path search %s: %d paths found in %dms", outcome, len(result.Paths), result.SearchTimeMs),
		"search_result",
		searchMetadata,
	)

	if err != nil {
		amt.logger.Debug("Failed to add path search observation: %v", err)
	}

	amt.logger.Debug("Tracked path search: %s->%s on network %s (%d paths, %dms)",
		srcIP, dstIP, networkID, len(result.Paths), result.SearchTimeMs)

	return nil
}

// GetQueryAnalytics returns analytics about query patterns
func (amt *APIMemoryTracker) GetQueryAnalytics(networkID string) (map[string]interface{}, error) {
	if amt.memorySystem == nil {
		return nil, fmt.Errorf("memory system not available")
	}

	analytics := make(map[string]interface{})

	// Get network entity
	networkEntity, err := amt.memorySystem.GetEntity(networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	// Get relations where entities executed on this network (incoming relations)
	allEntities, err := amt.memorySystem.SearchEntities("", "", 1000) // Get all entities to check their relations
	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}

	queryCount := 0
	totalExecutionTime := int64(0)
	resultCounts := []int{}

	// Check all entities for relations to this network
	for _, entity := range allEntities {
		if entity.Type == "query" {
			relations, err := amt.memorySystem.GetRelations(entity.ID, "executed_on")
			if err != nil {
				continue
			}

			for _, relation := range relations {
				if relation.ToID == networkEntity.ID {
					queryCount++
					if execTime, ok := relation.Properties["execution_time"].(float64); ok {
						totalExecutionTime += int64(execTime)
					}
				}
			}

			// Get produced relations to count results
			producedRelations, err := amt.memorySystem.GetRelations(entity.ID, "produced")
			if err != nil {
				continue
			}

			for _, relation := range producedRelations {
				if count, ok := relation.Properties["result_count"].(float64); ok {
					resultCounts = append(resultCounts, int(count))
				}
			}
		}
	}

	analytics["query_count"] = queryCount
	if queryCount > 0 {
		analytics["avg_execution_time_ms"] = totalExecutionTime / int64(queryCount)
	} else {
		analytics["avg_execution_time_ms"] = 0
	}

	if len(resultCounts) > 0 {
		totalResults := 0
		for _, count := range resultCounts {
			totalResults += count
		}
		analytics["avg_result_count"] = totalResults / len(resultCounts)
		analytics["total_results"] = totalResults
	} else {
		analytics["avg_result_count"] = 0
		analytics["total_results"] = 0
	}

	// Get recent observations
	observations, err := amt.memorySystem.GetObservations(networkEntity.ID, "")
	if err == nil {
		analytics["recent_observations"] = len(observations)
	}

	return analytics, nil
}

// Helper methods for entity management

func (amt *APIMemoryTracker) ensureNetworkEntity(networkID string) (*Entity, error) {
	// Try to get existing network entity
	entity, err := amt.memorySystem.GetEntity(networkID)
	if err == nil {
		return entity, nil
	}

	// Create new network entity
	metadata := map[string]interface{}{
		"network_id":    networkID,
		"discovered_at": time.Now().Unix(),
	}

	return amt.memorySystem.CreateEntity(networkID, "network", metadata)
}

func (amt *APIMemoryTracker) ensureQueryEntity(queryID string) (*Entity, error) {
	// Try to get existing query entity
	entity, err := amt.memorySystem.GetEntity(queryID)
	if err == nil {
		return entity, nil
	}

	// Create new query entity
	metadata := map[string]interface{}{
		"query_id":   queryID,
		"first_seen": time.Now().Unix(),
	}

	return amt.memorySystem.CreateEntity(queryID, "query", metadata)
}

func (amt *APIMemoryTracker) ensureSnapshotEntity(snapshotID, networkID string) (*Entity, error) {
	// Try to get existing snapshot entity
	entity, err := amt.memorySystem.GetEntity(snapshotID)
	if err == nil {
		return entity, nil
	}

	// Create new snapshot entity
	metadata := map[string]interface{}{
		"snapshot_id":   snapshotID,
		"network_id":    networkID,
		"discovered_at": time.Now().Unix(),
	}

	return amt.memorySystem.CreateEntity(snapshotID, "snapshot", metadata)
}

func (amt *APIMemoryTracker) createQueryResultEntity(queryID, networkID, snapshotID string, result *forward.NQERunResult, executionTime time.Duration) (*Entity, error) {
	// Create unique result ID
	resultID := fmt.Sprintf("result_%s_%s_%d", queryID, networkID, time.Now().Unix())

	// Calculate result size
	resultBytes, _ := json.Marshal(result)
	resultSize := len(resultBytes)

	metadata := map[string]interface{}{
		"query_id":       queryID,
		"network_id":     networkID,
		"snapshot_id":    snapshotID,
		"result_count":   len(result.Items),
		"result_size":    resultSize,
		"execution_time": executionTime.Milliseconds(),
		"timestamp":      time.Now().Unix(),
	}

	return amt.memorySystem.CreateEntity(resultID, "query_result", metadata)
}
