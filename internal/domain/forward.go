// Package domain holds the values forward-mcp reasons about: Forward Networks
// API resources, NQE queries, and configuration. It imports nothing that does I/O.
package domain

// Network types
type Network struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"note,omitempty"` // API field is "note"
	CreatedAt   string `json:"createdAt,omitempty"`
	OrgID       string `json:"orgId,omitempty"`
	CreatorID   string `json:"creatorId,omitempty"`
	Creator     string `json:"creator,omitempty"`
	ParentID    string `json:"parentId,omitempty"`
}

type NetworkUpdate struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"note,omitempty"` // API field is "note"
}

// Path Search types
type PathSearchParams struct {
	From                    string `json:"from,omitempty"`
	SrcIP                   string `json:"srcIp,omitempty"`
	DstIP                   string `json:"dstIp"`
	Intent                  string `json:"intent,omitempty"`
	IPProto                 *int   `json:"ipProto,omitempty"`
	SrcPort                 string `json:"srcPort,omitempty"`
	DstPort                 string `json:"dstPort,omitempty"`
	IncludeNetworkFunctions bool   `json:"includeNetworkFunctions,omitempty"`
	MaxCandidates           int    `json:"maxCandidates,omitempty"`
	MaxResults              int    `json:"maxResults,omitempty"`
	MaxReturnPathResults    int    `json:"maxReturnPathResults,omitempty"`
	MaxSeconds              int    `json:"maxSeconds,omitempty"`
	SnapshotID              string `json:"snapshotId,omitempty"`
}

// PathSearchBulkRequest represents the request body for bulk path search
type PathSearchBulkRequest struct {
	Queries                 []PathSearchParams `json:"queries"`
	Intent                  string             `json:"intent,omitempty"`
	MaxCandidates           int                `json:"maxCandidates,omitempty"`
	MaxResults              int                `json:"maxResults,omitempty"`
	MaxReturnPathResults    int                `json:"maxReturnPathResults,omitempty"`
	MaxSeconds              int                `json:"maxSeconds,omitempty"`
	MaxOverallSeconds       int                `json:"maxOverallSeconds,omitempty"`
	IncludeNetworkFunctions bool               `json:"includeNetworkFunctions,omitempty"`
}

// PathSearchResponse represents the response from single path search
type PathSearchResponse struct {
	Paths              []Path                 `json:"paths"`
	ReturnPaths        []Path                 `json:"returnPaths,omitempty"`
	UnrecognizedValues map[string]interface{} `json:"unrecognizedValues,omitempty"`
	SnapshotID         string                 `json:"snapshotId"`
	SearchTimeMs       int                    `json:"searchTimeMs"`
	NumCandidatesFound int                    `json:"numCandidatesFound"`
}

// PathSearchBulkResponse represents the response from bulk path search
// (the spec's PathSearchResponse schema, shared by single and bulk searches)
type PathSearchBulkResponse struct {
	SrcIpLocationType  string                 `json:"srcIpLocationType,omitempty"`
	DstIpLocationType  string                 `json:"dstIpLocationType"`
	Info               PathSearchInfo         `json:"info"`
	ReturnPathInfo     PathSearchInfo         `json:"returnPathInfo"`
	TimedOut           bool                   `json:"timedOut"`
	QueryUrl           string                 `json:"queryUrl"`
	UnrecognizedValues map[string]interface{} `json:"unrecognizedValues,omitempty"`
}

type PathSearchInfo struct {
	Paths     []BulkPath `json:"paths"`
	TotalHits TotalHits  `json:"totalHits"`
}

type TotalHits struct {
	Value int    `json:"value"`
	Type  string `json:"type"`
}

type BulkPath struct {
	ForwardingOutcome string    `json:"forwardingOutcome"`
	SecurityOutcome   string    `json:"securityOutcome"`
	Hops              []BulkHop `json:"hops"`
}

type BulkHop struct {
	DeviceName       string   `json:"deviceName"`
	DeviceType       string   `json:"deviceType"`
	IngressInterface string   `json:"ingressInterface"`
	EgressInterface  string   `json:"egressInterface"`
	Behaviors        []string `json:"behaviors"`
}

