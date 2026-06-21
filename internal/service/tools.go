package service

// Network Management Tool Arguments
type ListNetworksArgs struct {
	Limit      int  `json:"limit,omitempty" jsonschema:"Maximum number of networks to return (default: 25, max: 100)"`
	Offset     int  `json:"offset,omitempty" jsonschema:"Number of networks to skip (default: 0)"`
	AllResults bool `json:"all_results,omitempty" jsonschema:"If true, fetch all networks using pagination and store in memory system"`
}

type CreateNetworkArgs struct {
	Name string `json:"name" jsonschema:"Name of the network to create"`
}

type DeleteNetworkArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network to delete"`
}

type UpdateNetworkArgs struct {
	NetworkID   string `json:"network_id" jsonschema:"ID of the network to update"`
	Name        string `json:"name,omitempty" jsonschema:"New name for the network"`
	Description string `json:"description,omitempty" jsonschema:"New description for the network"`
}

// NQE Tool Arguments
type RunNQEQueryByStringArgs struct {
	NetworkID  string                 `json:"network_id,omitempty" jsonschema:"Network ID to run the query against (optional if a default network is set)"`
	Query      string                 `json:"query" jsonschema:"NQE query source code to execute directly through Forward"`
	SnapshotID string                 `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID to query (optional)"`
	Parameters map[string]interface{} `json:"parameters,omitempty" jsonschema:"Query parameters to use"`
	Options    *NQEQueryOptions       `json:"options,omitempty" jsonschema:"Query options like limit, offset, sorting, etc."`
	AllResults bool                   `json:"all_results,omitempty" jsonschema:"If true, fetch all results using pagination (limit/offset) and aggregate them into a single response"`
}

type RunNQEQueryByIDArgs struct {
	NetworkID  string                 `json:"network_id,omitempty" jsonschema:"Network ID to run the query against (optional if a default network is set)"`
	QueryID    string                 `json:"query_id" jsonschema:"Query ID from NQE Library (use the 'queryId' field from list_nqe_queries response)"`
	SnapshotID string                 `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID to query (optional)"`
	Parameters map[string]interface{} `json:"parameters,omitempty" jsonschema:"Optional parameters for the query"`
	Options    *NQEQueryOptions       `json:"options,omitempty" jsonschema:"Optional query options for sorting and filtering"`
	AllResults bool                   `json:"all_results,omitempty" jsonschema:"If true, fetch all results using pagination (limit/offset) and aggregate them into a single response"`
}

type StartNQEQueryArgs struct {
	NetworkID     string                     `json:"network_id,omitempty" jsonschema:"Network ID to run the query against (optional if a default network is set)"`
	SnapshotID    string                     `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID to query (optional)"`
	Query         string                     `json:"query,omitempty" jsonschema:"Raw NQE query source code to execute asynchronously. Provide exactly one of query or query_id"`
	QueryID       string                     `json:"query_id,omitempty" jsonschema:"NQE Library query ID to execute asynchronously. Provide exactly one of query or query_id"`
	CommitID      string                     `json:"commit_id,omitempty" jsonschema:"Optional commit ID for a specific version of the query_id"`
	Parameters    map[string]interface{}     `json:"parameters,omitempty" jsonschema:"Optional parameter values for the query"`
	ColumnFilters []NQEExecutionColumnFilter `json:"column_filters,omitempty" jsonschema:"Optional async NQE column filters"`
	SortKeys      []NQESortBy                `json:"sort_keys,omitempty" jsonschema:"Optional async NQE sort keys"`
}

type GetNQEQueryStatusArgs struct {
	NetworkID    string `json:"network_id,omitempty" jsonschema:"Network ID where the async NQE execution was submitted (optional if a default network is set)"`
	ExecutionKey string `json:"execution_key" jsonschema:"Execution key returned by start_nqe_query"`
}

type GetNQEQueryResultArgs struct {
	NetworkID    string `json:"network_id,omitempty" jsonschema:"Network ID where the async NQE execution was submitted (optional if a default network is set)"`
	ExecutionKey string `json:"execution_key" jsonschema:"Execution key returned by start_nqe_query"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Zero-based index of the first row to return"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of rows to return"`
	AllResults   bool   `json:"all_results,omitempty" jsonschema:"If true, fetch all available result rows using pagination and aggregate them into a single response"`
}

type NQEQueryOptions struct {
	Limit   int               `json:"limit,omitempty" jsonschema:"Maximum number of rows to return"`
	Offset  int               `json:"offset,omitempty" jsonschema:"Number of rows to skip"`
	SortBy  []NQESortBy       `json:"sort_by,omitempty" jsonschema:"Sorting criteria for results"`
	Filters []NQEColumnFilter `json:"filters,omitempty" jsonschema:"Column filters to apply"`
	Format  string            `json:"format,omitempty" jsonschema:"Output format for results"`
}

