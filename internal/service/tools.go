package service

// Network Management Tool Arguments
type ListNetworksArgs struct {
	Limit      int  `json:"limit,omitempty" jsonschema:"description=Maximum number of networks to return (default: 25, max: 100)"`
	Offset     int  `json:"offset,omitempty" jsonschema:"description=Number of networks to skip (default: 0)"`
	AllResults bool `json:"all_results,omitempty" jsonschema:"description=If true, fetch all networks using pagination and store in memory system"`
}

type CreateNetworkArgs struct {
	Name string `json:"name" jsonschema:"required,description=Name of the network to create"`
}

type DeleteNetworkArgs struct {
	NetworkID string `json:"network_id" jsonschema:"required,description=ID of the network to delete"`
}

type UpdateNetworkArgs struct {
	NetworkID   string `json:"network_id" jsonschema:"required,description=ID of the network to update"`
	Name        string `json:"name,omitempty" jsonschema:"description=New name for the network"`
	Description string `json:"description,omitempty" jsonschema:"description=New description for the network"`
}

// NQE Tool Arguments
type RunNQEQueryByStringArgs struct {
	NetworkID  string                 `json:"network_id" jsonschema:"required,description=ID of the network to query"`
	Query      string                 `json:"query" jsonschema:"required,description=NQE query source code"`
	SnapshotID string                 `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID to query (optional)"`
	Parameters map[string]interface{} `json:"parameters,omitempty" jsonschema:"description=Query parameters to use"`
	Options    *NQEQueryOptions       `json:"options,omitempty" jsonschema:"description=Query options like limit, offset, sorting, etc."`
}

type RunNQEQueryByIDArgs struct {
	NetworkID  string                 `json:"network_id" jsonschema:"required,description=Network ID to run the query against"`
	QueryID    string                 `json:"query_id" jsonschema:"required,description=Query ID from NQE Library (use the 'queryId' field from list_nqe_queries response)"`
	SnapshotID string                 `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID to query (optional)"`
	Parameters map[string]interface{} `json:"parameters,omitempty" jsonschema:"description=Optional parameters for the query"`
	Options    *NQEQueryOptions       `json:"options,omitempty" jsonschema:"description=Optional query options for sorting and filtering"`
	AllResults bool                   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all results using pagination (limit/offset) and aggregate them into a single response"`
}

type NQEQueryOptions struct {
	Limit   int               `json:"limit,omitempty" jsonschema:"description=Maximum number of rows to return"`
	Offset  int               `json:"offset,omitempty" jsonschema:"description=Number of rows to skip"`
	SortBy  []NQESortBy       `json:"sort_by,omitempty" jsonschema:"description=Sorting criteria for results"`
	Filters []NQEColumnFilter `json:"filters,omitempty" jsonschema:"description=Column filters to apply"`
	Format  string            `json:"format,omitempty" jsonschema:"description=Output format for results"`
}

type NQESortBy struct {
	ColumnName string `json:"column_name" jsonschema:"required,description=Name of the column to sort by"`
	Order      string `json:"order" jsonschema:"required,description=Sort order (ASC or DESC)"`
}

type NQEColumnFilter struct {
	ColumnName string `json:"column_name" jsonschema:"required,description=Name of the column to filter"`
	Value      string `json:"value" jsonschema:"required,description=Value to filter by"`
}

type ListNQEQueriesArgs struct {
	Directory string `json:"directory,omitempty" jsonschema:"description=Filter queries by directory (e.g. '/L3/Advanced/')"`
}

// Device Management Tool Arguments
type ListDevicesArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=ID of the network"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID (optional)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum number of devices to return"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=Number of devices to skip"`
}

type GetDeviceLocationsArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=ID of the network"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum number of device locations to return (default: 25, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=Number of device locations to skip (default: 0)"`
	AllResults bool   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all device locations using pagination and store in memory system"`
}

