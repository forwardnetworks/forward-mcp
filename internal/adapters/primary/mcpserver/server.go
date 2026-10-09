// Package mcpserver is the primary adapter: it serves the use cases as MCP tools,
// prompts and a resource over the official MCP Go SDK.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/forward-mcp/internal/ports"
	"github.com/forward-mcp/internal/usecases"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This file adapts the use cases' handler style to the official
// MCP Go SDK (github.com/modelcontextprotocol/go-sdk), and centralizes the
// behavior annotations advertised for each tool.

// newTextContent builds a text content block for tool and prompt results.
func newTextContent(text string) mcp.Content {
	return &mcp.TextContent{Text: text}
}

// toCallToolResult renders a Result as an MCP tool result.
func toCallToolResult(r *usecases.Result) *mcp.CallToolResult {
	if r == nil {
		return nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{newTextContent(r.Text)}}
}

// addTool registers a typed handler with the go-sdk server. The input schema is
// inferred from the In struct (json + jsonschema tags), inputs are validated by
// the SDK before the handler runs, and handler errors are returned as tool
// execution errors (isError) per the MCP spec. The handler receives the request
// context, so a client that cancels the call stops the work behind it.
func addTool[In any](server *mcp.Server, name, description string, h func(context.Context, In) (*usecases.Result, error)) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: toolAnnotations[name],
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		res, err := h(ctx, in)
		return toCallToolResult(res), nil, err
	})
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
	"get_bloom_filter_stats":   annReadOnly,

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

// Register serves every tool, prompt and resource of svc on server.
func Register(server *mcp.Server, svc *usecases.Service, log ports.Logger) error {
	if err := registerTools(server, svc); err != nil {
		return fmt.Errorf("register tools: %w", err)
	}
	if err := registerPrompts(server, svc, log); err != nil {
		return fmt.Errorf("register prompts: %w", err)
	}
	if err := registerResources(server, svc, log); err != nil {
		return fmt.Errorf("register resources: %w", err)
	}
	return nil
}