type NQESortBy struct {
	ColumnName string `json:"column_name" jsonschema:"Name of the column to sort by"`
	Order      string `json:"order" jsonschema:"Sort order (ASC or DESC)"`
}

type NQEColumnFilter struct {
	ColumnName string `json:"column_name" jsonschema:"Name of the column to filter"`
	Value      string `json:"value" jsonschema:"Value to filter by"`
}

type NQEExecutionColumnFilter struct {
	ColumnName string `json:"column_name" jsonschema:"Name of the column to filter"`
	Operator   string `json:"operator" jsonschema:"Filter operator: DEFAULT or IS_BETWEEN"`
	Value      string `json:"value,omitempty" jsonschema:"Value for DEFAULT column filter"`
	LowerBound string `json:"lower_bound,omitempty" jsonschema:"Inclusive lower bound for IS_BETWEEN column filter"`
	UpperBound string `json:"upper_bound,omitempty" jsonschema:"Inclusive upper bound for IS_BETWEEN column filter"`
}

type ListNQEQueriesArgs struct {
	Directory string `json:"directory,omitempty" jsonschema:"Filter queries by directory (e.g. '/L3/Advanced/')"`
}

// Device Management Tool Arguments
type ListDevicesArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of devices to return"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of devices to skip"`
}

type GetMissingDevicesArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional)"`
}

type GetDeviceLocationsArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of device locations to return (default: 25, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of device locations to skip (default: 0)"`
	AllResults bool   `json:"all_results,omitempty" jsonschema:"If true, fetch all device locations using pagination and store in memory system"`
}

// Snapshot Management Tool Arguments
type ListSnapshotsArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of snapshots to return (default: 25, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of snapshots to skip (default: 0)"`
	AllResults bool   `json:"all_results,omitempty" jsonschema:"If true, fetch all snapshots using pagination and store in memory system"`
}

type GetLatestSnapshotArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type DeleteSnapshotArgs struct {
	SnapshotID string `json:"snapshot_id" jsonschema:"ID of the snapshot to delete"`
}

// Location Management Tool Arguments
type ListLocationsArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of locations to return (default: 25, max: 100)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of locations to skip (default: 0)"`
	AllResults bool   `json:"all_results,omitempty" jsonschema:"If true, fetch all locations using pagination and store in memory system"`
}

type CreateLocationArgs struct {
	NetworkID     string  `json:"network_id" jsonschema:"ID of the network"`
	ID            string  `json:"id,omitempty" jsonschema:"Optional custom ID for the location (if not provided, a numeric ID will be assigned)"`
	Name          string  `json:"name" jsonschema:"Name of the location"`
	Lat           float64 `json:"lat" jsonschema:"Latitude coordinate (angle from -90 to +90 degrees)"`
	Lng           float64 `json:"lng" jsonschema:"Longitude coordinate (angle from -180 to +180 degrees)"`
	City          string  `json:"city,omitempty" jsonschema:"Name of the closest city"`
	AdminDivision string  `json:"adminDivision,omitempty" jsonschema:"Administrative division (state, province, etc.)"`
	Country       string  `json:"country,omitempty" jsonschema:"Country name"`
}

type UpdateLocationArgs struct {
	NetworkID     string   `json:"network_id" jsonschema:"ID of the network"`
	LocationID    string   `json:"location_id" jsonschema:"ID of the location to update"`
	Name          string   `json:"name,omitempty" jsonschema:"New name for the location"`
	Lat           *float64 `json:"lat,omitempty" jsonschema:"New latitude coordinate (angle from -90 to +90 degrees)"`
	Lng           *float64 `json:"lng,omitempty" jsonschema:"New longitude coordinate (angle from -180 to +180 degrees)"`
	City          string   `json:"city,omitempty" jsonschema:"Name of the closest city"`
	AdminDivision string   `json:"adminDivision,omitempty" jsonschema:"Administrative division (state, province, etc.)"`
	Country       string   `json:"country,omitempty" jsonschema:"Country name"`
}

type DeleteLocationArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	LocationID string `json:"location_id" jsonschema:"ID of the location to delete"`
}

type UpdateDeviceLocationsArgs struct {
	NetworkID string            `json:"network_id" jsonschema:"ID of the network"`
	Locations map[string]string `json:"locations" jsonschema:"Map of device IDs to location IDs"`
}

// Network setup and collection arguments
type ListClassicDevicesArgs struct {
	NetworkID string   `json:"network_id" jsonschema:"ID of the network"`
	With      []string `json:"with,omitempty" jsonschema:"Optional related data to include: tags, locationId, testResult"`
	Limit     int      `json:"limit,omitempty" jsonschema:"Maximum number of classic devices to return (default: 25, max: 100)"`
	Offset    int      `json:"offset,omitempty" jsonschema:"Number of classic devices to skip (default: 0)"`
}

