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
	}, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		if req != nil && req.Session != nil {
			ctx = ports.WithSessionID(ctx, req.Session.ID())
		}
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
	"skills_list":              annReadOnly,
	"skills_get":               annReadOnly,
	"search_paths":             annReadOnly,
	"search_paths_bulk":        annReadOnly,
	"analyze_network_prefixes": annReadOnly,
	"run_nqe_query_by_id":      annReadOnly,
	"list_nqe_queries":         annReadOnly,
	"compare_nqe_results":      annReadOnly,
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
	if err := registerSkills(server, svc, log); err != nil {
		return fmt.Errorf("register skills: %w", err)
	}
	return nil
}

// registerTools registers all Forward Networks tools with the MCP server
func registerTools(server *mcp.Server, s *usecases.Service) error {
	// Network Management Tools
	addTool(server, "list_networks",
		"Tool to list all networks with IDs, names, and descriptions. Use when discovering available networks or finding network IDs for queries. Supports pagination and memory storage for large datasets.",
		s.ListNetworks)

	addTool(server, "create_network",
		"Tool to create a new network in the Forward platform. Use when setting up a new network for monitoring and analysis. Requires network name; returns network with UUID for subsequent operations.",
		s.CreateNetwork)

	// addTool(server, "delete_network",
	// 	"Delete a network from the Forward platform. Requires network_id. WARNING: This permanently deletes all associated data.",
	// 	s.deleteNetwork)

	addTool(server, "update_network",
		"Tool to update network properties (name or description). Use when modifying network metadata. Requires network_id and at least one of: name or description.",
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
		"Tool to list available NQE queries from the Forward Networks query library. Use when browsing queries by directory path or discovering predefined queries for reports. Filter by directory (e.g., '/L3/Basic/', '/L3/Security/'). Returns query IDs for use with run_nqe_query_by_id. For semantic search use search_nqe_queries.",
		s.ListNQEQueries)

	addTool(server, "compare_nqe_results",
		"Tool to compare NQE query results between two snapshots. Requires before_snapshot_id, after_snapshot_id, and query_id.",
		s.CompareNQEResults)

	// First-Class Query Tools - Most Important Network Operations
	addTool(server, "get_device_basic_info",
		"Tool to retrieve device inventory with names, types, platforms, OS versions, and management IPs. Use when discovering network devices, building device lists, or starting network analysis. Supports filters and pagination. Combine with get_device_hardware for hardware details and serial numbers.",
		s.GetDeviceBasicInfo)

	addTool(server, "get_device_hardware",
		"Tool to retrieve device hardware information including models, serial numbers, and vendor details. Use when building hardware inventory, planning hardware refresh, or validating support contracts. Returns hardware specifications and asset tracking data. Supports filters and pagination.",
		s.GetDeviceHardware)

	addTool(server, "get_hardware_support",
		"Tool to check hardware support status including end-of-life dates and support contract information. Use when conducting security compliance audits, planning hardware refresh, or assessing risk. Returns EOL dates, vulnerability information, and recommended upgrade paths. Critical for compliance validation.",
		s.GetHardwareSupport)

	addTool(server, "get_os_support",
		"Tool to check operating system support status including OS versions, support end dates, and security patch status. Use when conducting security compliance audits, planning OS upgrades, or assessing vulnerabilities. Returns version information, EOL dates, and upgrade recommendations. Critical for security compliance.",
		s.GetOSSupport)

	addTool(server, "search_configs",
		"Tool to search device configurations for specific patterns, commands, or settings across the network. Use when finding interface configurations, security policies, or auditing compliance. Supports hierarchical patterns with indentation and variable extraction using {name:type} syntax. Filter by device names for targeted searches.",
		s.SearchConfigs)

	addTool(server, "get_config_diff",
		"Tool to compare network configurations between two snapshots and identify changes. Use when tracking configuration changes, troubleshooting drift, or auditing modifications. Returns detailed diff of configuration changes between specified snapshots.",
		s.GetConfigDiff)

	// Device Management Tools
	addTool(server, "list_devices",
		"Tool to list all devices in a network with names, types, and operational status. Use when building device inventory or discovering network devices. Requires network_id. Returns basic device information. Supports pagination with limit and offset.",
		s.ListDevices)

	addTool(server, "get_device_locations",
		"Tool to retrieve device-to-location mappings showing which devices are assigned to physical locations. Use when planning topology or organizing devices by site. Requires network_id. Returns device location assignments.",
		s.GetDeviceLocations)

	// Snapshot Management Tools
	addTool(server, "list_snapshots",
		"Tool to list network configuration snapshots with timestamps and status. Use when viewing configuration history or finding specific snapshots for queries. Requires network_id. Returns historical network states. Supports pagination and memory storage for large datasets.",
		s.ListSnapshots)

	addTool(server, "get_latest_snapshot",
		"Tool to get the most recent processed snapshot for a network. Use when you need the current network state ID for queries. Requires network_id. Returns latest snapshot with ID and timestamp.",
		s.GetLatestSnapshot)

	addTool(server, "delete_snapshot",
		"Tool to permanently delete a network snapshot and its associated historical data. Use when cleaning up old snapshots to free storage. Requires snapshot_id. WARNING: This action cannot be undone.",
		s.DeleteSnapshot)

	// Location Management Tools
	addTool(server, "list_locations",
		"Tool to list all physical locations in a network with names, coordinates, and site information. Use when viewing network topology or organizing devices by site. Requires network_id. Returns locations with lat/lng coordinates. Supports pagination and memory storage.",
		s.ListLocations)

	addTool(server, "create_location",
		"Tool to create a new physical location for device organization. Use when setting up new sites or data centers. Requires network_id, name, latitude, and longitude. Optional: city, adminDivision, country. Returns created location with ID.",
		s.CreateLocation)

	addTool(server, "update_location",
		"Tool to modify an existing physical location's details. Use when correcting location information or updating coordinates. Requires network_id and location_id. Optional: name, latitude, longitude, city, adminDivision, country.",
		s.UpdateLocation)

	addTool(server, "delete_location",
		"Tool to remove a physical location from a network. Use when decommissioning sites or cleaning up unused locations. Requires network_id and location_id. Devices assigned to this location will be unassigned.",
		s.DeleteLocation)

	addTool(server, "create_locations_bulk",
		"Tool to create or update multiple locations in a single operation for efficiency. Use when importing site data or bulk location setup. Requires network_id and array of locations. Locations with existing IDs are updated; new ones are created.",
		s.CreateLocationsBulk)

	addTool(server, "update_device_locations",
		"Tool to assign multiple devices to physical locations in bulk. Use when organizing devices by site or importing topology data. Requires network_id and map of device IDs to location IDs. Note: Cloud devices (CSR1KV, PAN-FW) cannot be assigned to physical locations.",
		s.UpdateDeviceLocations)

	// Default Settings Management Tools
	addTool(server, "get_default_settings",
		"Tool to view current default settings including default network ID, snapshot ID, and query limits. Use when checking session configuration or troubleshooting default behavior. Returns current defaults for this session.",
		s.GetDefaultSettings)

	addTool(server, "set_default_network",
		"Tool to set the default network for all subsequent operations. Use when working with a single network to avoid repeating network_id in every tool call. Accepts network ID or network name. Applies to all tools until changed.",
		s.SetDefaultNetwork)

	// Semantic Cache and AI Enhancement Tools
	addTool(server, "get_cache_stats",
		"Tool to view semantic cache performance statistics including hit rates, total queries, and efficiency metrics. Use when monitoring cache performance or troubleshooting slow queries. Returns cache statistics and memory usage.",
		s.GetCacheStats)

	addTool(server, "suggest_similar_queries",
		"Tool to find NQE queries similar to your query intent using semantic similarity. Use when discovering related queries or finding alternatives. Requires query intent description. Returns ranked query suggestions.",
		s.SuggestSimilarQueries)

	addTool(server, "clear_cache",
		"Tool to remove expired entries from the semantic cache and free memory. Use when cache is full or performance degrades. Removes only expired entries; active cache remains. Returns cleanup statistics.",
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
		"Tool to initialize or rebuild the AI-powered NQE query index from spec file. Use at startup or when receiving 'query index is empty' errors. Required before using search_nqe_queries. Can generate embeddings for semantic search if OpenAI API key available.",
		s.InitializeQueryIndex)

	// Database Hydration Tools
	addTool(server, "hydrate_database",
		"Tool to load NQE queries from Forward Networks API into local database. Use when refreshing query metadata or ensuring search performance. Automatically refreshes query index. Optionally regenerates AI embeddings. Run periodically to stay current with API changes.",
		s.HydrateDatabase)

	addTool(server, "refresh_query_index",
		"Tool to rebuild in-memory NQE query index from database. Use after hydrate_database or when index seems stale. Much faster than full initialization. Returns index statistics and query count.",
		s.RefreshQueryIndex)

	addTool(server, "get_database_status",
		"Tool to check NQE database health including query counts, last update timestamp, metadata coverage, and performance metrics. Use when troubleshooting search issues or verifying database freshness. Returns detailed status information.",
		s.GetDatabaseStatus)

	// Memory Management Tools
	addTool(server, "create_entity",
		"Tool to create a new entity (person, network, device, project, or concept) in the knowledge graph. Use when storing information for later retrieval. Requires name and type. Returns created entity with ID.",
		s.CreateEntity)

	addTool(server, "create_relation",
		"Tool to create a relationship between two entities expressing how they connect (e.g., 'owns', 'manages', 'depends_on'). Use when building knowledge connections. Requires from_entity, to_entity, and relation_type.",
		s.CreateRelation)

	addTool(server, "add_observation",
		"Tool to add a timestamped fact, note, preference, or behavior to an entity. Use when recording discoveries or important information. Requires entity name and observation text. Returns observation with timestamp.",
		s.AddObservation)

	addTool(server, "search_entities",
		"Tool to search for entities in the knowledge graph by name, type, or observation content using full-text search. Use when finding stored information. Returns matching entities.",
		s.SearchEntities)

	addTool(server, "get_entity",
		"Tool to retrieve a specific entity by ID or name with all its details. Use when looking up stored information about a person, network, device, or concept. Returns entity with metadata.",
		s.GetEntity)

	addTool(server, "get_relations",
		"Tool to retrieve all relationships for a specific entity showing its connections. Use when exploring knowledge network. Requires entity name. Returns list of relations.",
		s.GetRelations)

	addTool(server, "get_observations",
		"Tool to retrieve all timestamped facts, notes, and preferences for a specific entity. Use when reviewing stored information. Requires entity name. Returns list of observations.",
		s.GetObservations)

	addTool(server, "delete_entity",
		"Tool to permanently remove an entity and all its relations and observations from the knowledge graph. Use when cleaning up. Requires entity name. WARNING: Cannot be undone.",
		s.DeleteEntity)

	addTool(server, "delete_relation",
		"Tool to remove a specific relationship between entities. Use when connections are no longer relevant. Requires from_entity, to_entity, and relation_type.",
		s.DeleteRelation)

	addTool(server, "delete_observation",
		"Tool to remove a specific observation from an entity. Use when removing outdated or incorrect information. Requires entity name and observation ID.",
		s.DeleteObservation)

	addTool(server, "get_memory_stats",
		"Tool to view memory system statistics including counts of entities, relations, and observations by type. Use when monitoring knowledge graph usage. Returns detailed statistics.",
		s.GetMemoryStats)

	// API Analytics Tools
	addTool(server, "get_query_analytics",
		"Tool to view query analytics for a network including query counts, execution times, result patterns, and usage trends. Use when analyzing query performance or optimizing queries. Requires network_id. Returns analytics from memory system.",
		s.GetQueryAnalytics)

	// Instance Management Tools
	addTool(server, "list_instance_ids",
		"Tool to list all Forward Networks instance IDs in the database with query counts and sync dates. Use when finding the correct instance ID for FORWARD_INSTANCE_ID environment variable. Returns instance IDs and metadata.",
		s.ListInstanceIDs)

	// Tool handler for get_nqe_result_chunks
	addTool(server, "get_nqe_result_chunks",
		"Tool to retrieve chunked NQE query results from memory system. Use when accessing large stored query results. Provide entity_id or (query_id, network_id, snapshot_id). Optional chunk_index for single chunk. Returns result chunks.",
		s.GetNQEResultChunks)

	// Add get_nqe_result_summary tool handler
	addTool(server, "get_nqe_result_summary",
		"Tool to get a summary of stored NQE result including row count, columns, and preview rows. Use when checking result size before loading full dataset. Provide entity_id or (query_id, network_id, snapshot_id). Returns summary information.",
		s.GetNQEResultSummary)

	// Add analyze_nqe_result_sql tool handler
	addTool(server, "analyze_nqe_result_sql",
		"Tool to run SQL queries on stored NQE results for advanced analysis. Use when performing aggregations, filtering, or joins on stored data. Requires entity_id and SQL query. Example: SELECT COUNT(*) FROM nqe_result. Returns query results.",
		s.AnalyzeNQEResultSQL)

	// Add bloom search tool handlers
	addTool(server, "build_bloom_filter",
		"Tool to create a bloom filter index from NQE query results for ultra-fast searching of large datasets. Use when you need instant membership testing on results with >100 items. Creates persistent index. Returns filter statistics.",
		s.BuildBloomFilter)

	addTool(server, "search_bloom_filter",
		"Tool to perform sub-millisecond searches in bloom filter indexes. Use when checking if items exist in large datasets. Requires filter_type and search_terms. Returns matching items.",
		s.SearchBloomFilter)

	addTool(server, "get_bloom_filter_stats",
		"Tool to view statistics and performance metrics for all bloom filter indexes. Use when monitoring filter performance. Returns filter metrics including size, item count, and false positive rate.",
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
