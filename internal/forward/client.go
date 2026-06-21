package forward

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/logger"
)

// ClientInterface defines the interface for Forward platform client operations
type ClientInterface interface {
	// Legacy chat operations (keeping for backward compatibility)
	SendChatRequest(req *ChatRequest) (*ChatResponse, error)
	GetAvailableModels() ([]string, error)

	// Network operations
	GetNetworks() ([]Network, error)
	CreateNetwork(name string) (*Network, error)
	DeleteNetwork(networkID string) (*Network, error)
	UpdateNetwork(networkID string, update *NetworkUpdate) (*Network, error)

	// Path Search operations
	GetL7Applications() ([]map[string]interface{}, error)
	SearchPaths(networkID string, params *PathSearchParams) (*PathSearchResponse, error)
	SearchPathsBulk(networkID string, request *PathSearchBulkRequest, snapshotID string) ([]PathSearchBulkResponse, error)
	GetTopology(snapshotID string) ([]TopologyLink, error)

	// NQE operations
	RunNQEQueryByString(params *NQEQueryParams) (*NQERunResult, error)
	RunNQEQueryByID(params *NQEQueryParams) (*NQERunResult, error)
	StartNQEExecution(networkID, snapshotID string, request *NQEExecutionRequest) (*NQEExecutionResponse, error)
	GetNQEExecutionStatus(networkID, executionKey string) (*NQEExecutionStatus, error)
	GetNQEExecutionResult(networkID, executionKey string, offset, limit int) (*NQERunResult, error)
	GetNQEQueries(dir string) ([]NQEQuery, error)
	GetNQEOrgQueries() ([]NQEQuery, error)
	GetNQEOrgQueriesEnhanced() ([]NQEQueryDetail, error)
	GetNQEOrgQueriesEnhancedWithCache(existingCommitIDs map[string]string) ([]NQEQueryDetail, error)
	GetNQEOrgQueriesEnhancedWithCacheContext(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error)
	GetNQEFwdQueries() ([]NQEQuery, error)
	GetNQEFwdQueriesEnhanced() ([]NQEQueryDetail, error)
	GetNQEFwdQueriesEnhancedWithCache(existingCommitIDs map[string]string) ([]NQEQueryDetail, error)
	GetNQEFwdQueriesEnhancedWithCacheContext(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error)
	GetNQEAllQueriesEnhanced() ([]NQEQueryDetail, error)
	GetNQEAllQueriesEnhancedWithCache(existingCommitIDs map[string]string) ([]NQEQueryDetail, error)
	GetNQEAllQueriesEnhancedWithCacheContext(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error)
	GetNQEQueryByCommit(commitID string, path string, repository string) (*NQEQueryDetail, error)
	GetNQEQueryByCommitWithContext(ctx context.Context, commitID string, path string, repository string) (*NQEQueryDetail, error)
	DiffNQEQuery(before, after string, request *NQEDiffRequest) (*NQEDiffResult, error)
	GetSnapshotDiff(before, after, path string, query url.Values) (map[string]interface{}, error)
	GetBlastRadius(networkID, snapshotID string, request *BlastRadiusRequest, hostCentric bool) (map[string]interface{}, error)
	SuggestBlastRadiusSources(networkID, snapshotID, query string, max int) ([]map[string]interface{}, error)

	// Device operations
	GetDevices(networkID string, params *DeviceQueryParams) (*DeviceResponse, error)
	GetMissingDevices(networkID, snapshotID string) (map[string]interface{}, error)
	GetDeviceLocations(networkID string) (map[string]string, error)
	UpdateDeviceLocations(networkID string, locations map[string]string) error
	GetClassicDevices(networkID string, include []string) (map[string]interface{}, error)
	GetClassicDevice(networkID string, deviceName string, include []string) (map[string]interface{}, error)
	UpsertClassicDevices(networkID string, devices []map[string]interface{}) error
	DeleteClassicDevices(networkID string, names []string) error
	GetCliCredentials(networkID string) ([]map[string]interface{}, error)
	CreateCliCredential(networkID string, credential map[string]interface{}) (map[string]interface{}, error)
	DeleteCliCredential(networkID string, credentialID string) error
	GetSnmpCredentials(networkID string) ([]map[string]interface{}, error)
	CreateSnmpCredential(networkID string, credential map[string]interface{}) (map[string]interface{}, error)
	DeleteSnmpCredential(networkID string, credentialID string) error
	GetPerformanceSettings(networkID string) (map[string]interface{}, error)
	UpdatePerformanceSettings(networkID string, settings map[string]interface{}) (map[string]interface{}, error)
	GetDevicesWithPerformanceData(networkID string, query url.Values) ([]string, error)
	GetDevicePerformance(networkID, deviceName string, query url.Values) (map[string]interface{}, error)
	GetInterfacePerformance(networkID, deviceName, interfaceName string, query url.Values) (map[string]interface{}, error)

	// Collection operations
	GetCollectorStatus(networkID string) (map[string]interface{}, error)
	AddCollectorTask(networkID string) (map[string]interface{}, error)
	GetCollectorTask(taskID string) (map[string]interface{}, error)
	StartCollection(networkID string) (map[string]interface{}, error)
	CancelCollection(networkID string) (map[string]interface{}, error)
	GetCollectionSchedules(networkID string) (map[string]interface{}, error)

	// Snapshot operations
	GetSnapshots(networkID string) ([]Snapshot, error)
	GetLatestSnapshot(networkID string) (*Snapshot, error)
	DeleteSnapshot(snapshotID string) error

	// Location operations
	GetLocations(networkID string) ([]Location, error)
	CreateLocation(networkID string, location *LocationCreate) (*Location, error)
	CreateLocationsBulk(networkID string, locations []LocationBulkPatch) error
	UpdateLocation(networkID string, locationID string, update *LocationUpdate) (*Location, error)
	DeleteLocation(networkID string, locationID string) (*Location, error)

	// Checks and security operations
	GetAvailablePredefinedChecks() ([]map[string]interface{}, error)
	GetChecks(snapshotID string, query url.Values) ([]map[string]interface{}, error)
	GetCheck(snapshotID, checkID string) (map[string]interface{}, error)
}

// Client represents the Forward platform client
type Client struct {
	httpClient *http.Client
	config     *config.ForwardConfig
}

