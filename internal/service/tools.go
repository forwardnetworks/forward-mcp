package service

// Network Management Tool Arguments
type ListNetworksArgs struct {
	// No arguments needed for listing networks
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
type RunNQEQueryArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"required,description=ID of the network to query"`
	Query      string `json:"query,omitempty" jsonschema:"description=NQE query source code"`
	QueryID    string `json:"query_id,omitempty" jsonschema:"description=Query ID from NQE Library (alternative to query)"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"description=Specific snapshot ID to query (optional)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum number of rows to return"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=Number of rows to skip"`
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
