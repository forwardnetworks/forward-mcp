package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/forward-mcp/internal/domain"
)

// Arguments for get_nqe_result_chunks tool
// Either entity_id or (query_id, network_id, snapshot_id) must be provided
// Optionally, chunk_index can be used to fetch a single chunk
// If chunk_index is omitted, all chunks are returned
type GetNQEResultChunksArgs struct {
	EntityID   string `json:"entity_id,omitempty" jsonschema:"Entity ID containing the NQE results"`
	QueryID    string `json:"query_id,omitempty" jsonschema:"Query ID that was executed"`
	NetworkID  string `json:"network_id,omitempty" jsonschema:"Network ID where the query was run"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID used for the query"`
	ChunkIndex *int   `json:"chunk_index,omitempty" jsonschema:"Specific chunk index to retrieve (omit for all chunks)"`
}

// CreateEntity creates a new entity in the knowledge graph
func (s *Service) CreateEntity(ctx context.Context, args CreateEntityArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Validate required fields
	if err := s.validateEntityName(args.Name); err != nil {
		return nil, err
	}
	if err := s.validateEntityType(args.Type); err != nil {
		return nil, err
	}

	entity, err := s.memorySystem.CreateEntity(args.Name, args.Type, args.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	entityJSON, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entity: %w", err)
	}

	return textResult(fmt.Sprintf("Entity created successfully:\n%s", string(entityJSON))), nil
}

// CreateRelation creates a relation between two entities
func (s *Service) CreateRelation(ctx context.Context, args CreateRelationArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	relation, err := s.memorySystem.CreateRelation(args.FromID, args.ToID, args.Type, args.Properties)
	if err != nil {
		return nil, fmt.Errorf("failed to create relation: %w", err)
	}

	relationJSON, err := json.MarshalIndent(relation, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal relation: %w", err)
	}

	return textResult(fmt.Sprintf("Relation created successfully:\n%s", string(relationJSON))), nil
}

// AddObservation adds an observation to an entity
func (s *Service) AddObservation(ctx context.Context, args AddObservationArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	observation, err := s.memorySystem.AddObservation(args.EntityID, args.Content, args.Type, args.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to add observation: %w", err)
	}

	observationJSON, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal observation: %w", err)
	}

	return textResult(fmt.Sprintf("Observation added successfully:\n%s", string(observationJSON))), nil
}

// SearchEntities searches for entities in the knowledge graph with automatic bloom filter optimization
func (s *Service) SearchEntities(ctx context.Context, args SearchEntitiesArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Check if we have bloom filters available for NQE result entities
	if args.EntityType == "nqe_result" && s.bloomManager != nil {
		// Try to use bloom filter for faster searching
		networkID := s.getNetworkID("")
		if networkID != "" {
			// Check if we have any bloom filters for this network
			stats := s.bloomManager.GetFilterStats()
			for filterKey, metadata := range stats {
				if strings.Contains(filterKey, networkID) {
					// We have a bloom filter, use it for searching
					s.logger.Debug("Using bloom filter for entity search: %s", filterKey)

					// Extract search terms from the query
					searchTerms := s.extractSearchTerms(args.Query)
					if len(searchTerms) > 0 {
						// Use bloom filter search
						filterType := metadata.FilterType
						searchResult, err := s.bloomManager.SearchFilter(networkID, filterType, searchTerms, nil)
						if err == nil && searchResult.MatchedCount > 0 {
							// Bloom filter found matches, now get the actual entities
							entities, err := s.memorySystem.SearchEntities(args.Query, args.EntityType, args.Limit)
							if err != nil {
								return nil, fmt.Errorf("failed to search entities after bloom filter: %w", err)
							}

							response := fmt.Sprintf("🔍 Bloom filter search completed in %v!\n", searchResult.SearchTime)
							response += fmt.Sprintf("📊 Found %d potential matches (bloom filter)\n", searchResult.MatchedCount)
							response += fmt.Sprintf("📋 Retrieved %d entities:\n", len(entities))

							entitiesJSON, err := json.MarshalIndent(entities, "", "  ")
							if err != nil {
								return nil, fmt.Errorf("failed to marshal entities: %w", err)
							}
							response += string(entitiesJSON)

							return textResult(response), nil
						}
					}
				}
			}
		}
	}

	// Fallback to regular search
	entities, err := s.memorySystem.SearchEntities(args.Query, args.EntityType, args.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}

	if len(entities) == 0 {
		return textResult("No entities found matching the search criteria."), nil
	}

	entitiesJSON, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entities: %w", err)
	}

	return textResult(fmt.Sprintf("Found %d entities:\n%s", len(entities), string(entitiesJSON))), nil
}