// Snapshot Management Tool Arguments
type ListSnapshotsArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=ID of the network"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum number of snapshots to return (default: 25, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=Number of snapshots to skip (default: 0)"`
	AllResults bool   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all snapshots using pagination and store in memory system"`
}

type GetLatestSnapshotArgs struct {
	NetworkID string `json:"network_id" jsonschema:"required,description=ID of the network"`
}

type DeleteSnapshotArgs struct {
	SnapshotID string `json:"snapshot_id" jsonschema:"required,description=ID of the snapshot to delete"`
}

// Location Management Tool Arguments
type ListLocationsArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=ID of the network"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum number of locations to return (default: 25, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=Number of locations to skip (default: 0)"`
	AllResults bool   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all locations using pagination and store in memory system"`
}

type CreateLocationArgs struct {
	NetworkID     string  `json:"network_id" jsonschema:"required,description=ID of the network"`
	ID            string  `json:"id,omitempty" jsonschema:"description=Optional custom ID for the location (if not provided, a numeric ID will be assigned)"`
	Name          string  `json:"name" jsonschema:"required,description=Name of the location"`
	Lat           float64 `json:"lat" jsonschema:"required,description=Latitude coordinate (angle from -90 to +90 degrees)"`
	Lng           float64 `json:"lng" jsonschema:"required,description=Longitude coordinate (angle from -180 to +180 degrees)"`
	City          string  `json:"city,omitempty" jsonschema:"description=Name of the closest city"`
	AdminDivision string  `json:"adminDivision,omitempty" jsonschema:"description=Administrative division (state, province, etc.)"`
	Country       string  `json:"country,omitempty" jsonschema:"description=Country name"`
}

type UpdateLocationArgs struct {
	NetworkID     string   `json:"network_id" jsonschema:"required,description=ID of the network"`
	LocationID    string   `json:"location_id" jsonschema:"required,description=ID of the location to update"`
	Name          string   `json:"name,omitempty" jsonschema:"description=New name for the location"`
	Lat           *float64 `json:"lat,omitempty" jsonschema:"description=New latitude coordinate (angle from -90 to +90 degrees)"`
	Lng           *float64 `json:"lng,omitempty" jsonschema:"description=New longitude coordinate (angle from -180 to +180 degrees)"`
	City          string   `json:"city,omitempty" jsonschema:"description=Name of the closest city"`
	AdminDivision string   `json:"adminDivision,omitempty" jsonschema:"description=Administrative division (state, province, etc.)"`
	Country       string   `json:"country,omitempty" jsonschema:"description=Country name"`
}

type DeleteLocationArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=ID of the network"`
	LocationID string `json:"location_id" jsonschema:"required,description=ID of the location to delete"`
}

type UpdateDeviceLocationsArgs struct {
	NetworkID string            `json:"network_id" jsonschema:"required,description=ID of the network"`
	Locations map[string]string `json:"locations" jsonschema:"required,description=Map of device IDs to location IDs"`
}

// Bulk create/update locations using PATCH
type CreateLocationsBulkArgs struct {
	NetworkID string                   `json:"network_id" jsonschema:"required,description=ID of the network"`
	Locations []CreateLocationItemArgs `json:"locations" jsonschema:"required,description=Array of locations to create or update"`
}

type CreateLocationItemArgs struct {
	ID            string   `json:"id,omitempty" jsonschema:"description=ID of existing location to update (if not provided, creates new location)"`
	Name          string   `json:"name,omitempty" jsonschema:"description=Name of the location (required if id not provided)"`
	Lat           *float64 `json:"lat,omitempty" jsonschema:"description=Latitude (-90 to +90)"`
	Lng           *float64 `json:"lng,omitempty" jsonschema:"description=Longitude (-180 to +180)"`
	City          string   `json:"city,omitempty" jsonschema:"description=Name of the closest city"`
	AdminDivision string   `json:"adminDivision,omitempty" jsonschema:"description=Administrative division (state, province, etc.)"`
	Country       string   `json:"country,omitempty" jsonschema:"description=Country name"`
}