// registerTools registers all Forward Networks tools with the MCP server
func registerTools(server *mcp.Server, s *usecases.Service) error {
	// Network Management Tools
	addTool(server, "list_networks",
		"List all networks in the Forward platform. Returns network IDs, names, and descriptions. Use this to discover available networks or find network IDs for other operations. Supports pagination (limit/offset) and memory storage for large datasets.",
		s.ListNetworks)

	addTool(server, "create_network",
		"Create a new network in the Forward platform. Requires a network name. Returns the new network with ID for subsequent operations.",
		s.CreateNetwork)

	// addTool(server, "delete_network",
	// 	"Delete a network from the Forward platform. Requires network_id. WARNING: This permanently deletes all associated data.",
	// 	s.deleteNetwork)

	addTool(server, "update_network",
		"Update network properties in the Forward platform. Requires network_id and at least one property to update (name or description).",
		s.UpdateNetwork)

	// Path Search Tools
	addTool(server, "search_paths",
		"Tool to trace L3/L4 packet paths from source to destination through network devices and links. Use when troubleshooting connectivity, verifying traffic flow, or analyzing routing decisions. Requires dst_ip (IP or CIDR); from (device name) or src_ip optional. For multiple queries use search_paths_bulk.",
		s.SearchPathsEntry)

	addTool(server, "search_paths_bulk",
		"Tool to trace multiple L3/L4 packet paths in a single request for better performance. Use when analyzing multiple source-destination pairs or bulk connectivity validation. Requires array of queries with dst_ip; from or src_ip optional per query. Returns results for all paths in one response.",
		s.SearchPathsBulkEntry)

	// Register network prefix analysis tool
	addTool(server, "analyze_network_prefixes",
		"Tool to discover network prefixes at different aggregation levels and analyze site-to-site connectivity. Use when validating network segmentation, verifying route aggregation, or planning multi-site connectivity. Specify prefix_levels (e.g., /8, /16, /24) and optional device filters. Returns prefix mappings and connectivity matrices.",
		s.AnalyzeNetworkPrefixes)

	// NQE Tools
	addTool(server, "run_nqe_query_by_id",
		"Tool to execute a Network Query Engine (NQE) query using a predefined query ID from the Forward library. Use when running standard network analysis, compliance checks, or inventory reports. Requires query_id; network_id optional if default set. Set all_results=true to fetch complete datasets with automatic pagination. Large results are cached and chunked automatically.",
		s.RunNQEQueryByID)

	addTool(server, "list_nqe_queries",
		"🔍 **DISCOVERY TOOL**: Find available NQE queries for your analysis needs.\n\nList available NQE queries from the Forward Networks query library. Use this to discover predefined queries for reports and analysis.\n\n**Usage Tips:**\n- Filter by directory (e.g., '/L3/Basic/', '/L3/Advanced/', '/L3/Security/')\n- Use search_nqe_queries for semantic search\n- Check query descriptions before running\n- Use query IDs with run_nqe_query_by_id",
		s.ListNQEQueries)

	// First-Class Query Tools - Most Important Network Operations
	addTool(server, "get_device_basic_info",
		"Tool to retrieve device inventory with names, types, platforms, OS versions, and management IPs. Use when discovering network devices, building device lists, or starting network analysis. Supports filters and pagination. Combine with get_device_hardware for hardware details and serial numbers.",
		s.GetDeviceBasicInfo)

	addTool(server, "get_device_hardware",
		"🔧 **HARDWARE INVENTORY**: Get detailed hardware information for lifecycle management.\n\nGet device hardware information including models, serial numbers, and hardware details. Critical for hardware inventory and lifecycle management.\n\n**What you get:**\n- Device models and serial numbers\n- Hardware specifications\n- Vendor and platform details\n- Interface hardware information\n- Asset tracking data\n\n**Use Cases:**\n- Hardware refresh planning\n- Asset inventory management\n- Support contract validation\n- Capacity planning",
		s.GetDeviceHardware)

	addTool(server, "get_hardware_support",
		"⚠️ **COMPLIANCE CRITICAL**: Check hardware support status for security and compliance.\n\nGet hardware support status including end-of-life and support dates. Essential for compliance and planning hardware refreshes.\n\n**What you get:**\n- End-of-life dates\n- Support contract status\n- Security vulnerability information\n- Recommended upgrade paths\n- Compliance status\n\n**Critical Use Cases:**\n- Security compliance audits\n- Hardware refresh planning\n- Risk assessment\n- Budget planning for upgrades",
		s.GetHardwareSupport)

	addTool(server, "get_os_support",
		"🔒 **SECURITY ESSENTIAL**: Check OS support status for security compliance.\n\nGet operating system support status including OS versions and support dates. Critical for security compliance and OS upgrade planning.\n\n**What you get:**\n- OS version information\n- Support end dates\n- Security patch status\n- Upgrade recommendations\n- Compliance status\n\n**Security Use Cases:**\n- Security compliance audits\n- Vulnerability assessment\n- Patch management planning\n- OS upgrade planning",
		s.GetOSSupport)

	addTool(server, "search_configs",
		"🔍 **CONFIGURATION SEARCH**: Search device configurations for specific patterns and settings.\n\nSearch device configurations for specific patterns, commands, or settings. Use this to find specific configurations across your network.\n\n**Pattern Examples:**\n```\ninterface\n  zone-member security\n  ip address {ip:string}\n```\n\n**Best Practices:**\n- Use hierarchical patterns with indentation\n- Extract variables with {name:type} syntax\n- Filter by device names for targeted searches\n- Use specific patterns for better results\n\n**Common Use Cases:**\n- Find specific interface configurations\n- Locate security policies\n- Identify routing configurations\n- Audit configuration compliance",
		s.SearchConfigs)

	addTool(server, "get_config_diff",
		"Compare network configurations between snapshots to identify changes. Essential for change tracking and troubleshooting configuration drift.",
		s.GetConfigDiff)

	// Device Management Tools
	addTool(server, "list_devices",
		"List devices in a network. Requires network_id. Returns basic device inventory with names, types, and status. Supports pagination with limit and offset. Use for device discovery and inventory management.",
		s.ListDevices)

	addTool(server, "get_device_locations",
		"Get device location mappings for a network. Requires network_id. Shows which devices are assigned to which physical locations. Use for topology planning and device organization.",
		s.GetDeviceLocations)

	// Snapshot Management Tools
	addTool(server, "list_snapshots",
		"List network configuration snapshots. Requires network_id. Shows historical network states with timestamps and status. Use to view configuration history and find specific snapshots for queries. Supports pagination (limit/offset) and memory storage for large datasets.",
		s.ListSnapshots)

	addTool(server, "get_latest_snapshot",
		"Get the latest processed snapshot for a network. Requires network_id. Returns the most recent network state. Use to ensure queries run against current configuration.",
		s.GetLatestSnapshot)

	addTool(server, "delete_snapshot",
		"Delete a network snapshot. Requires snapshot_id. WARNING: This permanently removes the snapshot and associated historical data. Use with caution for cleanup of old snapshots.",
		s.DeleteSnapshot)

	// Location Management Tools
	addTool(server, "list_locations",
		"List locations in a network. Requires network_id. Returns physical locations with names and coordinates. Use to view network topology and organize devices by location. Supports pagination (limit/offset) and memory storage for large datasets. Default limit is 25 to prevent token overflow.",
		s.ListLocations)

	addTool(server, "create_location",
		"Create a new location in a network. Requires network_id, location name, latitude, and longitude. Optional city, adminDivision, and country. Use to set up new sites or data centers for device organization.",
		s.CreateLocation)

	addTool(server, "update_location",
		"Update an existing location in a network. Requires network_id and location_id. Optional new name, description, latitude, and longitude. Use to modify location details.",
		s.UpdateLocation)

	addTool(server, "delete_location",
		"Delete a location from a network. Requires network_id and location_id. Use to remove locations that are no longer needed.",
		s.DeleteLocation)

	addTool(server, "create_locations_bulk",
		"Create or update multiple network locations in a single operation. Requires network_id and an array of locations. Uses PATCH /api/networks/{networkId}/locations. Locations with existing IDs will be updated, others will be created.",
		s.CreateLocationsBulk)

	addTool(server, "update_device_locations",
		"Update device location assignments in bulk. Requires network_id and a map of device IDs to location IDs. Use to assign multiple devices to their physical locations efficiently. Note: Cloud devices (CSR1KV, PAN-FW, etc.) cannot be moved to physical locations.",
		s.UpdateDeviceLocations)

	// Default Settings Management Tools
	addTool(server, "get_default_settings",
		"View current default settings for network operations. Shows the default network ID, snapshot ID, and query limits configured for this session.",
		s.GetDefaultSettings)

	addTool(server, "set_default_network",
		"Set the default network for all operations. Accepts either a network ID or network name. This will be used when network_id is not specified in other tools.",
		s.SetDefaultNetwork)

	// Semantic Cache and AI Enhancement Tools
	addTool(server, "get_cache_stats",
		"View semantic cache performance statistics including hit rates, total queries, and cache efficiency metrics.",
		s.GetCacheStats)

	addTool(server, "suggest_similar_queries",
		"Get suggestions for similar NQE queries based on semantic similarity to your query intent. Helps discover relevant existing queries.",
		s.SuggestSimilarQueries)

	addTool(server, "clear_cache",
		"Clear expired entries from the semantic cache to free up memory and improve performance.",
		s.ClearCache)

	// AI-Powered Query Discovery Tools
	addTool(server, "search_nqe_queries",
		"Tool to find relevant NQE queries from 6000+ predefined queries using natural language semantic search. Use when discovering queries for specific network analysis tasks without knowing exact query names. Describe what you want to analyze; returns ranked query suggestions with similarity scores. Be specific for best results.",
		s.SearchNQEQueries)

	// Bulk location setup workflow (guides bulk upsert using PATCH)
	server.AddPrompt(&mcp.Prompt{
		Name:        "bulk_location_setup",
		Description: "Guide to bulk create or update network locations",
		Arguments:   []*mcp.PromptArgument{{Name: "session_id", Description: "Session ID for tracking workflow state"}},
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		content := "" +
			"You can create or update multiple locations in bulk using PATCH.\n\n" +
			"- Use create_locations_bulk for both creating new locations and updating existing ones.\n" +
			"- Locations with existing IDs will be updated, others will be created.\n" +
			"- Either 'id' (for updates) or 'name' (for creates) must be provided for each location.\n\n" +
			"Example request for create_locations_bulk:\n" +
			"{" +
			"\"network_id\": \"12345\",\n" +
			"\"locations\": [\n" +
			"  { \"id\": \"dyt-a\", \"name\": \"Dayton DC Updated\", \"lat\": 39.8113, \"lng\": -84.2722 },\n" +
			"  { \"name\": \"NY Edge\", \"lat\": 40.7128, \"lng\": -74.0060, \"city\": \"New York\" }\n" +
			"]}\n\n" +
			"Tip: This is safe to run multiple times - existing locations will be updated, new ones created."
		return promptResult("Bulk Location Setup", newTextContent(content)), nil
	})

	addTool(server, "initialize_query_index",
		"Initialize or rebuild the AI-powered NQE query index from the spec file. REQUIRED before using search_nqe_queries. Run this once at startup or when you get 'query index is empty' errors. Can generate embeddings for semantic search if OpenAI API key is available.",
		s.InitializeQueryIndex)

	// Database Hydration Tools
	addTool(server, "hydrate_database",
		"Hydrate the NQE database by loading queries from the Forward Networks API. Use this to refresh the database with latest query metadata and ensure optimal performance for search operations. Automatically refreshes the query index and optionally regenerates AI embeddings.",
		s.HydrateDatabase)

	addTool(server, "refresh_query_index",
		"Refresh the query index from the current database content. Use this after hydrating the database to ensure the search index reflects the latest data.",
		s.RefreshQueryIndex)

	addTool(server, "get_database_status",
		"Get the current status of the database and query index including query counts, last update times, and performance metrics.",
		s.GetDatabaseStatus)

	// Memory Management Tools
	addTool(server, "create_entity",
		"Create a new entity in the knowledge graph memory system. Entities represent people, networks, devices, projects, or any other important concept to remember.",
		s.CreateEntity)

	addTool(server, "create_relation",
		"Create a relation between two entities in the knowledge graph. Relations represent how entities are connected (e.g., 'owns', 'manages', 'depends_on').",
		s.CreateRelation)

	addTool(server, "add_observation",
		"Add an observation to an entity. Observations are additional facts, notes, preferences, or behaviors associated with an entity.",
		s.AddObservation)

	addTool(server, "search_entities",
		"Search for entities in the knowledge graph by name, type, or observation content. Use this to find information you've stored about people, networks, or concepts.",
		s.SearchEntities)

	addTool(server, "get_entity",
		"Retrieve a specific entity by ID or name. Use this to get detailed information about a specific person, network, device, or concept.",
		s.GetEntity)

	addTool(server, "get_relations",
		"Get all relations for a specific entity. Use this to understand how an entity is connected to others in the knowledge graph.",
		s.GetRelations)

	addTool(server, "get_observations",
		"Get all observations for a specific entity. Use this to retrieve all stored facts, notes, and preferences about an entity.",
		s.GetObservations)

	addTool(server, "delete_entity",
		"Delete an entity and all its relations and observations. Use with caution as this permanently removes all stored information about the entity.",
		s.DeleteEntity)

	addTool(server, "delete_relation",
		"Delete a specific relation between entities. Use this to remove connections that are no longer relevant.",
		s.DeleteRelation)

	addTool(server, "delete_observation",
		"Delete a specific observation from an entity. Use this to remove outdated or incorrect information.",
		s.DeleteObservation)

	addTool(server, "get_memory_stats",
		"Get statistics about the memory system including counts of entities, relations, and observations by type.",
		s.GetMemoryStats)

	// API Analytics Tools
	addTool(server, "get_query_analytics",
		"Get analytics about query patterns and performance for a specific network. Shows query counts, execution times, result patterns, and usage trends from the memory system.",
		s.GetQueryAnalytics)

	// Instance Management Tools
	addTool(server, "list_instance_ids",
		"List all available Forward Networks instance IDs in the database. Shows instance IDs with query counts and sync dates. Use this to find the correct instance ID to configure in FORWARD_INSTANCE_ID environment variable.",
		s.ListInstanceIDs)

	// Tool handler for get_nqe_result_chunks
	addTool(server, "get_nqe_result_chunks",
		"Retrieve chunked NQE query results from the memory system. Provide either entity_id or (query_id, network_id, snapshot_id). Optionally, specify chunk_index to fetch a single chunk.",
		s.GetNQEResultChunks)

	// Add get_nqe_result_summary tool handler
	addTool(server, "get_nqe_result_summary",
		"Get a summary of a stored NQE result (row count, columns, preview rows) by entity_id or (query_id, network_id, snapshot_id).",
		s.GetNQEResultSummary)

	// Add analyze_nqe_result_sql tool handler
	addTool(server, "analyze_nqe_result_sql",
		"Run a SQL query on a stored NQE result (by entity_id). Example: SELECT COUNT(*) FROM nqe_result;",
		s.AnalyzeNQEResultSQL)

	// Add bloom search tool handlers
	addTool(server, "build_bloom_filter",
		"Build a bloom filter from NQE query results for efficient large dataset searching",
		s.BuildBloomFilter)

	addTool(server, "search_bloom_filter",
		"Search a bloom filter for matching items with sub-millisecond performance",
		s.SearchBloomFilter)

	addTool(server, "get_bloom_filter_stats",
		"Get statistics and performance metrics for all bloom filters",
		s.GetBloomFilterStats)

	return nil
}