// GetEntity retrieves a specific entity by ID or name
func (s *Service) GetEntity(ctx context.Context, args GetEntityArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	entity, err := s.memorySystem.GetEntity(args.Identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	entityJSON, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entity: %w", err)
	}

	return textResult(fmt.Sprintf("Entity found:\n%s", string(entityJSON))), nil
}

// GetRelations retrieves relations for an entity
func (s *Service) GetRelations(ctx context.Context, args GetRelationsArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Get all relations from memory system
	allRelations, err := s.memorySystem.GetRelations(args.EntityID, args.RelationType)
	if err != nil {
		return nil, fmt.Errorf("failed to get relations: %w", err)
	}

	// Apply pagination with safe defaults to prevent token overflow
	limit := args.Limit
	if limit <= 0 {
		limit = 25 // Conservative default limit to prevent token overflow
	}
	if limit > 100 {
		limit = 100 // Cap at 100 to prevent excessive responses
	}
	offset := args.Offset
	if offset < 0 {
		offset = 0
	}

	var relations []*domain.Relation
	var totalCount int
	var hasMore bool

	if args.AllResults {
		// Store all relations in memory system for large datasets
		relations = allRelations
		totalCount = len(allRelations)
		hasMore = false

		// Store in memory system if available
		entity, err := s.memorySystem.CreateEntity("relations_list", "query_result", map[string]interface{}{
			"query_type":    "get_relations",
			"entity_id":     args.EntityID,
			"relation_type": args.RelationType,
			"total_count":   totalCount,
			"timestamp":     time.Now().Unix(),
		})
		if err == nil {
			// Store the relations data
			relationsJSON, _ := json.Marshal(relations)
			s.memorySystem.AddObservation(entity.ID, string(relationsJSON), "data", map[string]interface{}{
				"data_type": "relations_list",
				"count":     totalCount,
			})
		}
	} else {
		// Apply pagination
		totalCount = len(allRelations)
		start := offset
		end := offset + limit
		if start >= totalCount {
			relations = []*domain.Relation{}
		} else {
			if end > totalCount {
				end = totalCount
			}
			relations = allRelations[start:end]
		}
		hasMore = offset+len(relations) < totalCount
	}

	// Build response
	var responseText strings.Builder
	if totalCount == 0 {
		responseText.WriteString("No relations found for this entity.")
	} else {
		responseText.WriteString(fmt.Sprintf("Found %d relations", totalCount))
		if !args.AllResults {
			responseText.WriteString(fmt.Sprintf(" (showing %d-%d)", offset+1, offset+len(relations)))
			if hasMore {
				responseText.WriteString(fmt.Sprintf(", %d more available", totalCount-offset-len(relations)))
			}
		}
		responseText.WriteString(":\n")

		if len(relations) > 0 {
			relationsJSON, err := json.MarshalIndent(relations, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal relations: %w", err)
			}
			responseText.WriteString(string(relationsJSON))
		}
	}

	if args.AllResults && s.memorySystem != nil {
		responseText.WriteString(fmt.Sprintf("\n\n💾 Stored %d relations in memory system for future reference.", totalCount))
	}

	return textResult(responseText.String()), nil
}