// First-Class Query Tool Arguments - Critical Network Operations
type GetDeviceBasicInfoArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"required,description=ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"description=Query options like limit, offset, sorting, etc."`
}

type GetDeviceHardwareArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"required,description=ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"description=Query options like limit, offset, sorting, etc."`
}

type GetHardwareSupportArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"required,description=ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"description=Query options like limit, offset, sorting, etc."`
}

type GetOSSupportArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"required,description=ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"description=Query options like limit, offset, sorting, etc."`
}

// SearchConfigsArgs represents arguments for configuration search
type SearchConfigsArgs struct {
	NetworkID    string                 `json:"network_id" jsonschema:"description=Network ID (use list_networks to find, or set default with set_default_network)"`
	SnapshotID   string                 `json:"snapshot_id,omitempty" jsonschema:"description=Snapshot ID (optional, uses latest if not specified)"`
	SearchTerm   string                 `json:"search_term" jsonschema:"required,description=Text pattern to search for in configurations"`
	DeviceFilter string                 `json:"device_filter,omitempty" jsonschema:"description=Optional device name pattern to filter results"`
	Parameters   map[string]interface{} `json:"parameters,omitempty" jsonschema:"description=Additional query parameters"`
	Options      *NQEQueryOptions       `json:"options,omitempty" jsonschema:"description=Query options (limit, offset, etc.)"`
	AllResults   bool                   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all config matches using pagination and store in memory system"`
}

// GetConfigDiffArgs represents arguments for configuration comparison
type GetConfigDiffArgs struct {
	NetworkID      string                 `json:"network_id" jsonschema:"description=Network ID (use list_networks to find, or set default with set_default_network)"`
	BeforeSnapshot string                 `json:"before_snapshot" jsonschema:"required,description=Earlier snapshot ID for comparison"`
	AfterSnapshot  string                 `json:"after_snapshot" jsonschema:"required,description=Later snapshot ID for comparison"`
	DeviceFilter   string                 `json:"device_filter,omitempty" jsonschema:"description=Optional device name pattern to filter results"`
	Parameters     map[string]interface{} `json:"parameters,omitempty" jsonschema:"description=Additional query parameters"`
	Options        *NQEQueryOptions       `json:"options,omitempty" jsonschema:"description=Query options (limit, offset, etc.)"`
	AllResults     bool                   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all config diff results using pagination and store in memory system"`
}

type GetDeviceUtilitiesArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"required,description=ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID to query (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"description=Query options including limit, offset, sorting, and filtering"`
}

// Prompt Workflow Arguments
type NQEDiscoveryArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"description=Session ID for tracking workflow state"`
}

type NetworkDiscoveryArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"description=Session ID for tracking workflow state"`
}

// Resource Arguments
type NetworkContextArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// Default Settings Management argument structures
type GetDefaultSettingsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type SetDefaultNetworkArgs struct {
	NetworkIdentifier string `json:"network_identifier" jsonschema:"required,description=Network identifier (ID or name) to set as default"`
}

// Semantic Cache and AI Enhancement Args
type GetCacheStatsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type SuggestSimilarQueriesArgs struct {
	Query string `json:"query" jsonschema:"required,description=Query text to find similar queries for"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum number of suggestions to return (default: 5)"`
}

type ClearCacheArgs struct {
	ClearAll bool `json:"clear_all,omitempty" jsonschema:"description=Clear all cache entries instead of just expired ones"`
}

// AI-Powered Query Discovery Tools

// SearchNQEQueriesArgs represents arguments for intelligent query search
type SearchNQEQueriesArgs struct {
	Query       string `json:"query" jsonschema:"required,description=Natural language description of what you want to analyze. Be specific and descriptive. Good examples: 'show me AWS security vulnerabilities', 'find BGP routing issues', 'check interface utilization', 'devices with high CPU usage'. Avoid vague terms like 'network' or 'config'."`
	Limit       int    `json:"limit" jsonschema:"description=Maximum number of query suggestions to return (default: 10, max: 50)"`
	Category    string `json:"category" jsonschema:"description=Filter by category to narrow results (e.g., 'Cloud', 'L3', 'Security', 'Device')."`
	Subcategory string `json:"subcategory" jsonschema:"description=Filter by subcategory (e.g., 'AWS', 'BGP', 'ACL', 'OSPF')."`
	IncludeCode bool   `json:"include_code" jsonschema:"description=Include NQE source code in results for advanced users (default: false). Warning: makes response much longer."`
}

