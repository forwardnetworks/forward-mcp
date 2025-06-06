package service

import (
	"fmt"
	"strings"
)

// ExecutableQuery represents a query that can actually be executed via the Forward Networks API
type ExecutableQuery struct {
	QueryID     string   `json:"query_id"`    // Real Forward Networks GlobalQueryId (FQ_...)
	Name        string   `json:"name"`        // Human-readable name
	Description string   `json:"description"` // What the query does
	Category    string   `json:"category"`    // Category for organization
	Keywords    []string `json:"keywords"`    // Keywords for search matching
	WhenToUse   string   `json:"when_to_use"` // Guidance on when to use this query
	// New fields for semantic mapping
	SemanticKeywords []string `json:"semantic_keywords"` // Additional keywords for semantic matching
	RelatedQueries   []string `json:"related_queries"`   // Query paths that map to this executable query
}

// GetExecutableQueries returns the curated list of queries that can actually be executed
// These are the queries with real Forward Networks GlobalQueryIds that work with the API
func GetExecutableQueries() []ExecutableQuery {
	return []ExecutableQuery{
		{
			QueryID:          "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029",
			Name:             "Device Basic Info",
			Description:      "Get basic device information including names, platforms, management IPs, and device types",
			Category:         "Device Management",
			Keywords:         []string{"device", "inventory", "basic", "info", "platform", "management", "ip"},
			WhenToUse:        "Use for device discovery, inventory management, and getting overview of network devices",
			SemanticKeywords: []string{"device list", "device inventory", "show devices", "network devices", "router list", "switch list", "device details", "host inventory"},
			RelatedQueries:   []string{"device_basic_info", "device_inventory", "device_list", "network_devices"},
		},
		{
			QueryID:          "FQ_7ec4a8148b48a91271f342c512b2af1cdb276744",
			Name:             "Device Hardware",
			Description:      "Get detailed hardware information including models, serial numbers, and hardware specifications",
			Category:         "Hardware Management",
			Keywords:         []string{"hardware", "device", "model", "serial", "specifications", "equipment"},
			WhenToUse:        "Use for hardware inventory, lifecycle management, and tracking device specifications",
			SemanticKeywords: []string{"hardware info", "device models", "serial numbers", "equipment specs", "hardware inventory", "device specs"},
			RelatedQueries:   []string{"device_hardware", "hardware_inventory", "device_models", "equipment_info"},
		},
		{
			QueryID:          "FQ_f0984b777b940b4376ed3ec4317ad47437426e7c",
			Name:             "Hardware Support",
			Description:      "Get hardware support status including end-of-life dates and support information",
			Category:         "Lifecycle Management",
			Keywords:         []string{"support", "hardware", "end-of-life", "eol", "lifecycle", "maintenance"},
			WhenToUse:        "Use for compliance reporting, planning hardware refreshes, and tracking support status",
			SemanticKeywords: []string{"end of life", "eol status", "hardware support", "support dates", "lifecycle status", "maintenance windows"},
			RelatedQueries:   []string{"hardware_support", "eol_status", "lifecycle_management", "support_status"},
		},
		{
			QueryID:          "FQ_fc33d9fd70ba19a18455b0e4d26ca8420003d9cc",
			Name:             "OS Support",
			Description:      "Get operating system support status including OS versions and support dates",
			Category:         "OS Management",
			Keywords:         []string{"os", "operating", "system", "support", "version", "update", "security"},
			WhenToUse:        "Use for security compliance, OS upgrade planning, and tracking software lifecycle",
			SemanticKeywords: []string{"operating system", "os version", "software support", "os updates", "version compliance", "software lifecycle"},
			RelatedQueries:   []string{"os_support", "software_support", "os_versions", "software_lifecycle"},
		},
		{
			QueryID:          "FQ_e636c47826ad7144f09eaf6bc14dfb0b560e7cc9",
			Name:             "Configuration Search",
			Description:      "Search device configurations for specific patterns, commands, or settings",
			Category:         "Configuration Management",
			Keywords:         []string{"config", "configuration", "search", "commands", "settings", "audit"},
			WhenToUse:        "Use for configuration auditing, compliance checking, and finding specific settings",
			SemanticKeywords: []string{"config search", "find config", "configuration audit", "search commands", "config patterns", "setting search"},
			RelatedQueries:   []string{"config_search", "configuration_search", "config_audit", "command_search"},
		},
		{
			QueryID:          "FQ_51f090cbea069b4049eb283716ab3bbb3f578aea",
			Name:             "Configuration Diff",
			Description:      "Compare network configurations between snapshots to identify changes",
			Category:         "Change Management",
			Keywords:         []string{"diff", "difference", "changes", "compare", "configuration", "drift"},
			WhenToUse:        "Use for change tracking, troubleshooting configuration drift, and impact analysis",
			SemanticKeywords: []string{"config diff", "configuration changes", "compare configs", "config drift", "change tracking", "configuration comparison"},
			RelatedQueries:   []string{"config_diff", "configuration_diff", "config_changes", "change_tracking"},
		},
		{
			QueryID:          "FQ_af8404fc747f814842b8c0cee31491614b904bd5",
			Name:             "Device Utilities",
			Description:      "Get device utility information including CPU, memory, and system metrics",
			Category:         "Performance Monitoring",
			Keywords:         []string{"utilities", "cpu", "memory", "performance", "metrics", "monitoring"},
			WhenToUse:        "Use for performance monitoring, capacity planning, and system health checks",
			SemanticKeywords: []string{"cpu usage", "memory usage", "system metrics", "performance stats", "device performance", "system utilization"},
			RelatedQueries:   []string{"device_utilities", "performance_metrics", "system_metrics", "device_performance"},
		},
		// Add more executable queries as they become available
		// Each query here MUST have a real Forward Networks GlobalQueryId that works with the API
	}
}