type GetClassicDeviceArgs struct {
	NetworkID  string   `json:"network_id" jsonschema:"ID of the network"`
	DeviceName string   `json:"device_name" jsonschema:"Classic device name"`
	With       []string `json:"with,omitempty" jsonschema:"Optional related data to include: tags, locationId, testResult"`
}

type UpsertClassicDevicesArgs struct {
	NetworkID string                   `json:"network_id" jsonschema:"ID of the network"`
	Devices   []map[string]interface{} `json:"devices" jsonschema:"Classic devices using Forward NewClassicDevice JSON. Each device requires name and host; optional fields include type, port, cliCredentialId, cliCredential2Id, snmpCredentialId, enableSnmpCollection, collect, note, and collection/performance threshold settings."`
}

type DeleteClassicDevicesArgs struct {
	NetworkID string   `json:"network_id" jsonschema:"ID of the network"`
	Names     []string `json:"names" jsonschema:"Classic device names to delete"`
}

type ListCliCredentialsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type CreateCliCredentialArgs struct {
	NetworkID  string                 `json:"network_id" jsonschema:"ID of the network"`
	Credential map[string]interface{} `json:"credential" jsonschema:"Forward NewCliCredential JSON. Required fields: type and name. For LOGIN/SHELL/PRIVILEGED_MODE/EXPERT_MODE include password; username is required for LOGIN and SHELL. For SSH_KEY include username and sshKey instead of password; sshCert is optional. Optional autoAssociate and privilegedModePasswordId follow the Forward API."`
}

type DeleteCliCredentialArgs struct {
	NetworkID    string `json:"network_id" jsonschema:"ID of the network"`
	CredentialID string `json:"credential_id" jsonschema:"CLI credential ID to delete"`
}

type ListSnmpCredentialsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type CreateSnmpCredentialArgs struct {
	NetworkID  string                 `json:"network_id" jsonschema:"ID of the network"`
	Credential map[string]interface{} `json:"credential" jsonschema:"Forward SNMP credential JSON. Required fields: name and version. For V2C include communityString. For V3 include authSettings with username, authType, password, and optional privacyProtocol/privacyPassword. Optional fields include port, timeoutSec, and autoAssociate."`
}

type DeleteSnmpCredentialArgs struct {
	NetworkID    string `json:"network_id" jsonschema:"ID of the network"`
	CredentialID string `json:"credential_id" jsonschema:"SNMP credential ID to delete"`
}

type GetPerformanceSettingsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type UpdatePerformanceSettingsArgs struct {
	NetworkID string                 `json:"network_id" jsonschema:"ID of the network"`
	Settings  map[string]interface{} `json:"settings" jsonschema:"Performance settings patch. Set enabled to true/false and optionally intervalMinutes (1-1440). Network must have a supported collector; enabling collection also requires SNMP credentials and device enableSnmpCollection settings."`
}

type ListPerformanceDevicesArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID for the performance time context (optional; uses latest if omitted)"`
	StartTime  string `json:"start_time,omitempty" jsonschema:"Inclusive RFC3339 start time (optional; defaults to Forward's history window)"`
	EndTime    string `json:"end_time,omitempty" jsonschema:"Exclusive RFC3339 end time (optional; defaults based on snapshot time or now)"`
}

type GetDevicePerformanceArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	DeviceName string `json:"device_name" jsonschema:"Device name"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID for the performance time context (optional; uses latest if omitted)"`
	StartTime  string `json:"start_time,omitempty" jsonschema:"Inclusive RFC3339 start time (optional; defaults to Forward's history window)"`
	EndTime    string `json:"end_time,omitempty" jsonschema:"Exclusive RFC3339 end time (optional; defaults based on snapshot time or now)"`
	MaxSamples int    `json:"max_samples,omitempty" jsonschema:"Maximum samples to return (default 400)"`
}

type GetInterfacePerformanceArgs struct {
	NetworkID     string `json:"network_id" jsonschema:"ID of the network"`
	DeviceName    string `json:"device_name" jsonschema:"Device name"`
	InterfaceName string `json:"interface_name" jsonschema:"Interface name exactly as Forward models it"`
	SnapshotID    string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID for the performance time context (optional; uses latest if omitted)"`
	StartTime     string `json:"start_time,omitempty" jsonschema:"Inclusive RFC3339 start time (optional; defaults to Forward's history window)"`
	EndTime       string `json:"end_time,omitempty" jsonschema:"Exclusive RFC3339 end time (optional; defaults based on snapshot time or now)"`
	MaxSamples    int    `json:"max_samples,omitempty" jsonschema:"Maximum samples to return (default 400)"`
}

type GetCollectorStatusArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type StartCollectionArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type StartCollectionTaskArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type GetCollectorTaskArgs struct {
	TaskID string `json:"task_id" jsonschema:"Collector task ID returned by start_collection_task"`
}

type CancelCollectionArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type ListCollectionSchedulesArgs struct {
	NetworkID string `json:"network_id" jsonschema:"ID of the network"`
}

type WaitForLatestSnapshotArgs struct {
	NetworkID           string `json:"network_id" jsonschema:"ID of the network"`
	PreviousSnapshotID  string `json:"previous_snapshot_id,omitempty" jsonschema:"Optional snapshot ID to wait past; returns only when latest processed snapshot differs"`
	TimeoutSeconds      int    `json:"timeout_seconds,omitempty" jsonschema:"Maximum seconds to wait (default: 300, max: 1800)"`
	PollIntervalSeconds int    `json:"poll_interval_seconds,omitempty" jsonschema:"Seconds between polls (default: 10, min: 2)"`
}

// Bulk create/update locations using PATCH
type CreateLocationsBulkArgs struct {
	NetworkID string                   `json:"network_id" jsonschema:"ID of the network"`
	Locations []CreateLocationItemArgs `json:"locations" jsonschema:"Array of locations to create or update"`
}

type CreateLocationItemArgs struct {
	ID            string   `json:"id,omitempty" jsonschema:"ID of existing location to update (if not provided, creates new location)"`
	Name          string   `json:"name,omitempty" jsonschema:"Name of the location (required if id not provided)"`
	Lat           *float64 `json:"lat,omitempty" jsonschema:"Latitude (-90 to +90)"`
	Lng           *float64 `json:"lng,omitempty" jsonschema:"Longitude (-180 to +180)"`
	City          string   `json:"city,omitempty" jsonschema:"Name of the closest city"`
	AdminDivision string   `json:"adminDivision,omitempty" jsonschema:"Administrative division (state, province, etc.)"`
	Country       string   `json:"country,omitempty" jsonschema:"Country name"`
}

// First-Class Query Tool Arguments - Critical Network Operations
type GetDeviceBasicInfoArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"Query options like limit, offset, sorting, etc."`
}

type GetDeviceHardwareArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"Query options like limit, offset, sorting, etc."`
}

type GetHardwareSupportArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"Query options like limit, offset, sorting, etc."`
}

type GetOSSupportArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"Query options like limit, offset, sorting, etc."`
}

// SearchConfigsArgs represents arguments for configuration search
type SearchConfigsArgs struct {
	NetworkID    string                 `json:"network_id,omitempty" jsonschema:"Network ID (use list_networks to find, or set default with set_default_network)"`
	SnapshotID   string                 `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID (optional, uses latest if not specified)"`
	SearchTerm   string                 `json:"search_term" jsonschema:"Text pattern to search for in configurations"`
	DeviceFilter string                 `json:"device_filter,omitempty" jsonschema:"Optional device name pattern to filter results"`
	Parameters   map[string]interface{} `json:"parameters,omitempty" jsonschema:"Additional query parameters"`
	Options      *NQEQueryOptions       `json:"options,omitempty" jsonschema:"Query options (limit, offset, etc.)"`
	AllResults   bool                   `json:"all_results,omitempty" jsonschema:"If true, fetch all config matches using pagination and store in memory system"`
}

// GetConfigDiffArgs represents arguments for configuration comparison
type GetConfigDiffArgs struct {
	NetworkID      string                 `json:"network_id,omitempty" jsonschema:"Network ID (use list_networks to find, or set default with set_default_network)"`
	BeforeSnapshot string                 `json:"before_snapshot" jsonschema:"Earlier snapshot ID for comparison"`
	AfterSnapshot  string                 `json:"after_snapshot" jsonschema:"Later snapshot ID for comparison"`
	DeviceFilter   string                 `json:"device_filter,omitempty" jsonschema:"Optional device name pattern to filter results"`
	Parameters     map[string]interface{} `json:"parameters,omitempty" jsonschema:"Additional query parameters"`
	Options        *NQEQueryOptions       `json:"options,omitempty" jsonschema:"Query options (limit, offset, etc.)"`
	AllResults     bool                   `json:"all_results,omitempty" jsonschema:"If true, fetch all config diff results using pagination and store in memory system"`
}