// InitializeQueryIndexArgs represents arguments for building the AI query index
type InitializeQueryIndexArgs struct {
	RebuildIndex       bool `json:"rebuild_index" jsonschema:"description=Force rebuild of the query index from spec file (default: false). Only needed if spec file has been updated."`
	GenerateEmbeddings bool `json:"generate_embeddings" jsonschema:"description=Generate new AI embeddings for semantic search (default: false). Requires OpenAI API key and takes several minutes. Creates offline cache for fast searches."`
}

// FindExecutableQueryArgs represents the arguments for finding executable queries
type FindExecutableQueryArgs struct {
	Query          string `json:"query" jsonschema:"required,description=Natural language description of what you want to analyze or accomplish. Be specific about the network analysis goal. Examples: 'show me all network devices', 'check device CPU and memory usage', 'find BGP neighbor information', 'compare configuration changes'."`
	Limit          int    `json:"limit" jsonschema:"description=Maximum number of executable query recommendations to return (default: 5, max: 10). Each result includes a real Forward Networks Query ID you can execute."`
	IncludeRelated bool   `json:"include_related" jsonschema:"description=Include the semantic search matches that led to these executable recommendations (default: false). Useful for understanding why these queries were suggested."`
}

// Smart Query Workflow Arguments
type SmartQueryWorkflowArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// Database Hydration Tools Arguments
type HydrateDatabaseArgs struct {
	ForceRefresh         bool `json:"force_refresh" jsonschema:"description=Force refresh all queries from API even if database has data (default: false)"`
	EnhancedMode         bool `json:"enhanced_mode" jsonschema:"description=Use enhanced API mode for metadata enrichment (default: true)"`
	MaxRetries           int  `json:"max_retries" jsonschema:"description=Maximum number of retry attempts for API calls (default: 3)"`
	RegenerateEmbeddings bool `json:"regenerate_embeddings" jsonschema:"description=Automatically regenerate AI embeddings after hydration for improved semantic search (default: false)"`
}

type RefreshQueryIndexArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type GetDatabaseStatusArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type GetQueryIndexStatsArgs struct {
	Detailed bool `json:"detailed,omitempty" jsonschema:"description=Include detailed statistics (default: false)"`
}

// Memory Management Tools Arguments
type CreateEntityArgs struct {
	Name     string                 `json:"name" jsonschema:"required,description=Name of the entity"`
	Type     string                 `json:"type" jsonschema:"required,description=Type of the entity (e.g., 'user', 'network', 'device', 'project')"`
	Metadata map[string]interface{} `json:"metadata" jsonschema:"description=Additional metadata for the entity"`
}

type CreateRelationArgs struct {
	FromID     string                 `json:"from_id" jsonschema:"required,description=ID of the source entity"`
	ToID       string                 `json:"to_id" jsonschema:"required,description=ID of the target entity"`
	Type       string                 `json:"type" jsonschema:"required,description=Type of the relation (e.g., 'owns', 'manages', 'depends_on')"`
	Properties map[string]interface{} `json:"properties" jsonschema:"description=Properties of the relation"`
}

type AddObservationArgs struct {
	EntityID string                 `json:"entity_id" jsonschema:"required,description=ID of the entity to add observation to"`
	Content  string                 `json:"content" jsonschema:"required,description=Content of the observation"`
	Type     string                 `json:"type" jsonschema:"required,description=Type of the observation (e.g., 'note', 'preference', 'behavior')"`
	Metadata map[string]interface{} `json:"metadata" jsonschema:"description=Additional metadata for the observation"`
}