// addWorkflowPrompt registers an interactive workflow prompt whose single
// "session_id" argument is forwarded to the underlying workflow handler.
func addWorkflowPrompt(server *mcp.Server, name, description, title, fallback string, run func(ctx context.Context, sessionID string) (*usecases.Result, error)) {
	server.AddPrompt(&mcp.Prompt{
		Name:        name,
		Description: description,
		Arguments:   []*mcp.PromptArgument{{Name: "session_id", Description: "Session ID for tracking workflow state"}},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		response, err := run(ctx, req.Params.Arguments["session_id"])
		if err != nil {
			return nil, err
		}
		if response != nil {
			return promptResult(title, newTextContent(response.Text)), nil
		}
		return promptResult(title, newTextContent(fallback)), nil
	})
}

// registerPrompts registers workflow prompts with the MCP server
func registerPrompts(server *mcp.Server, s *usecases.Service, log ports.Logger) error {
	addWorkflowPrompt(server, "nqe_discovery",
		"Interactive NQE query discovery workflow to help find and run network queries",
		"NQE Query Discovery", "Welcome to NQE Query Discovery!",
		func(ctx context.Context, sessionID string) (*usecases.Result, error) {
			return s.NqeQueryDiscoveryWorkflow(ctx, usecases.NQEDiscoveryArgs{SessionID: sessionID})
		})

	addWorkflowPrompt(server, "network_discovery",
		"Interactive network discovery workflow to explore available networks and devices",
		"Network Discovery", "Network discovery workflow",
		func(ctx context.Context, sessionID string) (*usecases.Result, error) {
			return s.NetworkDiscoveryWorkflow(ctx, usecases.NetworkDiscoveryArgs{SessionID: sessionID})
		})

	addWorkflowPrompt(server, "large_nqe_results_workflow",
		"Interactive workflow for handling large NQE query results with memory system storage and SQL analysis",
		"Large NQE Results Workflow", "Welcome to Large NQE Results Workflow!",
		func(ctx context.Context, sessionID string) (*usecases.Result, error) {
			return s.LargeNQEResultsWorkflow(usecases.LargeNQEResultsWorkflowArgs{SessionID: sessionID})
		})

	addWorkflowPrompt(server, "path_search_workflow",
		"Interactive workflow for effective path search using best practices including 'from' property and bulk operations",
		"Path Search Workflow", "Welcome to Path Search Workflow!",
		func(ctx context.Context, sessionID string) (*usecases.Result, error) {
			return s.PathSearchWorkflow(usecases.PathSearchWorkflowArgs{SessionID: sessionID})
		})

	// The prefix discovery workflow also accepts a "step" argument.
	server.AddPrompt(&mcp.Prompt{
		Name:        "network_prefix_discovery_workflow",
		Description: "Interactive workflow for discovering network prefixes, mapping them to devices, and analyzing connectivity between sites using different aggregation levels",
		Arguments: []*mcp.PromptArgument{
			{Name: "session_id", Description: "Session ID for tracking workflow state"},
			{Name: "step", Description: "Current step in the workflow"},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		response, err := s.NetworkPrefixDiscoveryWorkflow(usecases.NetworkPrefixDiscoveryArgs{
			SessionID: req.Params.Arguments["session_id"],
			Step:      req.Params.Arguments["step"],
		})
		if err != nil {
			return nil, err
		}
		if response != nil {
			return promptResult("Network Prefix Discovery Workflow", newTextContent(response.Text)), nil
		}
		return promptResult("Network Prefix Discovery Workflow", newTextContent("Welcome to Network Prefix Discovery Workflow!")), nil
	})

	log.Info("MCP ready - Forward Networks tools registered")
	return nil
}

// registerResources registers contextual resources with the MCP server
func registerResources(server *mcp.Server, s *usecases.Service, log ports.Logger) error {
	// Register network context as a resource
	server.AddResource(&mcp.Resource{
		URI:         "forward://network/context",
		Name:        "network_context",
		Description: "Current network context including available networks and queries",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		networkContext, err := s.GetNetworkContext(ctx, usecases.NetworkContextArgs{})
		if err != nil {
			return nil, fmt.Errorf("failed to get network context: %w", err)
		}

		contextStr, ok := networkContext.(string)
		if !ok {
			return nil, fmt.Errorf("network context is not a string")
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      "forward://network/context",
				MIMEType: "application/json",
				Text:     contextStr,
			}},
		}, nil
	})

	log.Debug("Successfully registered MCP resources")
	return nil
}
