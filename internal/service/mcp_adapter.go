package service

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This file adapts the service's argument-only handler style to the official
// MCP Go SDK (github.com/modelcontextprotocol/go-sdk), and centralizes the
// behavior annotations advertised for each tool.

// newTextContent builds a text content block for tool and prompt results.
func newTextContent(text string) mcp.Content {
	return &mcp.TextContent{Text: text}
}

// newToolResponse builds a tool result from one or more content blocks.
func newToolResponse(content ...mcp.Content) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: content}
}

// addTool registers a typed, argument-only handler with the go-sdk server.
// The input schema is inferred from the In struct (json + jsonschema tags),
// inputs are validated by the SDK before the handler runs, and handler errors
// are returned as tool execution errors (isError) per the MCP spec.
func addTool[In any](server *mcp.Server, name, description string, h func(In) (*mcp.CallToolResult, error)) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: toolAnnotations[name],
	}, func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		res, err := h(in)
		return res, nil, err
	})
}

// contentText returns the text of a content block, or "" if it is not text.
func contentText(c mcp.Content) string {
	if tc, ok := c.(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// promptResult builds a single-message prompt result attributed to the assistant.
func promptResult(description string, content mcp.Content) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Description: description,
		Messages:    []*mcp.PromptMessage{{Role: "assistant", Content: content}},
	}
}

func boolPtr(b bool) *bool { return &b }

// Annotation classes shared by the tool table below. All hints are advisory
// (clients use them for confirmation UX and display, never for enforcement).
var (
	// annReadOnly marks tools that do not modify any environment.
	annReadOnly = &mcp.ToolAnnotations{ReadOnlyHint: true}
	// annCreate marks tools that only add new data (safe, not idempotent).
	annCreate = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false)}
	// annRebuild marks maintenance tools that rebuild derived state (safe to repeat).
	annRebuild = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true}
	// annUpdate marks tools that overwrite existing data.
	annUpdate = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true}
	// annDelete marks tools that permanently remove data.
	annDelete = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true}
)

// toolAnnotations classifies every registered tool. Tools absent from this
// table are advertised without annotations.
var toolAnnotations = map[string]*mcp.ToolAnnotations{
	// Read-only discovery, query, and analysis tools.
	"list_networks":            annReadOnly,
	"search_paths":             annReadOnly,
	"search_paths_bulk":        annReadOnly,
	"analyze_network_prefixes": annReadOnly,
	"run_nqe_query_by_id":      annReadOnly,
	"list_nqe_queries":         annReadOnly,
	"get_device_basic_info":    annReadOnly,
	"get_device_hardware":      annReadOnly,
	"get_hardware_support":     annReadOnly,
	"get_os_support":           annReadOnly,
	"search_configs":           annReadOnly,
	"get_config_diff":          annReadOnly,
	"list_devices":             annReadOnly,
	"get_device_locations":     annReadOnly,
	"list_snapshots":           annReadOnly,
	"get_latest_snapshot":      annReadOnly,
	"list_locations":           annReadOnly,
	"get_default_settings":     annReadOnly,
	"get_cache_stats":          annReadOnly,
	"suggest_similar_queries":  annReadOnly,
	"search_nqe_queries":       annReadOnly,
	"get_database_status":      annReadOnly,
	"search_entities":          annReadOnly,
	"get_entity":               annReadOnly,
	"get_relations":            annReadOnly,
	"get_observations":         annReadOnly,
	"get_memory_stats":         annReadOnly,
	"get_query_analytics":      annReadOnly,
	"list_instance_ids":        annReadOnly,
	"get_nqe_result_chunks":    annReadOnly,
	"get_nqe_result_summary":   annReadOnly,
	"analyze_nqe_result_sql":   annReadOnly,
	"search_bloom_filter":      annReadOnly,
	"get_bloom_filter_stats":      annReadOnly,
	"generate_drawio_topology":    annReadOnly,

	// Additive creation tools.
	"create_network":  annCreate,
	"create_location": annCreate,
	"create_entity":   annCreate,
	"create_relation": annCreate,
	"add_observation": annCreate,

	// Maintenance tools that (re)build derived state.
	"initialize_query_index": annRebuild,
	"hydrate_database":       annRebuild,
	"refresh_query_index":    annRebuild,
	"build_bloom_filter":     annRebuild,
	"set_default_network":    annRebuild,

	// Tools that overwrite existing data.
	"update_network":          annUpdate,
	"update_location":         annUpdate,
	"update_device_locations": annUpdate,
	"create_locations_bulk":   annUpdate, // upsert: may overwrite existing locations

	// Tools that permanently remove data.
	"delete_snapshot":    annDelete,
	"delete_location":    annDelete,
	"delete_entity":      annDelete,
	"delete_relation":    annDelete,
	"delete_observation": annDelete,
	"clear_cache":        annDelete,
}