// NewClient creates a new Forward platform client
func NewClient(config *config.ForwardConfig) ClientInterface {
	// Create TLS configuration with strong security settings
	tlsConfig := &tls.Config{
		// SECURITY: Enforce TLS 1.3 minimum version
		MinVersion: tls.VersionTLS13,
		// SECURITY: Use only secure cipher suites
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	// Load custom CA certificate if provided
	if config.CACertPath != "" {
		caCert, err := os.ReadFile(config.CACertPath)
		if err == nil {
			caCertPool := x509.NewCertPool()
			if caCertPool.AppendCertsFromPEM(caCert) {
				tlsConfig.RootCAs = caCertPool
			}
		}
	}

	// Load client certificate and key if provided
	if config.ClientCertPath != "" && config.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(config.ClientCertPath, config.ClientKeyPath)
		if err == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	// Create custom transport with TLS configuration
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   time.Duration(config.Timeout) * time.Second,
			Transport: transport,
		},
		config: config,
	}
}

// makeSecureAuthHeader creates a Basic Auth header and zeros sensitive data from memory
// SECURITY: This function ensures credentials don't persist in memory after use
func (c *Client) makeSecureAuthHeader() (string, error) {
	// Use byte slice for credentials to enable zeroing
	credentials := make([]byte, len(c.config.APIKey)+len(c.config.APISecret)+1)
	defer func() {
		// SECURITY: Zero sensitive data from memory after use
		for i := range credentials {
			credentials[i] = 0
		}
	}()

	// Build credentials: "key:secret"
	copy(credentials, c.config.APIKey)
	credentials[len(c.config.APIKey)] = ':'
	copy(credentials[len(c.config.APIKey)+1:], c.config.APISecret)

	// Encode to base64
	auth := base64.StdEncoding.EncodeToString(credentials)
	return "Basic " + auth, nil
}

// Legacy types for backward compatibility
type ChatRequest struct {
	Messages []map[string]string `json:"messages"`
	Model    string              `json:"model"`
}

type ChatResponse struct {
	Response string `json:"response"`
	Model    string `json:"model"`
}

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
	AppID                   string `json:"appId,omitempty"`
	UserID                  string `json:"userId,omitempty"`
	UserGroupID             string `json:"userGroupId,omitempty"`
	URL                     string `json:"url,omitempty"`
	Domain                  string `json:"domain,omitempty"`
	IncludeTags             bool   `json:"includeTags,omitempty"`
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
	IncludeTags             bool               `json:"includeTags,omitempty"`
	IncludeNetworkFunctions bool               `json:"includeNetworkFunctions,omitempty"`
}