type SearchEntitiesArgs struct {
	Query      string `json:"query" jsonschema:"description=Search query to find entities by name or observation content"`
	EntityType string `json:"entity_type" jsonschema:"description=Filter by entity type"`
	Limit      int    `json:"limit" jsonschema:"description=Maximum number of results to return (default: 50)"`
}

type GetEntityArgs struct {
	Identifier string `json:"identifier" jsonschema:"required,description=Entity ID or name to retrieve"`
}

type GetRelationsArgs struct {
	EntityID     string `json:"entity_id" jsonschema:"required,description=ID of the entity to get relations for"`
	RelationType string `json:"relation_type" jsonschema:"description=Filter by relation type"`
	Limit        int    `json:"limit,omitempty" jsonschema:"description=Maximum number of relations to return (default: 25, max: 100)"`
	Offset       int    `json:"offset,omitempty" jsonschema:"description=Number of relations to skip (default: 0)"`
	AllResults   bool   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all relations using pagination and store in memory system"`
}

type GetObservationsArgs struct {
	EntityID        string `json:"entity_id" jsonschema:"required,description=ID of the entity to get observations for"`
	ObservationType string `json:"observation_type" jsonschema:"description=Filter by observation type"`
	Limit           int    `json:"limit,omitempty" jsonschema:"description=Maximum number of observations to return (default: 25, max: 100)"`
	Offset          int    `json:"offset,omitempty" jsonschema:"description=Number of observations to skip (default: 0)"`
	AllResults      bool   `json:"all_results,omitempty" jsonschema:"description=If true, fetch all observations using pagination and store in memory system"`
}

type DeleteEntityArgs struct {
	EntityID string `json:"entity_id" jsonschema:"required,description=ID of the entity to delete"`
}

type DeleteRelationArgs struct {
	RelationID string `json:"relation_id" jsonschema:"required,description=ID of the relation to delete"`
}

type DeleteObservationArgs struct {
	ObservationID string `json:"observation_id" jsonschema:"required,description=ID of the observation to delete"`
}

type GetMemoryStatsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// API Analytics Tools Arguments
type GetQueryAnalyticsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"required,description=Network ID to get analytics for"`
}

// Instance Management Tool Arguments
type ListInstanceIDsArgs struct {
	// Dummy parameter for MCP framework compatibility
	Dummy string `json:"dummy,omitempty" jsonschema:"description=Dummy parameter for no-parameter tools"`
}

// For the config search tool schema/registration:
// Update the description or prompt to include:
//
// "To create a block pattern, use triple backticks (```) to start and end the pattern, and indent lines to show hierarchy. Example:
//
// pattern = ```
// interface
//   zone-member security
//   ip address {ip:string}
// ```
//
// Each line is a line pattern. Indentation defines parent/child relationships. Use curly braces for variable extraction (e.g., {ip:string}). For more, see the data extraction guide."

// Large NQE Results Workflow Arguments
type LargeNQEResultsWorkflowArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"description=Session ID for tracking workflow state"`
}

// Path Search Arguments
type SearchPathsArgs struct {
	NetworkID               string `json:"network_id" jsonschema:"required,description=Network ID to search in"`
	SnapshotID              string `json:"snapshot_id,omitempty" jsonschema:"description=Snapshot ID to use (optional, uses latest if omitted)"`
	From                    string `json:"from,omitempty" jsonschema:"description=Source device name"`
	SrcIP                   string `json:"src_ip,omitempty" jsonschema:"description=Source IP address or subnet"`
	DstIP                   string `json:"dst_ip" jsonschema:"required,description=Destination IP address or subnet"`
	IPProto                 *int   `json:"ip_proto,omitempty" jsonschema:"description=IP protocol number"`
	SrcPort                 string `json:"src_port,omitempty" jsonschema:"description=Source port"`
	DstPort                 string `json:"dst_port,omitempty" jsonschema:"description=Destination port"`
	Intent                  string `json:"intent,omitempty" jsonschema:"description=Search intent (PREFER_DELIVERED, PREFER_VIOLATIONS, VIOLATIONS_ONLY)"`
	MaxCandidates           int    `json:"max_candidates,omitempty" jsonschema:"description=Maximum number of candidates to consider"`
	MaxResults              int    `json:"max_results,omitempty" jsonschema:"description=Maximum number of results to return"`
	MaxReturnPathResults    int    `json:"max_return_path_results,omitempty" jsonschema:"description=Maximum number of return path results"`
	MaxSeconds              int    `json:"max_seconds,omitempty" jsonschema:"description=Maximum seconds per query"`
	IncludeNetworkFunctions bool   `json:"include_network_functions,omitempty" jsonschema:"description=Include network functions in results"`
}

// Path Search Workflow Arguments
type PathSearchWorkflowArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"description=Session ID for tracking workflow state"`
}

