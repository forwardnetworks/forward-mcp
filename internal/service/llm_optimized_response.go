package service

import (
	"fmt"
	"strings"
)

// LLMOptimizedQueryResult represents a query result optimized for LLM consumption
type LLMOptimizedQueryResult struct {
	// Core query identification
	QueryID     string  `json:"query_id"`
	QueryPath   string  `json:"query_path"`
	QueryIntent string  `json:"query_intent"`
	Confidence  float64 `json:"confidence_score"` // 0.0 to 1.0

	// Categorization for LLM understanding
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory,omitempty"`
	Keywords    []string `json:"keywords"` // Extracted key terms
	Domain      string   `json:"domain"`   // e.g., "network", "security", "cloud"

	// Execution readiness
	ExecutionReady bool                   `json:"execution_ready"`
	RequiredParams []string               `json:"required_parameters,omitempty"`
	OptionalParams []string               `json:"optional_parameters,omitempty"`
	DefaultValues  map[string]interface{} `json:"default_values,omitempty"`

	// LLM guidance
	WhatItDoes     string   `json:"what_it_does"`         // Clear explanation
	WhenToUse      string   `json:"when_to_use"`          // Use cases
	Prerequisites  []string `json:"prerequisites"`        // What's needed first
	NextSteps      []string `json:"suggested_next_steps"` // What to do after
	RelatedQueries []string `json:"related_queries"`      // Similar query IDs

	// Technical details (optional for advanced users)
	CodePreview string `json:"code_preview,omitempty"`
	Complexity  string `json:"complexity"` // "simple", "intermediate", "advanced"
}

// LLMOptimizedSearchResponse represents the complete search response for LLMs
type LLMOptimizedSearchResponse struct {
	// Search context
	SearchQuery  string `json:"search_query"`
	SearchMethod string `json:"search_method"` // "keyword", "semantic", "hybrid"
	ResultCount  int    `json:"result_count"`
	TotalQueries int    `json:"total_available_queries"`
	SearchTimeMs int    `json:"search_time_ms"`

	// Results
	Queries []LLMOptimizedQueryResult `json:"queries"`

	// LLM guidance
	Interpretation string   `json:"search_interpretation"`  // What the search understood
	Suggestions    []string `json:"refinement_suggestions"` // How to improve search
	WorkflowAdvice string   `json:"workflow_advice"`        // Overall guidance

	// Context for decision making
	UserIntent        string            `json:"inferred_user_intent"`
	RecommendedAction string            `json:"recommended_action"`
	ContextualHelp    map[string]string `json:"contextual_help"`
}

// FormatForLLM converts internal search results to LLM-optimized format
func (idx *NQEQueryIndex) FormatForLLM(searchQuery string, results []*QuerySearchResult, searchTimeMs int) *LLMOptimizedSearchResponse {
	optimizedResults := make([]LLMOptimizedQueryResult, 0, len(results))

	for _, result := range results {
		optimized := LLMOptimizedQueryResult{
			QueryID:     result.QueryID,
			QueryPath:   result.Path,
			QueryIntent: result.Intent,
			Confidence:  result.SimilarityScore,
			Category:    result.Category,
			Subcategory: result.Subcategory,
			Keywords:    extractKeywords(result),
			Domain:      inferDomain(result.Category),

			// Execution analysis
			ExecutionReady: len(result.QueryID) > 0,
			RequiredParams: analyzeRequiredParams(result),
			OptionalParams: analyzeOptionalParams(result),
			DefaultValues:  getDefaultValues(result),

			// LLM guidance
			WhatItDoes:     generateWhatItDoes(result),
			WhenToUse:      generateWhenToUse(result),
			Prerequisites:  generatePrerequisites(result),
			NextSteps:      generateNextSteps(result),
			RelatedQueries: findRelatedQueries(idx, result),

			// Technical details
			CodePreview: truncateCode(result.Code, 200),
			Complexity:  assessComplexity(result),
		}

		optimizedResults = append(optimizedResults, optimized)
	}

	return &LLMOptimizedSearchResponse{
		SearchQuery:  searchQuery,
		SearchMethod: inferSearchMethod(results),
		ResultCount:  len(results),
		TotalQueries: len(idx.queries),
		SearchTimeMs: searchTimeMs,
		Queries:      optimizedResults,

		// High-level LLM guidance
		Interpretation:    interpretSearchIntent(searchQuery),
		Suggestions:       generateRefinementSuggestions(searchQuery, results),
		WorkflowAdvice:    generateWorkflowAdvice(searchQuery, results),
		UserIntent:        inferUserIntent(searchQuery),
		RecommendedAction: recommendAction(results),
		ContextualHelp:    generateContextualHelp(searchQuery, results),
	}
}

// Helper functions for LLM optimization
func extractKeywords(result *QuerySearchResult) []string {
	// Extract meaningful keywords from path and intent
	keywords := []string{}

	// Add category-based keywords
	if result.Category != "" {
		keywords = append(keywords, result.Category)
	}

	// Add domain-specific terms based on analysis
	if containsAny(result.Path, []string{"BGP", "bgp"}) {
		keywords = append(keywords, "BGP", "routing", "protocol")
	}
	if containsAny(result.Path, []string{"Security", "security", "ACL", "firewall"}) {
		keywords = append(keywords, "security", "access-control", "filtering")
	}
	if containsAny(result.Path, []string{"AWS", "Cloud", "cloud"}) {
		keywords = append(keywords, "cloud", "AWS", "infrastructure")
	}

	return keywords
}