// GetObservations retrieves observations for an entity
func (s *Service) GetObservations(ctx context.Context, args GetObservationsArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Get all observations from memory system
	allObservations, err := s.memorySystem.GetObservations(args.EntityID, args.ObservationType)
	if err != nil {
		return nil, fmt.Errorf("failed to get observations: %w", err)
	}

	// Apply pagination with safe defaults to prevent token overflow
	limit := args.Limit
	if limit <= 0 {
		limit = 25 // Conservative default limit to prevent token overflow
	}
	if limit > 100 {
		limit = 100 // Cap at 100 to prevent excessive responses
	}
	offset := args.Offset
	if offset < 0 {
		offset = 0
	}

	var observations []*domain.Observation
	var totalCount int
	var hasMore bool

	if args.AllResults {
		// Store all observations in memory system for large datasets
		observations = allObservations
		totalCount = len(allObservations)
		hasMore = false

		// Store in memory system if available
		entity, err := s.memorySystem.CreateEntity("observations_list", "query_result", map[string]interface{}{
			"query_type":       "get_observations",
			"entity_id":        args.EntityID,
			"observation_type": args.ObservationType,
			"total_count":      totalCount,
			"timestamp":        time.Now().Unix(),
		})
		if err == nil {
			// Store the observations data
			observationsJSON, _ := json.Marshal(observations)
			s.memorySystem.AddObservation(entity.ID, string(observationsJSON), "data", map[string]interface{}{
				"data_type": "observations_list",
				"count":     totalCount,
			})
		}
	} else {
		// Apply pagination
		totalCount = len(allObservations)
		start := offset
		end := offset + limit
		if start >= totalCount {
			observations = []*domain.Observation{}
		} else {
			if end > totalCount {
				end = totalCount
			}
			observations = allObservations[start:end]
		}
		hasMore = offset+len(observations) < totalCount
	}

	// Build response
	var responseText strings.Builder
	if totalCount == 0 {
		responseText.WriteString("No observations found for this entity.")
	} else {
		responseText.WriteString(fmt.Sprintf("Found %d observations", totalCount))
		if !args.AllResults {
			responseText.WriteString(fmt.Sprintf(" (showing %d-%d)", offset+1, offset+len(observations)))
			if hasMore {
				responseText.WriteString(fmt.Sprintf(", %d more available", totalCount-offset-len(observations)))
			}
		}
		responseText.WriteString(":\n")

		if len(observations) > 0 {
			observationsJSON, err := json.MarshalIndent(observations, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal observations: %w", err)
			}
			responseText.WriteString(string(observationsJSON))
		}
	}

	if args.AllResults && s.memorySystem != nil {
		responseText.WriteString(fmt.Sprintf("\n\n💾 Stored %d observations in memory system for future reference.", totalCount))
	}

	return textResult(responseText.String()), nil
}

// DeleteEntity deletes an entity and all its relations and observations
func (s *Service) DeleteEntity(ctx context.Context, args DeleteEntityArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Get entity details before deletion for confirmation
	entity, err := s.memorySystem.GetEntity(args.EntityID)
	if err != nil {
		return nil, fmt.Errorf("entity not found: %w", err)
	}

	err = s.memorySystem.DeleteEntity(args.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete entity: %w", err)
	}

	return textResult(fmt.Sprintf("Entity '%s' (%s) deleted successfully, including all its relations and observations.", entity.Name, entity.Type)), nil
}

// DeleteRelation deletes a specific relation
func (s *Service) DeleteRelation(ctx context.Context, args DeleteRelationArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	err := s.memorySystem.DeleteRelation(args.RelationID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete relation: %w", err)
	}

	return textResult(fmt.Sprintf("Relation '%s' deleted successfully.", args.RelationID)), nil
}

// DeleteObservation deletes a specific observation
func (s *Service) DeleteObservation(ctx context.Context, args DeleteObservationArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	err := s.memorySystem.DeleteObservation(args.ObservationID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete observation: %w", err)
	}

	return textResult(fmt.Sprintf("Observation '%s' deleted successfully.", args.ObservationID)), nil
}

// GetMemoryStats returns statistics about the memory system
func (s *Service) GetMemoryStats(ctx context.Context, args GetMemoryStatsArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	stats, err := s.memorySystem.GetMemoryStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	return textResult(fmt.Sprintf("Memory system statistics:\n%s", string(statsJSON))), nil
}

// GetQueryAnalytics gets analytics about query patterns for a network
func (s *Service) GetQueryAnalytics(ctx context.Context, args GetQueryAnalyticsArgs) (*Result, error) {
	if s.apiTracker == nil {
		return nil, fmt.Errorf("API memory tracker is not available")
	}

	analytics, err := s.apiTracker.GetQueryAnalytics(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get query analytics: %w", err)
	}

	analyticsJSON, err := json.MarshalIndent(analytics, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analytics: %w", err)
	}

	return textResult(fmt.Sprintf("Query analytics for network %s:\n%s", args.NetworkID, string(analyticsJSON))), nil
}

// GetNQEResultChunks retrieves chunked NQE query results from the memory system
func (s *Service) GetNQEResultChunks(ctx context.Context, args GetNQEResultChunksArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	entityID := args.EntityID
	// If entity_id is not provided, try to look up by query_id/network_id/snapshot_id
	if entityID == "" && args.QueryID != "" && args.NetworkID != "" && args.SnapshotID != "" {
		lookupName := fmt.Sprintf("%s-%s-%s", args.QueryID, args.NetworkID, args.SnapshotID)
		entity, err := s.memorySystem.GetEntityByName(lookupName)
		if err != nil {
			return nil, fmt.Errorf("could not find result entity for query/network/snapshot: %w", err)
		}
		entityID = entity.ID
	}

	if entityID == "" {
		return nil, fmt.Errorf("must provide either entity_id or (query_id, network_id, snapshot_id)")
	}

	chunks, err := s.memorySystem.GetNQEResultChunks(entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve result chunks: %w", err)
	}

	// If chunk_index is provided, return only that chunk
	if args.ChunkIndex != nil {
		idx := *args.ChunkIndex
		if idx < 0 || idx >= len(chunks) {
			return nil, fmt.Errorf("chunk_index %d out of range (total chunks: %d)", idx, len(chunks))
		}
		return textResult(chunks[idx]), nil
	}

	// Otherwise, return all chunks as a JSON array
	chunksJSON, _ := json.Marshal(chunks)
	return textResult(string(chunksJSON)), nil
}

