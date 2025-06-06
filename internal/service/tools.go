package service

// Network Management Tool Arguments
type ListNetworksArgs struct {
	// Dummy parameter for MCP framework compatibility (the tool doesn't actually use this)
	RandomString string `json:"random_string" jsonschema:"description=Dummy parameter for no-parameter tools"`
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

// Path Search Tool Arguments
type SearchPathsArgs struct {
	NetworkID               string `json:"network_id" jsonschema:"required,description=ID of the network to search paths in"`
	DstIP                   string `json:"dst_ip" jsonschema:"required,description=Destination IP address or subnet"`
	SrcIP                   string `json:"src_ip,omitempty" jsonschema:"description=Source IP address or subnet"`
	From                    string `json:"from,omitempty" jsonschema:"description=Device from which traffic originates"`
	Intent                  string `json:"intent,omitempty" jsonschema:"description=Search intent,enum=PREFER_DELIVERED|PREFER_VIOLATIONS|VIOLATIONS_ONLY"`
	IPProto                 int    `json:"ip_proto,omitempty" jsonschema:"description=IP protocol number"`
	SrcPort                 string `json:"src_port,omitempty" jsonschema:"description=Source port (e.g. '80' or '8080-8088')"`
	DstPort                 string `json:"dst_port,omitempty" jsonschema:"description=Destination port (e.g. '80' or '8080-8088')"`
	MaxResults              int    `json:"max_results,omitempty" jsonschema:"description=Maximum number of results to return (default: 1)"`
	IncludeNetworkFunctions bool   `json:"include_network_functions,omitempty" jsonschema:"description=Include detailed forwarding info for each hop"`
	SnapshotID              string `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID to use (optional)"`
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
	NetworkID  string                 `json:"network_id" description:"Network ID to run the query against"`
	QueryID    string                 `json:"query_id" description:"Query ID from NQE Library (use the 'queryId' field from list_nqe_queries response)"`
	SnapshotID string                 `json:"snapshot_id,omitempty" description:"Specific snapshot ID to query (optional)"`
	Parameters map[string]interface{} `json:"parameters,omitempty" description:"Optional parameters for the query"`
	Options    *NQEQueryOptions       `json:"options,omitempty" description:"Optional query options for sorting and filtering"`
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
	NetworkID string `json:"network_id" jsonschema:"required,description=ID of the network"`
}

// Snapshot Management Tool Arguments
type ListSnapshotsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"required,description=ID of the network"`
}

type GetLatestSnapshotArgs struct {
	NetworkID string `json:"network_id" jsonschema:"required,description=ID of the network"`
}

// Location Management Tool Arguments
type ListLocationsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"required,description=ID of the network"`
}

type CreateLocationArgs struct {
	NetworkID   string   `json:"network_id" jsonschema:"required,description=ID of the network"`
	Name        string   `json:"name" jsonschema:"required,description=Name of the location"`
	Description string   `json:"description,omitempty" jsonschema:"description=Description of the location"`
	Latitude    *float64 `json:"latitude,omitempty" jsonschema:"description=Latitude coordinate"`
	Longitude   *float64 `json:"longitude,omitempty" jsonschema:"description=Longitude coordinate"`
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
}

// GetConfigDiffArgs represents arguments for configuration comparison
type GetConfigDiffArgs struct {
	NetworkID      string                 `json:"network_id" jsonschema:"description=Network ID (use list_networks to find, or set default with set_default_network)"`
	BeforeSnapshot string                 `json:"before_snapshot" jsonschema:"required,description=Earlier snapshot ID for comparison"`
	AfterSnapshot  string                 `json:"after_snapshot" jsonschema:"required,description=Later snapshot ID for comparison"`
	DeviceFilter   string                 `json:"device_filter,omitempty" jsonschema:"description=Optional device name pattern to filter results"`
	Parameters     map[string]interface{} `json:"parameters,omitempty" jsonschema:"description=Additional query parameters"`
	Options        *NQEQueryOptions       `json:"options,omitempty" jsonschema:"description=Query options (limit, offset, etc.)"`
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
	// Empty struct - context doesn't need parameters
}

// Default Settings Management argument structures
type GetDefaultSettingsArgs struct {
	// No parameters needed to view current defaults
}

type SetDefaultNetworkArgs struct {
	NetworkIdentifier string `json:"network_identifier"`
}

// Semantic Cache and AI Enhancement Args
type GetCacheStatsArgs struct {
	// No parameters needed for cache stats
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
	Category    string `json:"category" jsonschema:"description=Filter by category to narrow results (e.g., 'Cloud', 'L3', 'Security', 'Device'). Use get_query_index_stats to see available categories."`
	Subcategory string `json:"subcategory" jsonschema:"description=Filter by subcategory (e.g., 'AWS', 'BGP', 'ACL', 'OSPF'). Use get_query_index_stats with detailed:true to see available subcategories."`
	IncludeCode bool   `json:"include_code" jsonschema:"description=Include NQE source code in results for advanced users (default: false). Warning: makes response much longer."`
}

// InitializeQueryIndexArgs represents arguments for building the AI query index
type InitializeQueryIndexArgs struct {
	RebuildIndex       bool `json:"rebuild_index" jsonschema:"description=Force rebuild of the query index from spec file (default: false). Only needed if spec file has been updated."`
	GenerateEmbeddings bool `json:"generate_embeddings" jsonschema:"description=Generate new AI embeddings for semantic search (default: false). Requires OpenAI API key and takes several minutes. Creates offline cache for fast searches."`
}

// GetQueryIndexStatsArgs represents arguments for query index statistics
type GetQueryIndexStatsArgs struct {
	Detailed bool `json:"detailed"`
}

// FindExecutableQueryArgs represents the arguments for finding executable queries
type FindExecutableQueryArgs struct {
	Query          string `json:"query" jsonschema:"required,description=Natural language description of what you want to analyze or accomplish. Be specific about the network analysis goal. Examples: 'show me all network devices', 'check device CPU and memory usage', 'find BGP neighbor information', 'compare configuration changes'."`
	Limit          int    `json:"limit" jsonschema:"description=Maximum number of executable query recommendations to return (default: 5, max: 10). Each result includes a real Forward Networks Query ID you can execute."`
	IncludeRelated bool   `json:"include_related" jsonschema:"description=Include the semantic search matches that led to these executable recommendations (default: false). Useful for understanding why these queries were suggested."`
}

// Smart Query Workflow Arguments
type SmartQueryWorkflowArgs struct {
	// No parameters needed for the workflow guide - it's a static documentation prompt
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