// QueryMappingResult represents the result of mapping semantic search to executable queries
type QueryMappingResult struct {
	ExecutableQuery   *ExecutableQuery     `json:"executable_query"`   // The executable query that can be run
	SemanticMatches   []*QuerySearchResult `json:"semantic_matches"`   // Related queries found via semantic search
	MappingConfidence float64              `json:"mapping_confidence"` // How confident we are in this mapping
	MappingReason     string               `json:"mapping_reason"`     // Why this mapping was chosen
}

// SearchExecutableQueries performs keyword-based search through executable queries only
func SearchExecutableQueries(searchText string, limit int) []ExecutableQuery {
	if limit <= 0 {
		limit = 10
	}

	queries := GetExecutableQueries()
	var results []ExecutableQuery

	searchTerms := strings.ToLower(searchText)

	for _, query := range queries {
		score := calculateExecutableQueryScore(query, searchTerms)
		if score > 0 {
			results = append(results, query)
		}
	}

	// Sort by relevance (simple keyword matching for now)
	// In a more sophisticated implementation, we could use proper scoring
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// calculateExecutableQueryScore calculates relevance score for executable queries
func calculateExecutableQueryScore(query ExecutableQuery, searchTerms string) float64 {
	score := 0.0

	// Check name (highest weight)
	if strings.Contains(strings.ToLower(query.Name), searchTerms) {
		score += 3.0
	}

	// Check description (medium weight)
	if strings.Contains(strings.ToLower(query.Description), searchTerms) {
		score += 2.0
	}

	// Check keywords (medium weight)
	for _, keyword := range query.Keywords {
		if strings.Contains(strings.ToLower(keyword), searchTerms) {
			score += 1.5
		}
	}

	// Check semantic keywords (medium weight)
	for _, keyword := range query.SemanticKeywords {
		if strings.Contains(strings.ToLower(keyword), searchTerms) {
			score += 1.5
		}
	}

	// Check category (lower weight)
	if strings.Contains(strings.ToLower(query.Category), searchTerms) {
		score += 1.0
	}

	return score
}

// MapSemanticToExecutable uses semantic search results to find the best executable query
func MapSemanticToExecutable(semanticResults []*QuerySearchResult) []QueryMappingResult {
	executableQueries := GetExecutableQueries()
	var mappings []QueryMappingResult

	for _, execQuery := range executableQueries {
		var relatedMatches []*QuerySearchResult
		var totalConfidence float64
		var bestMatch *QuerySearchResult

		// Find semantic matches that relate to this executable query
		for _, semanticResult := range semanticResults {
			confidence := calculateMappingConfidence(execQuery, semanticResult)
			if confidence > 0.3 { // Threshold for considering a match
				relatedMatches = append(relatedMatches, semanticResult)
				totalConfidence += confidence
				if bestMatch == nil || confidence > calculateMappingConfidence(execQuery, bestMatch) {
					bestMatch = semanticResult
				}
			}
		}

		if len(relatedMatches) > 0 {
			avgConfidence := totalConfidence / float64(len(relatedMatches))
			reason := generateMappingReason(execQuery, bestMatch, len(relatedMatches))

			mappings = append(mappings, QueryMappingResult{
				ExecutableQuery:   &execQuery,
				SemanticMatches:   relatedMatches,
				MappingConfidence: avgConfidence,
				MappingReason:     reason,
			})
		}
	}

	// Sort by confidence (highest first)
	for i := 0; i < len(mappings); i++ {
		for j := i + 1; j < len(mappings); j++ {
			if mappings[i].MappingConfidence < mappings[j].MappingConfidence {
				mappings[i], mappings[j] = mappings[j], mappings[i]
			}
		}
	}

	return mappings
}

// calculateMappingConfidence determines how well a semantic result maps to an executable query
func calculateMappingConfidence(execQuery ExecutableQuery, semanticResult *QuerySearchResult) float64 {
	confidence := 0.0

	// Check direct keyword matches
	queryText := strings.ToLower(semanticResult.Path + " " + semanticResult.Intent)

	for _, keyword := range execQuery.Keywords {
		if strings.Contains(queryText, strings.ToLower(keyword)) {
			confidence += 0.2
		}
	}

	for _, keyword := range execQuery.SemanticKeywords {
		if strings.Contains(queryText, strings.ToLower(keyword)) {
			confidence += 0.3
		}
	}

	// Check for related query patterns
	for _, relatedPattern := range execQuery.RelatedQueries {
		if strings.Contains(queryText, strings.ToLower(relatedPattern)) {
			confidence += 0.4
		}
	}

	// Boost confidence based on semantic similarity score
	confidence += semanticResult.SimilarityScore * 0.5

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// generateMappingReason creates a human-readable explanation for the mapping
func generateMappingReason(execQuery ExecutableQuery, bestMatch *QuerySearchResult, matchCount int) string {
	if bestMatch == nil {
		return "No specific matches found"
	}

	reason := fmt.Sprintf("Mapped based on %d related queries", matchCount)

	if bestMatch.SimilarityScore > 0.7 {
		reason += " with high semantic similarity"
	} else if bestMatch.SimilarityScore > 0.4 {
		reason += " with moderate semantic similarity"
	}

	if strings.Contains(strings.ToLower(bestMatch.Intent), strings.ToLower(execQuery.Name)) {
		reason += " and direct intent match"
	}

	return reason
}