// Add get_nqe_result_summary tool handler
// Arguments: entity_id OR (query_id, network_id, snapshot_id)
func (s *Service) GetNQEResultSummary(ctx context.Context, args GetNQEResultChunksArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}
	entityID := args.EntityID
	if entityID == "" && args.QueryID != "" && args.NetworkID != "" && args.SnapshotID != "" {
		lookupName := fmt.Sprintf("%s-%s-%s", args.QueryID, args.NetworkID, args.SnapshotID)
		entity, err := s.memorySystem.GetEntityByName(lookupName)
		if err != nil {
			return nil, fmt.Errorf("could not find result entity for query/network/snapshot: %w", err)
		}
		entityID = entity.ID
	}
	if entityID == "" {
		return nil, fmt.Errorf("must provide either entity_id or (query_id, network_id, snapshot_id)")
	}
	// Get summary observation
	obs, err := s.memorySystem.GetObservations(entityID, "nqe_result_summary")
	if err != nil || len(obs) == 0 {
		return nil, fmt.Errorf("no summary found for entity %s", entityID)
	}

	response := fmt.Sprintf("NQE result summary for entity %s:\n%s", entityID, obs[0].Content)

	// Check if bloom filter is available for this data
	if s.bloomManager != nil {
		networkID := s.getNetworkID(args.NetworkID)
		if networkID != "" {
			stats := s.bloomManager.GetFilterStats()
			for filterKey, metadata := range stats {
				if strings.Contains(filterKey, networkID) {
					response += fmt.Sprintf("\n\n🔍 Bloom Filter Available!\n")
					response += fmt.Sprintf("- Filter Type: %s\n", metadata.FilterType)
					response += fmt.Sprintf("- Items Indexed: %d\n", metadata.ItemCount)
					response += fmt.Sprintf("- Memory Usage: %s\n", formatBytes(metadata.MemoryUsage))
					response += fmt.Sprintf("- Last Updated: %v\n", metadata.LastUpdated)
					response += fmt.Sprintf("\n💡 Use search_bloom_filter for sub-millisecond searches!")
					break
				}
			}
		}
	}

	return textResult(response), nil
}

// Add analyze_nqe_result_sql tool handler
type AnalyzeNQEResultSQLArgs struct {
	EntityID string `json:"entity_id" jsonschema:"Entity ID containing the NQE results to analyze"`
	SQLQuery string `json:"sql_query" jsonschema:"SQL query to execute against the NQE results"`
}

func (s *Service) AnalyzeNQEResultSQL(ctx context.Context, args AnalyzeNQEResultSQLArgs) (*Result, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}
	if args.EntityID == "" || args.SQLQuery == "" {
		return nil, fmt.Errorf("entity_id and sql_query are required")
	}
	// Get all chunks for the entity
	chunks, err := s.memorySystem.GetNQEResultChunks(args.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve result chunks: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no data found for entity %s", args.EntityID)
	}
	// Parse all rows from all chunks
	var allRows []map[string]interface{}
	for _, chunk := range chunks {
		var rows []map[string]interface{}
		if err := json.Unmarshal([]byte(chunk), &rows); err != nil {
			return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
		}
		allRows = append(allRows, rows...)
	}
	if len(allRows) == 0 {
		return nil, fmt.Errorf("no rows found for entity %s", args.EntityID)
	}
	// Run the query (limit to 100 rows)
	query := args.SQLQuery
	if !strings.Contains(strings.ToLower(query), "limit") {
		query += " LIMIT 100"
	}
	if s.rowQuerier == nil {
		return nil, fmt.Errorf("SQL analysis is not available")
	}
	resultRows, err := s.rowQuerier.QueryRows(ctx, allRows, query)
	if err != nil {
		return nil, err
	}
	resultJSON, _ := json.MarshalIndent(resultRows, "", "  ")
	response := fmt.Sprintf("SQL query result (%d rows, max 100 shown):\n%s", len(resultRows), string(resultJSON))
	return textResult(response), nil
}