// Legacy types for backward compatibility with single path search
type Path struct {
	Hops        []Hop  `json:"hops"`
	Outcome     string `json:"outcome"`
	OutcomeType string `json:"outcomeType"`
}

type Hop struct {
	Device    string                 `json:"device"`
	Interface string                 `json:"interface,omitempty"`
	Action    string                 `json:"action"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// NQE types
type NQEQueryParams struct {
	NetworkID          string                 `json:"networkId,omitempty"`
	SnapshotID         string                 `json:"snapshotId,omitempty"`
	Query              string                 `json:"query,omitempty"`
	QueryID            string                 `json:"queryId,omitempty"`
	CommitID           string                 `json:"commitId,omitempty"`
	UseLatestDataFiles bool                   `json:"useLatestDataFiles,omitempty"`
	Options            *NQEQueryOptions       `json:"queryOptions,omitempty"`
	Parameters         map[string]interface{} `json:"parameters,omitempty"`
}

type NQEQueryOptions struct {
	Offset  int               `json:"offset,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	SortBy  *NQESortBy        `json:"sortBy,omitempty"` // API accepts a single sort order, not a list
	Filters []NQEColumnFilter `json:"columnFilters,omitempty"`
	Format  string            `json:"itemFormat,omitempty"` // API field is "itemFormat": JSON (default) or LEGACY (deprecated)
}

type NQESortBy struct {
	ColumnName string `json:"columnName"`
	Order      string `json:"order"` // "ASC" or "DESC"
}

type NQEColumnFilter struct {
	ColumnName string `json:"columnName"`
	Value      string `json:"value"`
}

type NQERunResult struct {
	SnapshotID    string                   `json:"snapshotId"`
	Items         []map[string]interface{} `json:"items"`
	TotalNumItems int64                    `json:"totalNumItems,omitempty"`
}

type NQEQuery struct {
	QueryID    string `json:"queryId"`
	Path       string `json:"path"`
	Intent     string `json:"intent"`
	Repository string `json:"repository"`
}

// NQEOrgQuerySummary represents a query summary from the org repository
type NQEOrgQuerySummary struct {
	Path          string `json:"path"`
	LastCommitId  string `json:"lastCommitId"`
	QueryID       string `json:"queryId"`
	SourceCodeSha string `json:"sourceCodeSha"`
}

// NQEOrgQueriesResponse represents the response from /api/nqe/repos/org/commits/head/queries
type NQEOrgQueriesResponse struct {
	Queries        []NQEOrgQuerySummary `json:"queries"`
	AccessSettings []interface{}        `json:"accessSettings"`
}

// NQECommitInfo represents commit information
type NQECommitInfo struct {
	ID          string `json:"id"`
	AuthorEmail string `json:"authorEmail"`
	CommittedAt int64  `json:"committedAt"`
	Title       string `json:"title"`
	Body        string `json:"body"`
}

// NQEQueryDetail represents detailed query information from commit endpoint
type NQEQueryDetail struct {
	QueryID       string        `json:"queryId"`
	Path          string        `json:"path"` // Added from org queries response
	SourceCode    string        `json:"sourceCode"`
	Intent        string        `json:"intent"`
	Description   string        `json:"description"`
	SourceCodeSha string        `json:"sourceCodeSha"`
	CommitCount   int           `json:"commitCount"`
	LastCommit    NQECommitInfo `json:"lastCommit"`
	FirstCommit   NQECommitInfo `json:"firstCommit"`
	Repository    string        `json:"repository"` // Added to track repository source
}

type NQEDiffRequest struct {
	QueryID    string                 `json:"queryId"`
	CommitID   string                 `json:"commitId,omitempty"`
	Options    *NQEQueryOptions       `json:"options,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type NQEDiffResult struct {
	TotalNumRows int                      `json:"totalNumRows"`
	Rows         []map[string]interface{} `json:"rows"`
}

// Device types
type DeviceQueryParams struct {
	SnapshotID string `json:"snapshotId,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type DeviceResponse struct {
	Devices    []Device `json:"devices"`
	TotalCount int      `json:"totalCount"`
}

