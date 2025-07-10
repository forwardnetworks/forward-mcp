package service

import (
	"encoding/json"
	"os"
	"strings"
)

// JSONFormatMode determines how JSON should be formatted
type JSONFormatMode int

const (
	// JSONCompact produces minimal JSON without whitespace (best for LLMs)
	JSONCompact JSONFormatMode = iota
	// JSONFormatted produces human-readable JSON with indentation
	JSONFormatted
	// JSONAuto chooses format based on context (compact for production, formatted for debug)
	JSONAuto
)

// MarshalJSON formats JSON according to the specified mode
func MarshalJSON(v interface{}, mode JSONFormatMode) ([]byte, error) {
	switch mode {
	case JSONCompact:
		return json.Marshal(v)
	case JSONFormatted:
		return json.MarshalIndent(v, "", "  ")
	case JSONAuto:
		// Use compact format in production, formatted in debug mode
		if isDebugMode() {
			return json.MarshalIndent(v, "", "  ")
		}
		return json.Marshal(v)
	default:
		return json.Marshal(v)
	}
}

// MarshalJSONString is a convenience function that returns a string
func MarshalJSONString(v interface{}, mode JSONFormatMode) string {
	data, err := MarshalJSON(v, mode)
	if err != nil {
		return "{\"error\":\"failed to marshal JSON\"}"
	}
	return string(data)
}

// MarshalCompactJSON produces minimal JSON for token efficiency
func MarshalCompactJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// MarshalCompactJSONString produces minimal JSON string for token efficiency
func MarshalCompactJSONString(v interface{}) string {
	return MarshalJSONString(v, JSONCompact)
}

// isDebugMode checks if we're in debug mode for formatting decisions
func isDebugMode() bool {
	debug := os.Getenv("DEBUG")
	if debug == "" {
		debug = os.Getenv("FORWARD_MCP_DEBUG")
	}

	switch strings.ToLower(debug) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// OptimizeJSONForLLM removes unnecessary whitespace and optimizes JSON structure for LLM consumption
func OptimizeJSONForLLM(data interface{}) (string, error) {
	// First marshal to compact JSON
	compactBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// For very large responses, we could potentially implement additional optimizations:
	// - Remove null fields
	// - Abbreviate field names
	// - Use shorter representations
	// But for now, compact JSON is a good start

	return string(compactBytes), nil
}

// EstimateTokenSavings calculates approximate token savings from using compact JSON
func EstimateTokenSavings(v interface{}) (compactTokens, formattedTokens, savingsPercent int) {
	compact, _ := json.Marshal(v)
	formatted, _ := json.MarshalIndent(v, "", "  ")

	// Rough token estimation (GPT-style: ~4 chars per token)
	compactTokens = len(compact) / 4
	formattedTokens = len(formatted) / 4

	if formattedTokens > 0 {
		savingsPercent = ((formattedTokens - compactTokens) * 100) / formattedTokens
	}

	return compactTokens, formattedTokens, savingsPercent
}