// GetNQEDiffArgs represents arguments for comparing arbitrary NQE results.
type GetNQEDiffArgs struct {
	BeforeSnapshot string                 `json:"before_snapshot" jsonschema:"Earlier snapshot ID to use as the base of the diff"`
	AfterSnapshot  string                 `json:"after_snapshot" jsonschema:"Later snapshot ID to diff against the base snapshot"`
	QueryID        string                 `json:"query_id" jsonschema:"NQE Library query ID to diff between snapshots"`
	CommitID       string                 `json:"commit_id,omitempty" jsonschema:"Optional query commit ID to run a specific query version"`
	Parameters     map[string]interface{} `json:"parameters,omitempty" jsonschema:"Optional parameter values for the NQE query"`
	Options        *NQEQueryOptions       `json:"options,omitempty" jsonschema:"Query options. Use filters or sort_by on ChangeType for ADDED, DELETED, or MODIFIED rows"`
}

type GetSnapshotDiffSummaryArgs struct {
	BeforeSnapshot string   `json:"before_snapshot" jsonschema:"Earlier snapshot ID to use as the base of the diff"`
	AfterSnapshot  string   `json:"after_snapshot" jsonschema:"Later snapshot ID to diff against the base snapshot"`
	Include        []string `json:"include,omitempty" jsonschema:"Optional diff domains to include. Defaults to broad overview: files, devices, cloud-objects, interfaces, acl, cloud-acl, nat, arp, mac, topology, l2, l3, checks, inventory-queries, routing-loop, vulnerabilities"`
}

type GetSnapshotDiffArgs struct {
	BeforeSnapshot string            `json:"before_snapshot" jsonschema:"Earlier snapshot ID to use as the base of the diff"`
	AfterSnapshot  string            `json:"after_snapshot" jsonschema:"Later snapshot ID to diff against the base snapshot"`
	DiffType       string            `json:"diff_type" jsonschema:"Diff domain: files, devices, cloud-objects, interfaces, topology, l2/vlans, acl, cloud-acl, nat, l3/routes, arp, mac, checks, inventory-queries, routing-loop, vulnerabilities"`
	View           string            `json:"view,omitempty" jsonschema:"Optional view for domains that support it, such as summary/devices/prefixes for l3 or deviceSummary for files"`
	DeviceName     string            `json:"device_name,omitempty" jsonschema:"Optional device name for device-scoped diffs"`
	InterfaceName  string            `json:"interface_name,omitempty" jsonschema:"Optional interface name when diff_type is interfaces and device_name is set"`
	Prefix         string            `json:"prefix,omitempty" jsonschema:"Optional route prefix when diff_type is l3/routes"`
	CloudObjectID  string            `json:"cloud_object_id,omitempty" jsonschema:"Optional cloud object ID when diff_type is cloud-acl"`
	CheckID        string            `json:"check_id,omitempty" jsonschema:"Optional check ID when diff_type is checks"`
	CheckType      string            `json:"check_type,omitempty" jsonschema:"Optional check type filter for check diffs, such as Predefined, NQE, Existential, Isolation, or Reachability"`
	FileType       string            `json:"file_type,omitempty" jsonschema:"Optional file type for files diffs: CONFIG, STATE, CUSTOM, or ALL"`
	Count          bool              `json:"count,omitempty" jsonschema:"Return count/counts for domains that support count queries"`
	Stats          bool              `json:"stats,omitempty" jsonschema:"Return stats for check diffs when supported"`
	Limit          int               `json:"limit,omitempty" jsonschema:"Optional result limit for paged endpoints"`
	Offset         int               `json:"offset,omitempty" jsonschema:"Optional result offset for paged endpoints"`
	Params         map[string]string `json:"params,omitempty" jsonschema:"Additional query parameters for supported diff endpoints"`
}

type BlastRadiusPagingOptionsArgs struct {
	Offset int `json:"offset,omitempty" jsonschema:"Number of rows to skip"`
	Limit  int `json:"limit,omitempty" jsonschema:"Maximum number of rows to return"`
}

type GetBlastRadiusArgs struct {
	NetworkID          string                        `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID         string                        `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional; uses latest if omitted)"`
	Source             map[string]interface{}        `json:"source,omitempty" jsonschema:"Forward LocationFilter JSON for the source, such as {\"type\":\"DeviceFilter\",\"value\":\"leaf01\"}. If omitted, source_device is used."`
	SourceDevice       string                        `json:"source_device,omitempty" jsonschema:"Device name shorthand for source; converted to a DeviceFilter when source is omitted"`
	DstSubnets         []string                      `json:"dst_subnets" jsonschema:"IPv4 destination subnets to include in the blast radius, such as [\"0.0.0.0/0\"]"`
	ProtocolExclusions []int                         `json:"protocol_exclusions,omitempty" jsonschema:"Optional IP protocol numbers to exclude"`
	TimeoutSeconds     int                           `json:"timeout_seconds,omitempty" jsonschema:"Blast-radius computation timeout in seconds (default: 30, max: 3600)"`
	HostCentric        bool                          `json:"host_centric,omitempty" jsonschema:"If true, request host-centric blast radius rows"`
	PagingOptions      *BlastRadiusPagingOptionsArgs `json:"paging_options,omitempty" jsonschema:"Optional paging for host-centric blast radius output"`
}