type Device struct {
	Name            string   `json:"name"`
	DisplayName     string   `json:"displayName,omitempty"`
	SourceName      string   `json:"sourceName,omitempty"`
	Type            string   `json:"type,omitempty"`
	Vendor          string   `json:"vendor,omitempty"`
	OSVersion       string   `json:"osVersion,omitempty"`
	Platform        string   `json:"platform,omitempty"`
	Model           string   `json:"model,omitempty"`
	ManagementIPs   []string `json:"managementIps,omitempty"`
	CollectionError string   `json:"collectionError,omitempty"`
	ProcessingError string   `json:"processingError,omitempty"`
	Tags            []string `json:"tags,omitempty"`       // present only if requested via "with"
	LocationID      string   `json:"locationId,omitempty"` // present only if requested via "with"

	// The fields below are not part of the public API response; they are
	// populated internally (e.g. from NQE queries) by service-layer code.
	Hostname     string                 `json:"hostname,omitempty"`
	Version      string                 `json:"version,omitempty"`
	SerialNumber string                 `json:"serialNumber,omitempty"`
	Interfaces   []DeviceInterface      `json:"interfaces,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

type DeviceInterface struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IPAddress   string `json:"ipAddress,omitempty"`
	Status      string `json:"status,omitempty"`
	Type        string `json:"type,omitempty"`
}

// Snapshot types
type Snapshot struct {
	ID                string `json:"id"`
	ProcessingTrigger string `json:"processingTrigger,omitempty"`
	TotalDevices      int    `json:"totalDevices,omitempty"`
	TotalEndpoints    int    `json:"totalEndpoints,omitempty"`
	TotalOtherSources int    `json:"totalOtherSources,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`   // RFC3339 timestamp
	ProcessedAt       string `json:"processedAt,omitempty"` // RFC3339 timestamp
	IsDraft           bool   `json:"isDraft,omitempty"`
	State             string `json:"state,omitempty"`
	Note              string `json:"note,omitempty"`
	ParentSnapshotID  string `json:"parentSnapshotId,omitempty"`
	// Legacy fields for backward compatibility
	NetworkID   string `json:"networkId,omitempty"`
	Name        string `json:"name,omitempty"`
	Status      string `json:"status,omitempty"`
	DeviceCount int    `json:"deviceCount,omitempty"`
}

// Response wrapper for snapshots API
type SnapshotsResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Creator   string     `json:"creator"`
	CreatedAt string     `json:"createdAt,omitempty"`
	OrgID     string     `json:"orgId"`
	CreatorID string     `json:"creatorId"`
	Snapshots []Snapshot `json:"snapshots"`
}

// Location types
type Location struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	City          string  `json:"city,omitempty"`
	AdminDivision string  `json:"adminDivision,omitempty"`
	Country       string  `json:"country,omitempty"`
}

type LocationCreate struct {
	ID            string  `json:"id,omitempty"`
	Name          string  `json:"name"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	City          string  `json:"city,omitempty"`
	AdminDivision string  `json:"adminDivision,omitempty"`
	Country       string  `json:"country,omitempty"`
}

type LocationUpdate struct {
	ID            *string  `json:"id,omitempty"`
	Name          *string  `json:"name,omitempty"`
	Lat           *float64 `json:"lat,omitempty"`
	Lng           *float64 `json:"lng,omitempty"`
	City          *string  `json:"city,omitempty"`
	AdminDivision *string  `json:"adminDivision,omitempty"`
	Country       *string  `json:"country,omitempty"`
}

type LocationBulkPatch struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name,omitempty"`
	Lat           *float64 `json:"lat,omitempty"`
	Lng           *float64 `json:"lng,omitempty"`
	City          string   `json:"city,omitempty"`
	AdminDivision string   `json:"adminDivision,omitempty"`
	Country       string   `json:"country,omitempty"`
}