type TopologyLink struct {
	SourcePort string `json:"sourcePort"`
	TargetPort string `json:"targetPort"`
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
	NetworkID  string                 `json:"networkId,omitempty"`
	SnapshotID string                 `json:"snapshotId,omitempty"`
	Query      string                 `json:"query,omitempty"`
	QueryID    string                 `json:"queryId,omitempty"`
	CommitID   string                 `json:"commitId,omitempty"`
	Options    *NQEQueryOptions       `json:"queryOptions,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
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

type NQEExecutionColumnFilter struct {
	ColumnName string `json:"columnName"`
	Operator   string `json:"operator"`
	Value      string `json:"value,omitempty"`
	LowerBound string `json:"lowerBound,omitempty"`
	UpperBound string `json:"upperBound,omitempty"`
}

type NQERunResult struct {
	SnapshotID    string                   `json:"snapshotId"`
	Items         []map[string]interface{} `json:"items"`
	TotalNumItems int64                    `json:"totalNumItems,omitempty"`
}

type NQEExecutionRequest struct {
	Query         string                     `json:"query,omitempty"`
	QueryID       string                     `json:"queryId,omitempty"`
	CommitID      string                     `json:"commitId,omitempty"`
	Parameters    map[string]interface{}     `json:"parameters,omitempty"`
	ColumnFilters []NQEExecutionColumnFilter `json:"columnFilters,omitempty"`
	SortKeys      []NQESortBy                `json:"sortKeys,omitempty"`
}

type NQEExecutionStatus struct {
	Status          string                 `json:"status"`
	Outcome         string                 `json:"outcome,omitempty"`
	MillisExecuting int64                  `json:"millisExecuting,omitempty"`
	RowsProduced    int64                  `json:"rowsProduced,omitempty"`
	TimeoutMinutes  int64                  `json:"timeoutMinutes,omitempty"`
	Error           map[string]interface{} `json:"error,omitempty"`
}

type NQEExecutionResponse struct {
	ExecutionKey string `json:"executionKey"`
	NQEExecutionStatus
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

type BlastRadiusPagingOptions struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

type BlastRadiusRequest struct {
	Source             map[string]interface{}    `json:"source"`
	DstSubnets         []string                  `json:"dstSubnets"`
	ProtocolExclusions []int                     `json:"protocolExclusions,omitempty"`
	TimeoutSecs        int                       `json:"timeoutSecs,omitempty"`
	PagingOptions      *BlastRadiusPagingOptions `json:"pagingOptions,omitempty"`
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

// Helper method to make authenticated requests
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.config.APIBaseURL+endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// SECURITY FIX: Zero sensitive credentials from memory after use
	authHeader, err := c.makeSecureAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to create auth header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Provide LLM-friendly error messages based on error type
		errStr := err.Error()
		if strings.Contains(errStr, "connection refused") {
			return nil, fmt.Errorf("cannot connect to Forward Networks API at %s. Please verify FORWARD_API_BASE_URL is correct and the API is accessible: %w", c.config.APIBaseURL, err)
		}
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
			return nil, fmt.Errorf("API request timed out. The Forward Networks API may be slow or overloaded. Try again in a moment: %w", err)
		}
		if strings.Contains(errStr, "no such host") {
			return nil, fmt.Errorf("cannot resolve API hostname %s. Please check FORWARD_API_BASE_URL and network connectivity: %w", c.config.APIBaseURL, err)
		}
		return nil, fmt.Errorf("failed to send request to Forward Networks API: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the response body for error details
		errorBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Provide LLM-friendly error messages based on status code
		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("authentication failed (HTTP 401): please verify FORWARD_API_KEY and FORWARD_API_SECRET environment variables are correct. These credentials are required to access the Forward Networks API")
		case 403:
			return nil, fmt.Errorf("access forbidden (HTTP 403): your API credentials are valid but lack permission to access this resource. Please check your Forward Networks account permissions")
		case 404:
			return nil, fmt.Errorf("resource not found (HTTP 404): the requested endpoint %s does not exist. Please verify the network ID, snapshot ID, or resource ID is correct", endpoint)
		case 429:
			return nil, fmt.Errorf("rate limit exceeded (HTTP 429): too many requests to Forward Networks API. Please wait before retrying")
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("Forward Networks API server error (HTTP %d): the API is experiencing issues. Please try again later or contact Forward Networks support", resp.StatusCode)
		}

		// For other errors, provide the original error with context
		errorMsg := fmt.Sprintf("API request failed with HTTP %d", resp.StatusCode)
		if readErr == nil && len(errorBody) > 0 {
			// Log full response server-side for debugging, but don't expose it to LLM
			debugLogger := logger.New()
			debugLogger.Debug("API Error Response: Status=%d, Endpoint=%s, Body=%s", resp.StatusCode, endpoint, string(errorBody))

			// Provide sanitized message to LLM
			errorMsg += ". Check server logs for details"
		}

		// Log additional debugging information for 400 errors
		// Security: Do not log request bodies as they may contain sensitive data
		if resp.StatusCode == 400 {
			debugLogger := logger.New()
			debugLogger.Debug("400 Bad Request - URL: %s%s, Method: %s, Body Size: %d bytes",
				c.config.APIBaseURL, endpoint, method, len(reqBody))
			return nil, fmt.Errorf("bad request (HTTP 400): the API rejected the request parameters. Please verify all required fields are provided and have valid values")
		}

		return nil, fmt.Errorf("%s", errorMsg)
	}

	return resp, nil
}

// Legacy methods for backward compatibility
func (c *Client) SendChatRequest(req *ChatRequest) (*ChatResponse, error) {
	resp, err := c.makeRequest("POST", "/chat", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// retryWithBackoff performs an operation with exponential backoff
func (c *Client) retryWithBackoff(ctx context.Context, operation func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt == maxRetries {
			break
		}

		// Calculate exponential backoff delay
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		if delay > 60*time.Second {
			delay = 60 * time.Second // Cap at 60 seconds
		}

		// Log retry attempt
		if debugLogger := logger.New(); debugLogger != nil {
			debugLogger.Info("🔄 Retrying operation in %v (attempt %d/%d): %v", delay, attempt+1, maxRetries, err)
		}

		// Wait before retry with context cancellation check
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// makeRequestWithRetry makes an HTTP request with retry logic and exponential backoff
func (c *Client) makeRequestWithRetry(ctx context.Context, method, endpoint string, body interface{}, maxRetries int) (*http.Response, error) {
	var response *http.Response
	var reqBody []byte
	var err error

	// Prepare request body once
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	operation := func() error {
		req, err := http.NewRequestWithContext(ctx, method, c.config.APIBaseURL+endpoint, bytes.NewBuffer(reqBody))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		auth := base64.StdEncoding.EncodeToString([]byte(c.config.APIKey + ":" + c.config.APISecret))
		req.Header.Set("Authorization", "Basic "+auth)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Read the response body for error details
			errorBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			errorMsg := fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
			if readErr == nil && len(errorBody) > 0 {
				errorMsg += fmt.Sprintf(", response: %s", string(errorBody))
			}

			// Retry on 5xx errors and 429 (rate limiting)
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				return fmt.Errorf("retryable error: %s", errorMsg)
			}

			// Don't retry on 4xx errors (except 429)
			return fmt.Errorf("non-retryable error: %s", errorMsg)
		}

		response = resp
		return nil
	}

	err = c.retryWithBackoff(ctx, operation, maxRetries, 1*time.Second)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) GetAvailableModels() ([]string, error) {
	resp, err := c.makeRequest("GET", "/models", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var models []string
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return models, nil
}

// Network operations
func (c *Client) GetNetworks() ([]Network, error) {
	resp, err := c.makeRequest("GET", "/api/networks", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var networks []Network
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return networks, nil
}

func (c *Client) CreateNetwork(name string) (*Network, error) {
	resp, err := c.makeRequest("POST", fmt.Sprintf("/api/networks?name=%s", url.QueryEscape(name)), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var network Network
	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &network, nil
}

func (c *Client) DeleteNetwork(networkID string) (*Network, error) {
	resp, err := c.makeRequest("DELETE", fmt.Sprintf("/api/networks/%s", networkID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var network Network
	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &network, nil
}

func (c *Client) UpdateNetwork(networkID string, update *NetworkUpdate) (*Network, error) {
	resp, err := c.makeRequest("PATCH", fmt.Sprintf("/api/networks/%s", networkID), update)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var network Network
	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &network, nil
}

// Path Search operations
func (c *Client) SearchPaths(networkID string, params *PathSearchParams) (*PathSearchResponse, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/paths", networkID)

	// Build query parameters
	query := url.Values{}
	query.Set("dstIp", params.DstIP)
	if params.From != "" {
		query.Set("from", params.From)
	}
	if params.SrcIP != "" {
		query.Set("srcIp", params.SrcIP)
	}
	if params.Intent != "" {
		query.Set("intent", params.Intent)
	}
	if params.IPProto != nil {
		query.Set("ipProto", fmt.Sprintf("%d", *params.IPProto))
	}
	if params.SrcPort != "" {
		query.Set("srcPort", params.SrcPort)
	}
	if params.DstPort != "" {
		query.Set("dstPort", params.DstPort)
	}
	if params.AppID != "" {
		query.Set("appId", params.AppID)
	}
	if params.UserID != "" {
		query.Set("userId", params.UserID)
	}
	if params.UserGroupID != "" {
		query.Set("userGroupId", params.UserGroupID)
	}
	if params.URL != "" {
		query.Set("url", params.URL)
	}
	if params.Domain != "" {
		query.Set("domain", params.Domain)
	}
	if params.IncludeTags {
		query.Set("includeTags", "true")
	}
	if params.IncludeNetworkFunctions {
		query.Set("includeNetworkFunctions", "true")
	}
	if params.MaxCandidates > 0 {
		query.Set("maxCandidates", fmt.Sprintf("%d", params.MaxCandidates))
	}
	if params.MaxResults > 0 {
		query.Set("maxResults", fmt.Sprintf("%d", params.MaxResults))
	}
	if params.MaxReturnPathResults > 0 {
		query.Set("maxReturnPathResults", fmt.Sprintf("%d", params.MaxReturnPathResults))
	}
	if params.MaxSeconds > 0 {
		query.Set("maxSeconds", fmt.Sprintf("%d", params.MaxSeconds))
	}
	if params.SnapshotID != "" {
		query.Set("snapshotId", params.SnapshotID)
	}

	resp, err := c.makeRequest("GET", appendQuery(endpoint, query), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The API returns the same PathSearchResponse schema as the bulk endpoint
	// ({info, returnPathInfo, timedOut, queryUrl, ...}); decode that shape and
	// map it onto the legacy struct this method exposes.
	var apiResp PathSearchBulkResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	pathResp := &PathSearchResponse{
		Paths:              convertSpecPaths(apiResp.Info.Paths),
		ReturnPaths:        convertSpecPaths(apiResp.ReturnPathInfo.Paths),
		UnrecognizedValues: apiResp.UnrecognizedValues,
		SnapshotID:         params.SnapshotID,
	}
	return pathResp, nil
}

// convertSpecPaths maps the API's path schema onto the legacy Path/Hop types.
func convertSpecPaths(paths []BulkPath) []Path {
	converted := make([]Path, len(paths))
	for i, p := range paths {
		hops := make([]Hop, len(p.Hops))
		for j, h := range p.Hops {
			hops[j] = Hop{
				Device:    h.DeviceName,
				Interface: h.IngressInterface,
				Action:    h.DeviceType,
			}
		}
		converted[i] = Path{
			Hops:        hops,
			Outcome:     p.ForwardingOutcome,
			OutcomeType: p.SecurityOutcome,
		}
	}
	return converted
}

func (c *Client) SearchPathsBulk(networkID string, request *PathSearchBulkRequest, snapshotID string) ([]PathSearchBulkResponse, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/paths-bulk", networkID)

	// Add snapshotId as query parameter if provided (optional for bulk API)
	if snapshotID != "" && snapshotID != "latest" {
		endpoint += fmt.Sprintf("?snapshotId=%s", snapshotID)
	}

	// Debug logging
	debugLogger := logger.New()
	debugLogger.Debug("SearchPathsBulk - URL: %s, snapshotID: %s", endpoint, snapshotID)

	resp, err := c.makeRequest("POST", endpoint, request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responses []PathSearchBulkResponse
	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Debug logging
	bulkLogger := logger.New()
	bulkLogger.Debug("SearchPathsBulk decoded %d responses", len(responses))
	if len(responses) > 0 {
		bulkLogger.Debug("First response: %+v", responses[0])
	}

	return responses, nil
}

func (c *Client) GetTopology(snapshotID string) ([]TopologyLink, error) {
	endpoint := fmt.Sprintf("/api/snapshots/%s/topology", url.PathEscape(snapshotID))

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var links []TopologyLink
	if err := json.NewDecoder(resp.Body).Decode(&links); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return links, nil
}

// NQE operations
func (c *Client) RunNQEQueryByString(params *NQEQueryParams) (*NQERunResult, error) {
	endpoint := "/api/nqe"

	// Build query parameters
	query := ""
	if params.NetworkID != "" {
		query += fmt.Sprintf("?networkId=%s", params.NetworkID)
	}
	if params.SnapshotID != "" {
		if query == "" {
			query += "?"
		} else {
			query += "&"
		}
		query += fmt.Sprintf("snapshotId=%s", params.SnapshotID)
	}

	// For string-based queries, format the request body properly
	requestBody := map[string]interface{}{
		"query": params.Query,
	}
	if params.Parameters != nil {
		requestBody["parameters"] = params.Parameters
	}
	if params.Options != nil {
		requestBody["queryOptions"] = params.Options
	}

	// Debug logging
	debugLogger := logger.New()
	if requestBodyJSON, err := json.Marshal(requestBody); err == nil {
		debugLogger.Debug("NQE String Query Request - URL: %s%s, Body: %s", endpoint, query, string(requestBodyJSON))
	}

	resp, err := c.makeRequest("POST", endpoint+query, requestBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NQERunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) RunNQEQueryByID(params *NQEQueryParams) (*NQERunResult, error) {
	endpoint := "/api/nqe"

	// Build query parameters
	query := ""
	if params.NetworkID != "" {
		query += fmt.Sprintf("?networkId=%s", params.NetworkID)
	}
	if params.SnapshotID != "" {
		if query == "" {
			query += "?"
		} else {
			query += "&"
		}
		query += fmt.Sprintf("snapshotId=%s", params.SnapshotID)
	}

	// For query ID based execution, we only need to send the query ID and parameters
	requestBody := map[string]interface{}{
		"queryId": params.QueryID,
	}
	if params.Parameters != nil {
		requestBody["parameters"] = params.Parameters
	}
	if params.Options != nil {
		requestBody["queryOptions"] = params.Options
	}

	resp, err := c.makeRequest("POST", endpoint+query, requestBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NQERunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) StartNQEExecution(networkID, snapshotID string, request *NQEExecutionRequest) (*NQEExecutionResponse, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/nqe-executions", networkID)
	query := url.Values{}
	if snapshotID != "" {
		query.Set("snapshotId", snapshotID)
	}
	endpoint = appendQuery(endpoint, query)

	resp, err := c.makeRequest("POST", endpoint, request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NQEExecutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetNQEExecutionStatus(networkID, executionKey string) (*NQEExecutionStatus, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/nqe-executions/%s", networkID, url.PathEscape(executionKey))

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NQEExecutionStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetNQEExecutionResult(networkID, executionKey string, offset, limit int) (*NQERunResult, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/nqe-executions/%s/result", networkID, url.PathEscape(executionKey))
	query := url.Values{}
	if offset > 0 {
		query.Set("offset", fmt.Sprintf("%d", offset))
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	endpoint = appendQuery(endpoint, query)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NQERunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetNQEQueries(dir string) ([]NQEQuery, error) {
	// DEPRECATED: This method uses the legacy static API endpoint.
	// Use GetNQEAllQueriesEnhanced() for the new database-backed approach.
	warnLogger := logger.New()
	warnLogger.Warn("DEPRECATED: GetNQEQueries() uses legacy static API. Consider using database-backed query discovery instead.")

	endpoint := "/api/nqe/queries"
	if dir != "" {
		endpoint += fmt.Sprintf("?dir=%s", dir)
	}

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE queries: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the raw response for debugging
	debugLogger := logger.New()
	debugLogger.Debug("Raw API response: %s", string(body))

	// Check if the response is empty
	if len(body) == 0 {
		debugLogger.Warn("API returned empty response")
		return []NQEQuery{}, nil
	}

	// Try to parse the response as JSON
	var queries []NQEQuery
	if err := json.Unmarshal(body, &queries); err != nil {
		// If the first attempt fails, try to parse as a single object
		var singleQuery NQEQuery
		if err := json.Unmarshal(body, &singleQuery); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		queries = []NQEQuery{singleQuery}
	}

	// Validate the queries
	validQueries := make([]NQEQuery, 0)
	for _, q := range queries {
		if q.QueryID == "" || q.Path == "" {
			debugLogger.Debug("Skipping invalid query: %+v", q)
			continue
		}
		validQueries = append(validQueries, q)
	}

	// Log the results
	if len(validQueries) == 0 {
		debugLogger.Debug("No valid queries found in response")
	} else {
		debugLogger.Debug("Found %d valid queries", len(validQueries))
		// Log first query as sample
		if sample, err := json.Marshal(validQueries[0]); err == nil {
			debugLogger.Debug("Sample query: %s", string(sample))
		}
	}

	return validQueries, nil
}

func (c *Client) GetNQEOrgQueries() ([]NQEQuery, error) {
	endpoint := "/api/nqe/repos/org/commits/head/queries"

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE org queries: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response using the new structure
	var orgResponse NQEOrgQueriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&orgResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to simplified NQEQuery format for backward compatibility
	queries := make([]NQEQuery, len(orgResponse.Queries))
	for i, q := range orgResponse.Queries {
		queries[i] = NQEQuery{
			QueryID:    q.QueryID,
			Path:       q.Path,
			Intent:     "", // Will be filled in when fetching detailed metadata
			Repository: "org",
		}
	}

	// Log the results
	debugLogger := logger.New()
	debugLogger.Debug("Found %d NQE org queries", len(queries))
	if len(queries) > 0 {
		// Log first query as sample
		if sample, err := json.Marshal(queries[0]); err == nil {
			debugLogger.Debug("Sample query: %s", string(sample))
		}
	}

	return queries, nil
}

func (c *Client) GetNQEOrgQueriesEnhanced() ([]NQEQueryDetail, error) {
	return c.GetNQEOrgQueriesEnhancedWithCache(nil)
}

func (c *Client) GetNQEOrgQueriesEnhancedWithCache(existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	return c.GetNQEOrgQueriesEnhancedWithCacheContext(context.Background(), existingCommitIDs)
}

func (c *Client) GetNQEOrgQueriesEnhancedWithCacheContext(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	// First, get the list of queries with commit IDs
	endpoint := "/api/nqe/repos/org/commits/head/queries"

	// Use retry logic for the initial query list request
	resp, err := c.makeRequestWithRetry(ctx, "GET", endpoint, nil, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE org queries after retries: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response to get query summaries
	var orgResponse NQEOrgQueriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&orgResponse); err != nil {
		return nil, fmt.Errorf("failed to decode org queries response: %w", err)
	}

	debugLogger := logger.New()
	debugLogger.Info("Found %d queries, checking for changes...", len(orgResponse.Queries))

	// Filter queries that need updating
	queriesToFetch := make([]NQEOrgQuerySummary, 0)
	unchangedCount := 0

	for _, querySummary := range orgResponse.Queries {
		// Check if this query has changed
		if existingCommitIDs != nil {
			if existingCommitID, exists := existingCommitIDs[querySummary.Path]; exists {
				if existingCommitID == querySummary.LastCommitId {
					unchangedCount++
					continue // Skip unchanged queries
				}
			}
		}
		queriesToFetch = append(queriesToFetch, querySummary)
	}

	debugLogger.Info("📊 Commit comparison results: %d unchanged, %d to fetch", unchangedCount, len(queriesToFetch))

	// For each query that needs updating, fetch the detailed metadata
	enhancedQueries := make([]NQEQueryDetail, 0, len(queriesToFetch))
	failedQueries := 0
	var firstFailureExample string

	for i, querySummary := range queriesToFetch {
		// Check for context cancellation before processing each query
		select {
		case <-ctx.Done():
			debugLogger.Info("🚫 Org query loading cancelled after processing %d/%d queries", i, len(queriesToFetch))
			return nil, fmt.Errorf("org query loading cancelled: %w", ctx.Err())
		default:
			// Continue processing
		}

		queryDetail, err := c.GetNQEQueryByCommitWithContext(ctx, querySummary.LastCommitId, querySummary.Path, "org")
		if err != nil {
			failedQueries++
			// Log only the first failure as an example, not every single one
			if firstFailureExample == "" {
				firstFailureExample = fmt.Sprintf("Example failure: query %s (commit %s): %v", querySummary.Path, querySummary.LastCommitId, err)
			}
			// Continue with other queries instead of failing completely
			continue
		}

		// Enhance the query detail with path and commit information from the summary
		queryDetail.QueryID = querySummary.QueryID // Ensure consistency
		queryDetail.Path = querySummary.Path       // Add path from org summary

		// Add commit tracking information
		if queryDetail.LastCommit.ID == "" {
			queryDetail.LastCommit.ID = querySummary.LastCommitId
		}

		enhancedQueries = append(enhancedQueries, *queryDetail)

		// Log progress for large numbers of queries (every 100 queries) with cancellation check
		if (i+1)%100 == 0 {
			// Check for cancellation before logging progress
			select {
			case <-ctx.Done():
				debugLogger.Info("🚫 Org query loading cancelled during progress logging at %d/%d queries", i+1, len(queriesToFetch))
				return nil, fmt.Errorf("org query loading cancelled: %w", ctx.Err())
			default:
				debugLogger.Info("Progress: %d/%d queries processed (%d successful, %d failed)",
					i+1, len(queriesToFetch), len(enhancedQueries), failedQueries)
			}
		}
	}

	// Final summary with meaningful statistics
	debugLogger.Info("✅ Enhanced metadata loading complete:")
	debugLogger.Info("  📊 Total queries found: %d", len(orgResponse.Queries))
	debugLogger.Info("  ✅ Successfully loaded: %d", len(enhancedQueries))
	if failedQueries > 0 {
		debugLogger.Info("  ⚠️  Queries with path issues: %d (skipped, this is normal)", failedQueries)
		if firstFailureExample != "" {
			debugLogger.Debug("  %s", firstFailureExample)
		}
	}
	if existingCommitIDs != nil {
		debugLogger.Info("  🚀 Optimization: Skipped %d unchanged queries", unchangedCount)
	}

	return enhancedQueries, nil
}

func (c *Client) GetNQEFwdQueries() ([]NQEQuery, error) {
	endpoint := "/api/nqe/repos/fwd/commits/head/queries"

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE fwd queries: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response using the new structure
	var orgResponse NQEOrgQueriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&orgResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to simplified NQEQuery format for backward compatibility
	queries := make([]NQEQuery, len(orgResponse.Queries))
	for i, q := range orgResponse.Queries {
		queries[i] = NQEQuery{
			QueryID:    q.QueryID,
			Path:       q.Path,
			Intent:     "", // Will be filled in when fetching detailed metadata
			Repository: "fwd",
		}
	}

	// Log the results
	debugLogger := logger.New()
	debugLogger.Debug("Found %d NQE fwd queries", len(queries))
	if len(queries) > 0 {
		// Log first query as sample
		if sample, err := json.Marshal(queries[0]); err == nil {
			debugLogger.Debug("Sample query: %s", string(sample))
		}
	}

	return queries, nil
}

func (c *Client) GetNQEFwdQueriesEnhanced() ([]NQEQueryDetail, error) {
	return c.GetNQEFwdQueriesEnhancedWithCache(nil)
}

func (c *Client) GetNQEFwdQueriesEnhancedWithCache(existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	return c.GetNQEFwdQueriesEnhancedWithCacheContext(context.Background(), existingCommitIDs)
}

func (c *Client) GetNQEFwdQueriesEnhancedWithCacheContext(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	// First, get the list of queries with commit IDs
	endpoint := "/api/nqe/repos/fwd/commits/head/queries"

	// Use retry logic for the initial query list request
	resp, err := c.makeRequestWithRetry(ctx, "GET", endpoint, nil, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE fwd queries after retries: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response to get query summaries
	var orgResponse NQEOrgQueriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&orgResponse); err != nil {
		return nil, fmt.Errorf("failed to decode fwd queries response: %w", err)
	}

	debugLogger := logger.New()
	debugLogger.Info("Found %d fwd queries, checking for changes...", len(orgResponse.Queries))

	// Filter queries that need updating
	queriesToFetch := make([]NQEOrgQuerySummary, 0)
	unchangedCount := 0

	for _, querySummary := range orgResponse.Queries {
		// Check if this query has changed
		if existingCommitIDs != nil {
			if existingCommitID, exists := existingCommitIDs[querySummary.Path]; exists {
				if existingCommitID == querySummary.LastCommitId {
					unchangedCount++
					continue // Skip unchanged queries
				}
			}
		}
		queriesToFetch = append(queriesToFetch, querySummary)
	}

	debugLogger.Info("📊 Fwd commit comparison results: %d unchanged, %d to fetch", unchangedCount, len(queriesToFetch))

	// For each query that needs updating, fetch the detailed metadata
	enhancedQueries := make([]NQEQueryDetail, 0, len(queriesToFetch))
	failedQueries := 0
	var firstFailureExample string

	for i, querySummary := range queriesToFetch {
		// Check for context cancellation before processing each query
		select {
		case <-ctx.Done():
			debugLogger.Info("🚫 Fwd query loading cancelled after processing %d/%d queries", i, len(queriesToFetch))
			return nil, fmt.Errorf("fwd query loading cancelled: %w", ctx.Err())
		default:
			// Continue processing
		}

		queryDetail, err := c.GetNQEQueryByCommitWithContext(ctx, querySummary.LastCommitId, querySummary.Path, "fwd")
		if err != nil {
			failedQueries++
			// Log only the first failure as an example, not every single one
			if firstFailureExample == "" {
				firstFailureExample = fmt.Sprintf("Example failure: query %s (commit %s): %v", querySummary.Path, querySummary.LastCommitId, err)
			}
			// Continue with other queries instead of failing completely
			continue
		}

		// Enhance the query detail with path and commit information from the summary
		queryDetail.QueryID = querySummary.QueryID // Ensure consistency
		queryDetail.Path = querySummary.Path       // Add path from fwd summary

		// Add commit tracking information
		if queryDetail.LastCommit.ID == "" {
			queryDetail.LastCommit.ID = querySummary.LastCommitId
		}

		enhancedQueries = append(enhancedQueries, *queryDetail)

		// Log progress for large numbers of queries (every 100 queries) with cancellation check
		if (i+1)%100 == 0 {
			// Check for cancellation before logging progress
			select {
			case <-ctx.Done():
				debugLogger.Info("🚫 Fwd query loading cancelled during progress logging at %d/%d queries", i+1, len(queriesToFetch))
				return nil, fmt.Errorf("fwd query loading cancelled: %w", ctx.Err())
			default:
				debugLogger.Info("Progress: %d/%d fwd queries processed (%d successful, %d failed)",
					i+1, len(queriesToFetch), len(enhancedQueries), failedQueries)
			}
		}
	}

	// Final summary with meaningful statistics
	debugLogger.Info("✅ Enhanced fwd metadata loading complete:")
	debugLogger.Info("  📊 Total fwd queries found: %d", len(orgResponse.Queries))
	debugLogger.Info("  ✅ Successfully loaded: %d", len(enhancedQueries))
	if failedQueries > 0 {
		debugLogger.Info("  ⚠️  Queries with path issues: %d (skipped, this is normal)", failedQueries)
		if firstFailureExample != "" {
			debugLogger.Debug("  %s", firstFailureExample)
		}
	}
	if existingCommitIDs != nil {
		debugLogger.Info("  🚀 Optimization: Skipped %d unchanged fwd queries", unchangedCount)
	}

	return enhancedQueries, nil
}

func (c *Client) GetNQEAllQueriesEnhanced() ([]NQEQueryDetail, error) {
	return c.GetNQEAllQueriesEnhancedWithCache(nil)
}

func (c *Client) GetNQEAllQueriesEnhancedWithCache(existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	debugLogger := logger.New()
	debugLogger.Info("🔄 Loading queries from BOTH repositories (org + fwd)...")

	// Load from org repository
	debugLogger.Info("📡 Fetching org repository queries...")
	orgQueries, err := c.GetNQEOrgQueriesEnhancedWithCacheContext(context.Background(), existingCommitIDs)
	if err != nil {
		// Check if we were cancelled
		select {
		case <-context.Background().Done():
			debugLogger.Info("🚫 Org query loading cancelled")
			return nil, fmt.Errorf("org query loading cancelled: %w", context.Background().Err())
		default:
		}
		debugLogger.Warn("⚠️  Failed to load org queries: %v", err)
		orgQueries = []NQEQueryDetail{} // Continue with empty org queries
	}

	// Load from fwd repository
	debugLogger.Info("📡 Fetching fwd repository queries...")
	fwdQueries, err := c.GetNQEFwdQueriesEnhancedWithCacheContext(context.Background(), existingCommitIDs)
	if err != nil {
		// Check if we were cancelled
		select {
		case <-context.Background().Done():
			debugLogger.Info("🚫 Fwd query loading cancelled")
			return nil, fmt.Errorf("fwd query loading cancelled: %w", context.Background().Err())
		default:
		}
		debugLogger.Warn("⚠️  Failed to load fwd queries: %v", err)
		fwdQueries = []NQEQueryDetail{} // Continue with empty fwd queries
	}

	// Combine results (org takes precedence for duplicates)
	allQueries := make(map[string]NQEQueryDetail)

	// Add fwd queries first
	for _, q := range fwdQueries {
		q.Repository = "fwd" // Track repository source
		allQueries[q.QueryID] = q
	}

	// Add org queries (will override fwd if same QueryID)
	for _, q := range orgQueries {
		q.Repository = "org" // Track repository source
		allQueries[q.QueryID] = q
	}

	// Convert back to slice
	result := make([]NQEQueryDetail, 0, len(allQueries))
	for _, q := range allQueries {
		result = append(result, q)
	}

	debugLogger.Info("✅ Combined repository loading complete:")
	debugLogger.Info("  📊 Org queries: %d", len(orgQueries))
	debugLogger.Info("  📊 Fwd queries: %d", len(fwdQueries))
	debugLogger.Info("  📊 Total unique queries: %d", len(result))

	return result, nil
}

func (c *Client) GetNQEAllQueriesEnhancedWithCacheContext(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	debugLogger := logger.New()
	debugLogger.Info("🔄 Loading queries from BOTH repositories (org + fwd)...")

	// Load from org repository
	debugLogger.Info("📡 Fetching org repository queries...")
	orgQueries, err := c.GetNQEOrgQueriesEnhancedWithCacheContext(ctx, existingCommitIDs)
	if err != nil {
		// Check if we were cancelled
		select {
		case <-ctx.Done():
			debugLogger.Info("🚫 Org query loading cancelled")
			return nil, fmt.Errorf("org query loading cancelled: %w", ctx.Err())
		default:
		}
		debugLogger.Warn("⚠️  Failed to load org queries: %v", err)
		orgQueries = []NQEQueryDetail{} // Continue with empty org queries
	}

	// Load from fwd repository
	debugLogger.Info("📡 Fetching fwd repository queries...")
	fwdQueries, err := c.GetNQEFwdQueriesEnhancedWithCacheContext(ctx, existingCommitIDs)
	if err != nil {
		// Check if we were cancelled
		select {
		case <-ctx.Done():
			debugLogger.Info("🚫 Fwd query loading cancelled")
			return nil, fmt.Errorf("fwd query loading cancelled: %w", ctx.Err())
		default:
		}
		debugLogger.Warn("⚠️  Failed to load fwd queries: %v", err)
		fwdQueries = []NQEQueryDetail{} // Continue with empty fwd queries
	}

	// Combine results (org takes precedence for duplicates)
	allQueries := make(map[string]NQEQueryDetail)

	// Add fwd queries first
	for _, q := range fwdQueries {
		q.Repository = "fwd" // Track repository source
		allQueries[q.QueryID] = q
	}

	// Add org queries (will override fwd if same QueryID)
	for _, q := range orgQueries {
		q.Repository = "org" // Track repository source
		allQueries[q.QueryID] = q
	}

	// Convert back to slice
	result := make([]NQEQueryDetail, 0, len(allQueries))
	for _, q := range allQueries {
		result = append(result, q)
	}

	debugLogger.Info("✅ Combined repository loading complete:")
	debugLogger.Info("  📊 Org queries: %d", len(orgQueries))
	debugLogger.Info("  📊 Fwd queries: %d", len(fwdQueries))
	debugLogger.Info("  📊 Total unique queries: %d", len(result))

	return result, nil
}

func (c *Client) GetNQEQueryByCommit(commitID string, path string, repository string) (*NQEQueryDetail, error) {
	return c.GetNQEQueryByCommitWithContext(context.Background(), commitID, path, repository)
}

func (c *Client) GetNQEQueryByCommitWithContext(ctx context.Context, commitID string, path string, repository string) (*NQEQueryDetail, error) {
	endpoint := fmt.Sprintf("/api/nqe/repos/%s/commits/%s/queries?path=%s", repository, commitID, url.QueryEscape(path))
	// Use retry logic for individual query requests
	resp, err := c.makeRequestWithRetry(ctx, "GET", endpoint, nil, 2) // 2 retries for individual queries
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE query by commit after retries: %w", err)
	}
	defer resp.Body.Close()

	var queryDetail NQEQueryDetail
	if err := json.NewDecoder(resp.Body).Decode(&queryDetail); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &queryDetail, nil
}

func (c *Client) DiffNQEQuery(before, after string, request *NQEDiffRequest) (*NQEDiffResult, error) {
	endpoint := fmt.Sprintf("/api/nqe-diffs/%s/%s", before, after)

	resp, err := c.makeRequest("POST", endpoint, request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NQEDiffResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetSnapshotDiff(before, after, path string, query url.Values) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/diffs/%s/%s/%s", before, after, strings.TrimPrefix(path, "/"))
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (c *Client) GetBlastRadius(networkID, snapshotID string, request *BlastRadiusRequest, hostCentric bool) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/blast-radius", networkID)
	query := url.Values{}
	if hostCentric {
		query.Set("type", "host-centric")
	}
	if snapshotID != "" && snapshotID != "latest" {
		query.Set("snapshotId", snapshotID)
	}
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	resp, err := c.makeRequest("POST", endpoint, request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (c *Client) SuggestBlastRadiusSources(networkID, snapshotID, query string, max int) ([]map[string]interface{}, error) {
	params := url.Values{}
	params.Set("for", "blastRadius")
	params.Set("q", query)
	if max > 0 {
		params.Set("max", fmt.Sprintf("%d", max))
	}
	if snapshotID != "" && snapshotID != "latest" {
		params.Set("snapshotId", snapshotID)
	}
	endpoint := fmt.Sprintf("/api/networks/%s/suggestions/locations?%s", networkID, params.Encode())

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// Device operations
func (c *Client) GetDevices(networkID string, params *DeviceQueryParams) (*DeviceResponse, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/devices", networkID)

	// Build query parameters
	query := ""
	if params.SnapshotID != "" {
		query += fmt.Sprintf("?snapshotId=%s", params.SnapshotID)
	}
	if params.Offset > 0 {
		if query == "" {
			query += "?"
		} else {
			query += "&"
		}
		// API pagination parameter is "skip", not "offset"
		query += fmt.Sprintf("skip=%d", params.Offset)
	}
	if params.Limit > 0 {
		if query == "" {
			query += "?"
		} else {
			query += "&"
		}
		query += fmt.Sprintf("limit=%d", params.Limit)
	}

	resp, err := c.makeRequest("GET", endpoint+query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The API returns a direct array of devices, not wrapped in a response object
	var devices []Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Wrap in our response structure for consistency
	deviceResp := &DeviceResponse{
		Devices:    devices,
		TotalCount: len(devices),
	}

	return deviceResp, nil
}

func (c *Client) GetMissingDevices(networkID, snapshotID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/missing-devices", networkID)
	query := url.Values{}
	if snapshotID != "" && snapshotID != "latest" {
		query.Set("snapshotId", snapshotID)
	}

	resp, err := c.makeRequest("GET", appendQuery(endpoint, query), nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) GetDeviceLocations(networkID string) (map[string]string, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/atlas", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var locations map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&locations); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return locations, nil
}

func (c *Client) UpdateDeviceLocations(networkID string, locations map[string]string) error {
	endpoint := fmt.Sprintf("/api/networks/%s/atlas", networkID)

	resp, err := c.makeRequest("PATCH", endpoint, locations)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func queryInclude(include []string) string {
	if len(include) == 0 {
		return ""
	}

	values := url.Values{}
	for _, item := range include {
		if strings.TrimSpace(item) != "" {
			values.Add("with", item)
		}
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func decodeObjectResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

func decodeArrayResponse(resp *http.Response) ([]map[string]interface{}, error) {
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

func appendQuery(endpoint string, query url.Values) string {
	if len(query) == 0 {
		return endpoint
	}
	return endpoint + "?" + query.Encode()
}

func (c *Client) GetL7Applications() ([]map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", "/api/l7-applications", nil)
	if err != nil {
		return nil, err
	}

	return decodeArrayResponse(resp)
}

// Network setup operations
func (c *Client) GetClassicDevices(networkID string, include []string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/classic-devices%s", networkID, queryInclude(include))

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) GetClassicDevice(networkID string, deviceName string, include []string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf(
		"/api/networks/%s/classic-devices/%s%s",
		networkID,
		url.PathEscape(deviceName),
		queryInclude(include),
	)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) UpsertClassicDevices(networkID string, devices []map[string]interface{}) error {
	endpoint := fmt.Sprintf("/api/networks/%s/classic-devices?action=putBatch", networkID)

	resp, err := c.makeRequest("POST", endpoint, devices)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) DeleteClassicDevices(networkID string, names []string) error {
	endpoint := fmt.Sprintf("/api/networks/%s/classic-devices?action=deleteBatch", networkID)
	body := map[string]interface{}{"names": names}

	resp, err := c.makeRequest("POST", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) GetCliCredentials(networkID string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/cli-credentials", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

func (c *Client) CreateCliCredential(networkID string, credential map[string]interface{}) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/cli-credentials", networkID)

	resp, err := c.makeRequest("POST", endpoint, credential)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) DeleteCliCredential(networkID string, credentialID string) error {
	endpoint := fmt.Sprintf("/api/networks/%s/cli-credentials/%s", networkID, url.PathEscape(credentialID))

	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) GetSnmpCredentials(networkID string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/snmpCredentials", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeArrayResponse(resp)
}

func (c *Client) CreateSnmpCredential(networkID string, credential map[string]interface{}) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/snmpCredentials", networkID)

	resp, err := c.makeRequest("POST", endpoint, credential)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) DeleteSnmpCredential(networkID string, credentialID string) error {
	endpoint := fmt.Sprintf("/api/networks/%s/snmpCredentials/%s", networkID, url.PathEscape(credentialID))

	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) GetPerformanceSettings(networkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/performance/settings", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) UpdatePerformanceSettings(networkID string, settings map[string]interface{}) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/performance/settings", networkID)

	resp, err := c.makeRequest("PATCH", endpoint, settings)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) GetDevicesWithPerformanceData(networkID string, query url.Values) ([]string, error) {
	endpoint := appendQuery(fmt.Sprintf("/api/networks/%s/performance/devices-with-data", networkID), query)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

func (c *Client) GetDevicePerformance(networkID, deviceName string, query url.Values) (map[string]interface{}, error) {
	endpoint := appendQuery(
		fmt.Sprintf("/api/networks/%s/devices/%s/performance", networkID, url.PathEscape(deviceName)),
		query,
	)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) GetInterfacePerformance(networkID, deviceName, interfaceName string, query url.Values) (map[string]interface{}, error) {
	endpoint := appendQuery(
		fmt.Sprintf(
			"/api/networks/%s/devices/%s/interfaces/%s/performance",
			networkID,
			url.PathEscape(deviceName),
			url.PathEscape(interfaceName),
		),
		query,
	)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

// Network collection operations
func (c *Client) GetCollectorStatus(networkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/collector/status", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) AddCollectorTask(networkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/collector-tasks?networkId=%s&type=NETWORK_COLLECTION", url.QueryEscape(networkID))

	resp, err := c.makeRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) GetCollectorTask(taskID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/collector-tasks/%s", url.PathEscape(taskID))

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) StartCollection(networkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/startcollection", networkID)

	resp, err := c.makeRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) CancelCollection(networkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/cancelcollection", networkID)

	resp, err := c.makeRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

func (c *Client) GetCollectionSchedules(networkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/collection-schedules", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}

// Snapshot operations
func (c *Client) GetSnapshots(networkID string) ([]Snapshot, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/snapshots", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The API returns an object with a snapshots array property
	var snapshotsResp SnapshotsResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapshotsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return snapshotsResp.Snapshots, nil
}

func (c *Client) GetLatestSnapshot(networkID string) (*Snapshot, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/snapshots/latestProcessed", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var snapshot Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &snapshot, nil
}

func (c *Client) DeleteSnapshot(snapshotID string) error {
	endpoint := fmt.Sprintf("/api/snapshots/%s", snapshotID)

	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Location operations
func (c *Client) GetLocations(networkID string) ([]Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations", networkID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var locations []Location
	if err := json.NewDecoder(resp.Body).Decode(&locations); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return locations, nil
}

func (c *Client) CreateLocation(networkID string, location *LocationCreate) (*Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations", networkID)

	resp, err := c.makeRequest("POST", endpoint, location)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var newLocation Location
	if err := json.NewDecoder(resp.Body).Decode(&newLocation); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &newLocation, nil
}

// CreateLocationsBulk creates or updates multiple locations using PATCH.
// The API returns 204 No Content on success.
func (c *Client) CreateLocationsBulk(networkID string, locations []LocationBulkPatch) error {
	endpoint := fmt.Sprintf("/api/networks/%s/locations", networkID)

	resp, err := c.makeRequest("PATCH", endpoint, locations)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Expecting 204 No Content; treat any 2xx as success.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bulk patch locations failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) UpdateLocation(networkID string, locationID string, update *LocationUpdate) (*Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations/%s", networkID, locationID)

	resp, err := c.makeRequest("PATCH", endpoint, update)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var location Location
	if err := json.NewDecoder(resp.Body).Decode(&location); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &location, nil
}

func (c *Client) DeleteLocation(networkID string, locationID string) (*Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations/%s", networkID, locationID)

	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var location Location
	if err := json.NewDecoder(resp.Body).Decode(&location); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &location, nil
}

// Checks operations
func (c *Client) GetAvailablePredefinedChecks() ([]map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", "/api/predefinedChecks", nil)
	if err != nil {
		return nil, err
	}

	return decodeArrayResponse(resp)
}

func (c *Client) GetChecks(snapshotID string, query url.Values) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/snapshots/%s/checks", url.PathEscape(snapshotID))

	resp, err := c.makeRequest("GET", appendQuery(endpoint, query), nil)
	if err != nil {
		return nil, err
	}

	return decodeArrayResponse(resp)
}

func (c *Client) GetCheck(snapshotID, checkID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/snapshots/%s/checks/%s", url.PathEscape(snapshotID), url.PathEscape(checkID))

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	return decodeObjectResponse(resp)
}