type SuggestBlastRadiusSourcesArgs struct {
	NetworkID  string `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID (optional; uses latest if omitted)"`
	Query      string `json:"query" jsonschema:"Text to match against valid blast-radius source locations"`
	Max        int    `json:"max,omitempty" jsonschema:"Maximum suggestions to return (default: 10, max: 100)"`
}

type ListPredefinedChecksArgs struct{}

type ListChecksArgs struct {
	SnapshotID string   `json:"snapshot_id" jsonschema:"Snapshot ID to read checks from"`
	Types      []string `json:"types,omitempty" jsonschema:"Optional check types, such as NQE, Predefined, Existence, Isolation, or Reachability"`
	Priorities []string `json:"priorities,omitempty" jsonschema:"Optional priorities, such as HIGH, MEDIUM, or LOW"`
	Statuses   []string `json:"statuses,omitempty" jsonschema:"Optional statuses, such as PASS, FAIL, WARN, or ERROR"`
}

type GetCheckArgs struct {
	SnapshotID string `json:"snapshot_id" jsonschema:"Snapshot ID to read the check from"`
	CheckID    string `json:"check_id" jsonschema:"Check ID"`
}

type ListL7ApplicationsArgs struct{}

type GetDeviceUtilitiesArgs struct {
	NetworkID  string           `json:"network_id" jsonschema:"ID of the network"`
	SnapshotID string           `json:"snapshot_id,omitempty" jsonschema:"Specific snapshot ID to query (optional)"`
	Options    *NQEQueryOptions `json:"options,omitempty" jsonschema:"Query options including limit, offset, sorting, and filtering"`
}

// Prompt Workflow Arguments
type NQEDiscoveryArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for tracking workflow state"`
}

type NetworkDiscoveryArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for tracking workflow state"`
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
	NetworkIdentifier string `json:"network_identifier" jsonschema:"Network identifier (ID or name) to set as default"`
}

// Semantic Cache and AI Enhancement Args
type GetCacheStatsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type SuggestSimilarQueriesArgs struct {
	Query string `json:"query" jsonschema:"Query text to find similar queries for"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of suggestions to return (default: 5)"`
}

type ClearCacheArgs struct {
	ClearAll bool `json:"clear_all,omitempty" jsonschema:"Clear all cache entries instead of just expired ones"`
}

// AI-Powered Query Discovery Tools

// SearchNQEQueriesArgs represents arguments for intelligent query search
type SearchNQEQueriesArgs struct {
	Query       string `json:"query" jsonschema:"Natural language description of what you want to analyze. Be specific and descriptive. Good examples: 'show me AWS security vulnerabilities', 'find BGP routing issues', 'check interface utilization', 'devices with high CPU usage'. Avoid vague terms like 'network' or 'config'."`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of query suggestions to return (default: 10, max: 50)"`
	Category    string `json:"category,omitempty" jsonschema:"Filter by category to narrow results (e.g., 'Cloud', 'L3', 'Security', 'Device')."`
	Subcategory string `json:"subcategory,omitempty" jsonschema:"Filter by subcategory (e.g., 'AWS', 'BGP', 'ACL', 'OSPF')."`
	IncludeCode bool   `json:"include_code,omitempty" jsonschema:"Include NQE source code in results for advanced users (default: false). Warning: makes response much longer."`
}

// InitializeQueryIndexArgs represents arguments for building the AI query index
type InitializeQueryIndexArgs struct {
	RebuildIndex       bool `json:"rebuild_index,omitempty" jsonschema:"Force rebuild of the query index from spec file (default: false). Only needed if spec file has been updated."`
	GenerateEmbeddings bool `json:"generate_embeddings,omitempty" jsonschema:"Generate new AI embeddings for semantic search (default: false). Requires OpenAI API key and takes several minutes. Creates offline cache for fast searches."`
}

// FindExecutableQueryArgs represents the arguments for finding executable queries
type FindExecutableQueryArgs struct {
	Query          string `json:"query" jsonschema:"Natural language description of what you want to analyze or accomplish. Be specific about the network analysis goal. Examples: 'show me all network devices', 'check device CPU and memory usage', 'find BGP neighbor information', 'compare configuration changes'."`
	Limit          int    `json:"limit,omitempty" jsonschema:"Maximum number of executable query recommendations to return (default: 5, max: 10). Each result includes a real Forward Networks Query ID you can execute."`
	IncludeRelated bool   `json:"include_related,omitempty" jsonschema:"Include the semantic search matches that led to these executable recommendations (default: false). Useful for understanding why these queries were suggested."`
}