func inferDomain(category string) string {
	domains := map[string]string{
		"L3":         "network-layer3",
		"L2":         "network-layer2",
		"Security":   "security",
		"Cloud":      "cloud-infrastructure",
		"Devices":    "device-management",
		"Interfaces": "network-interfaces",
		"External":   "external-systems",
	}

	if domain, exists := domains[category]; exists {
		return domain
	}
	return "general"
}

func generateWhatItDoes(result *QuerySearchResult) string {
	if result.Intent != "" {
		return result.Intent
	}

	// Generate from path analysis
	pathLower := strings.ToLower(result.Path)
	if strings.Contains(pathLower, "inventory") {
		return "Provides comprehensive device inventory and hardware information"
	}
	if strings.Contains(pathLower, "security") {
		return "Analyzes network security configurations and potential vulnerabilities"
	}
	if strings.Contains(pathLower, "bgp") {
		return "Examines BGP routing protocol configuration and neighbor relationships"
	}

	return "Performs network analysis based on the specified query parameters"
}

func generateWhenToUse(result *QuerySearchResult) string {
	category := strings.ToLower(result.Category)
	pathLower := strings.ToLower(result.Path)

	if strings.Contains(pathLower, "inventory") {
		return "When you need to audit device hardware, check device counts, or plan capacity"
	}
	if strings.Contains(pathLower, "security") {
		return "During security assessments, compliance audits, or vulnerability analysis"
	}
	if strings.Contains(pathLower, "bgp") {
		return "For troubleshooting routing issues, verifying BGP neighbor states, or route analysis"
	}
	if category == "cloud" {
		return "When analyzing cloud infrastructure, AWS configurations, or hybrid connectivity"
	}

	return "Use this query when you need detailed analysis of the specified network components"
}

func assessComplexity(result *QuerySearchResult) string {
	codeLength := len(result.Code)
	paramCount := analyzeParamCount(result)

	if codeLength < 100 && paramCount <= 1 {
		return "simple"
	} else if codeLength < 500 && paramCount <= 3 {
		return "intermediate"
	}
	return "advanced"
}

// Format as clean JSON for LLM consumption (compact for token efficiency)
func (response *LLMOptimizedSearchResponse) ToJSON() (string, error) {
	// Use compact JSON to minimize token usage
	return OptimizeJSONForLLM(response)
}

// Create a concise summary for quick LLM understanding
func (response *LLMOptimizedSearchResponse) ToSummary() string {
	if len(response.Queries) == 0 {
		return fmt.Sprintf("No queries found for '%s'. Try broader terms or check available categories.", response.SearchQuery)
	}

	summary := fmt.Sprintf("Found %d relevant queries for '%s':\n\n", response.ResultCount, response.SearchQuery)

	for i, query := range response.Queries {
		if i >= 3 { // Limit to top 3 for conciseness
			break
		}

		summary += fmt.Sprintf("%d. %s (%.0f%% match)\n", i+1, query.QueryPath, query.Confidence*100)
		summary += fmt.Sprintf("   Purpose: %s\n", query.WhatItDoes)
		summary += fmt.Sprintf("   Execute: run_nqe_query_by_id(query_id='%s')\n\n", query.QueryID)
	}

	summary += fmt.Sprintf("Recommended: %s", response.RecommendedAction)
	return summary
}

// Helper functions
func containsAny(text string, terms []string) bool {
	textLower := strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(textLower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// Additional helper functions would go here...
func analyzeRequiredParams(result *QuerySearchResult) []string { return []string{} }
func analyzeOptionalParams(result *QuerySearchResult) []string { return []string{} }
func getDefaultValues(result *QuerySearchResult) map[string]interface{} {
	return make(map[string]interface{})
}
func generatePrerequisites(result *QuerySearchResult) []string { return []string{} }
func generateNextSteps(result *QuerySearchResult) []string {
	return []string{"Execute query", "Analyze results"}
}
func findRelatedQueries(idx *NQEQueryIndex, result *QuerySearchResult) []string { return []string{} }
func truncateCode(code string, maxLen int) string {
	if len(code) <= maxLen {
		return code
	}
	return code[:maxLen] + "..."
}
func inferSearchMethod(results []*QuerySearchResult) string {
	if len(results) > 0 {
		return results[0].MatchType
	}
	return "keyword"
}
func interpretSearchIntent(query string) string {
	return fmt.Sprintf("User wants to analyze: %s", query)
}
func generateRefinementSuggestions(query string, results []*QuerySearchResult) []string {
	return []string{}
}
func generateWorkflowAdvice(query string, results []*QuerySearchResult) string {
	return "Select a query and execute it"
}
func inferUserIntent(query string) string { return "network analysis" }
func recommendAction(results []*QuerySearchResult) string {
	if len(results) > 0 {
		return fmt.Sprintf("Execute the top result: %s", results[0].QueryID)
	}
	return "Refine search terms"
}
func generateContextualHelp(query string, results []*QuerySearchResult) map[string]string {
	return map[string]string{
		"next_step":     "Use run_nqe_query_by_id with the query_id",
		"documentation": "Each query includes purpose and usage guidance",
	}
}
func analyzeParamCount(result *QuerySearchResult) int { return 1 }