// Bloom Search Arguments
type BuildBloomFilterArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=Network ID to build filter for"`
	FilterType string `json:"filter_type" jsonschema:"required,description=Type of filter to build (device, interface, config)"`
	QueryID    string `json:"query_id" jsonschema:"required,description=NQE query ID to use for building the filter"`
	ChunkSize  int    `json:"chunk_size,omitempty" jsonschema:"description=Chunk size for processing (default: 200)"`
}

type SearchBloomFilterArgs struct {
	NetworkID   string   `json:"network_id" jsonschema:"required,description=Network ID to search in"`
	FilterType  string   `json:"filter_type" jsonschema:"required,description=Type of filter to search (device, interface, config)"`
	SearchTerms []string `json:"search_terms" jsonschema:"required,description=Search terms to look for"`
	EntityID    string   `json:"entity_id" jsonschema:"required,description=Entity ID containing the full dataset to search"`
}

type GetBloomFilterStatsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// Network Prefix Discovery and Connectivity Analysis
type NetworkPrefixDiscoveryArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"description=Session ID for tracking workflow state"`
	Step      string `json:"step,omitempty" jsonschema:"description=Current step in the workflow"`
}

type NetworkPrefixAnalysisArgs struct {
	NetworkID    string   `json:"network_id" jsonschema:"required,description=Network ID to analyze"`
	SnapshotID   string   `json:"snapshot_id,omitempty" jsonschema:"description=Snapshot ID to use (optional, uses latest if omitted)"`
	PrefixLevels []string `json:"prefix_levels,omitempty" jsonschema:"description=Aggregation levels to analyze (e.g., ['/8', '/16', '/24'])"`
	FromDevices  []string `json:"from_devices,omitempty" jsonschema:"description=Source devices to analyze"`
	ToDevices    []string `json:"to_devices,omitempty" jsonschema:"description=Destination devices to analyze"`
	Intent       string   `json:"intent,omitempty" jsonschema:"description=Search intent (PREFER_DELIVERED, PREFER_VIOLATIONS, VIOLATIONS_ONLY)"`
	MaxResults   int      `json:"max_results,omitempty" jsonschema:"description=Maximum number of results to return"`
}

type NetworkPrefixInfo struct {
	Prefix     string   `json:"prefix"`
	Device     string   `json:"device"`
	NetworkID  string   `json:"network_id"`
	Location   string   `json:"location,omitempty"`
	Aggregated bool     `json:"aggregated"`
	Subnets    []string `json:"subnets,omitempty"`
}

type ConnectivityAnalysisResult struct {
	FromPrefix       string   `json:"from_prefix"`
	ToPrefix         string   `json:"to_prefix"`
	FromDevice       string   `json:"from_device"`
	ToDevice         string   `json:"to_device"`
	Connectivity     string   `json:"connectivity"` // "CONNECTED", "PARTIAL", "DISCONNECTED"
	PathCount        int      `json:"path_count"`
	AggregationLevel string   `json:"aggregation_level"`
	Details          []string `json:"details,omitempty"`
}