// Smart Query Workflow Arguments
type SmartQueryWorkflowArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// Database Hydration Tools Arguments
type HydrateDatabaseArgs struct {
	ForceRefresh         bool `json:"force_refresh,omitempty" jsonschema:"Force refresh all queries from API even if database has data (default: false)"`
	EnhancedMode         bool `json:"enhanced_mode,omitempty" jsonschema:"Use enhanced API mode for metadata enrichment (default: true)"`
	MaxRetries           int  `json:"max_retries,omitempty" jsonschema:"Maximum number of retry attempts for API calls (default: 3)"`
	RegenerateEmbeddings bool `json:"regenerate_embeddings,omitempty" jsonschema:"Automatically regenerate AI embeddings after hydration for improved semantic search (default: false)"`
}

type RefreshQueryIndexArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type GetDatabaseStatusArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

type GetQueryIndexStatsArgs struct {
	Detailed bool `json:"detailed,omitempty" jsonschema:"Include detailed statistics (default: false)"`
}

// Memory Management Tools Arguments
type CreateEntityArgs struct {
	Name     string                 `json:"name" jsonschema:"Name of the entity"`
	Type     string                 `json:"type" jsonschema:"Type of the entity (e.g., 'user', 'network', 'device', 'project')"`
	Metadata map[string]interface{} `json:"metadata,omitempty" jsonschema:"Additional metadata for the entity"`
}

type CreateRelationArgs struct {
	FromID     string                 `json:"from_id" jsonschema:"ID of the source entity"`
	ToID       string                 `json:"to_id" jsonschema:"ID of the target entity"`
	Type       string                 `json:"type" jsonschema:"Type of the relation (e.g., 'owns', 'manages', 'depends_on')"`
	Properties map[string]interface{} `json:"properties,omitempty" jsonschema:"Properties of the relation"`
}

type AddObservationArgs struct {
	EntityID string                 `json:"entity_id" jsonschema:"ID of the entity to add observation to"`
	Content  string                 `json:"content" jsonschema:"Content of the observation"`
	Type     string                 `json:"type" jsonschema:"Type of the observation (e.g., 'note', 'preference', 'behavior')"`
	Metadata map[string]interface{} `json:"metadata,omitempty" jsonschema:"Additional metadata for the observation"`
}

type SearchEntitiesArgs struct {
	Query      string `json:"query,omitempty" jsonschema:"Search query to find entities by name or observation content"`
	EntityType string `json:"entity_type,omitempty" jsonschema:"Filter by entity type"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of results to return (default: 50)"`
}

type GetEntityArgs struct {
	Identifier string `json:"identifier" jsonschema:"Entity ID or name to retrieve"`
}

type GetRelationsArgs struct {
	EntityID     string `json:"entity_id" jsonschema:"ID of the entity to get relations for"`
	RelationType string `json:"relation_type,omitempty" jsonschema:"Filter by relation type"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of relations to return (default: 25, max: 100)"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Number of relations to skip (default: 0)"`
	AllResults   bool   `json:"all_results,omitempty" jsonschema:"If true, fetch all relations using pagination and store in memory system"`
}

type GetObservationsArgs struct {
	EntityID        string `json:"entity_id" jsonschema:"ID of the entity to get observations for"`
	ObservationType string `json:"observation_type,omitempty" jsonschema:"Filter by observation type"`
	Limit           int    `json:"limit,omitempty" jsonschema:"Maximum number of observations to return (default: 25, max: 100)"`
	Offset          int    `json:"offset,omitempty" jsonschema:"Number of observations to skip (default: 0)"`
	AllResults      bool   `json:"all_results,omitempty" jsonschema:"If true, fetch all observations using pagination and store in memory system"`
}

type DeleteEntityArgs struct {
	EntityID string `json:"entity_id" jsonschema:"ID of the entity to delete"`
}

type DeleteRelationArgs struct {
	RelationID string `json:"relation_id" jsonschema:"ID of the relation to delete"`
}

type DeleteObservationArgs struct {
	ObservationID string `json:"observation_id" jsonschema:"ID of the observation to delete"`
}

type GetMemoryStatsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// API Analytics Tools Arguments
type GetQueryAnalyticsArgs struct {
	NetworkID string `json:"network_id" jsonschema:"Network ID to get analytics for"`
}

// Instance Management Tool Arguments
type ListInstanceIDsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
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
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for tracking workflow state"`
}

// Path Search Arguments
type SearchPathsArgs struct {
	NetworkID               string `json:"network_id" jsonschema:"Network ID to search in"`
	SnapshotID              string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID to use (optional, uses latest if omitted)"`
	From                    string `json:"from,omitempty" jsonschema:"Source device name"`
	SrcIP                   string `json:"src_ip,omitempty" jsonschema:"Source IP address or subnet"`
	DstIP                   string `json:"dst_ip" jsonschema:"Destination IP address or subnet"`
	IPProto                 *int   `json:"ip_proto,omitempty" jsonschema:"IP protocol number"`
	SrcPort                 string `json:"src_port,omitempty" jsonschema:"Source port"`
	DstPort                 string `json:"dst_port,omitempty" jsonschema:"Destination port"`
	AppID                   string `json:"app_id,omitempty" jsonschema:"L7 app ID, such as ssh or unidentified"`
	UserID                  string `json:"user_id,omitempty" jsonschema:"L7 user ID or unidentified"`
	UserGroupID             string `json:"user_group_id,omitempty" jsonschema:"L7 user group ID"`
	URL                     string `json:"url,omitempty" jsonschema:"L7 URL with supported wildcards"`
	Domain                  string `json:"domain,omitempty" jsonschema:"DNS domain name with supported wildcards; cannot be used with url"`
	Intent                  string `json:"intent,omitempty" jsonschema:"Search intent (PREFER_DELIVERED, PREFER_VIOLATIONS, VIOLATIONS_ONLY)"`
	MaxCandidates           int    `json:"max_candidates,omitempty" jsonschema:"Maximum number of candidates to consider"`
	MaxResults              int    `json:"max_results,omitempty" jsonschema:"Maximum number of results to return"`
	MaxReturnPathResults    int    `json:"max_return_path_results,omitempty" jsonschema:"Maximum number of return path results"`
	MaxSeconds              int    `json:"max_seconds,omitempty" jsonschema:"Maximum seconds per query"`
	IncludeTags             bool   `json:"include_tags,omitempty" jsonschema:"Include device tags for each hop"`
	IncludeNetworkFunctions bool   `json:"include_network_functions,omitempty" jsonschema:"Include network functions in results"`
}

type GetSnapshotTopologyArgs struct {
	NetworkID    string `json:"network_id,omitempty" jsonschema:"Network ID to use when snapshot_id is omitted or set to latest"`
	SnapshotID   string `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID to fetch topology for; omit or set to latest to use the latest processed snapshot for network_id"`
	DeviceFilter string `json:"device_filter,omitempty" jsonschema:"Case-insensitive substring filter applied to source or target port"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Zero-based link offset after filtering"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum links to return (default 200, max 1000)"`
}

// Path Search Workflow Arguments
type PathSearchWorkflowArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for tracking workflow state"`
}

// Bloom Search Arguments
type BuildBloomFilterArgs struct {
	NetworkID  string `json:"network_id,omitempty" jsonschema:"Network ID to build filter for (optional if a default network is set)"`
	FilterType string `json:"filter_type" jsonschema:"Type of filter to build (device, interface, config)"`
	QueryID    string `json:"query_id" jsonschema:"NQE query ID to use for building the filter"`
	ChunkSize  int    `json:"chunk_size,omitempty" jsonschema:"Chunk size for processing (default: 200)"`
}

type SearchBloomFilterArgs struct {
	NetworkID   string   `json:"network_id,omitempty" jsonschema:"Network ID to search in (optional if a default network is set)"`
	FilterType  string   `json:"filter_type" jsonschema:"Type of filter to search (device, interface, config)"`
	SearchTerms []string `json:"search_terms" jsonschema:"Search terms to look for"`
	EntityID    string   `json:"entity_id" jsonschema:"Entity ID containing the full dataset to search"`
}

type GetBloomFilterStatsArgs struct {
	// No parameters needed - MCP handles empty structs correctly
}

// Network Prefix Discovery and Connectivity Analysis
type NetworkPrefixDiscoveryArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for tracking workflow state"`
	Step      string `json:"step,omitempty" jsonschema:"Current step in the workflow"`
}

type NetworkPrefixAnalysisArgs struct {
	NetworkID    string   `json:"network_id,omitempty" jsonschema:"Network ID to analyze (optional if a default network is set)"`
	SnapshotID   string   `json:"snapshot_id,omitempty" jsonschema:"Snapshot ID to use (optional, uses latest if omitted)"`
	PrefixLevels []string `json:"prefix_levels,omitempty" jsonschema:"Aggregation levels to analyze (e.g., ['/8', '/16', '/24'])"`
	FromDevices  []string `json:"from_devices,omitempty" jsonschema:"Source devices to analyze"`
	ToDevices    []string `json:"to_devices,omitempty" jsonschema:"Destination devices to analyze"`
	Intent       string   `json:"intent,omitempty" jsonschema:"Search intent (PREFER_DELIVERED, PREFER_VIOLATIONS, VIOLATIONS_ONLY)"`
	MaxResults   int      `json:"max_results,omitempty" jsonschema:"Maximum number of results to return"`
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
