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
	"net/http"
	"net/url"
	"os"
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
	SearchPaths(networkID string, params *PathSearchParams) (*PathSearchResponse, error)
	SearchPathsBulk(networkID string, requests []PathSearchParams) ([]PathSearchResponse, error)

	// NQE operations
	RunNQEQueryByString(params *NQEQueryParams) (*NQERunResult, error)
	RunNQEQueryByID(params *NQEQueryParams) (*NQERunResult, error)
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
	DiffNQEQuery(before, after string, request *NQEDiffRequest) (*NQEDiffResult, error)

	// Device operations
	GetDevices(networkID string, params *DeviceQueryParams) (*DeviceResponse, error)
	GetDeviceLocations(networkID string) (map[string]string, error)
	UpdateDeviceLocations(networkID string, locations map[string]string) error

	// Snapshot operations
	GetSnapshots(networkID string) ([]Snapshot, error)
	GetLatestSnapshot(networkID string) (*Snapshot, error)
	DeleteSnapshot(snapshotID string) error

	// Location operations
	GetLocations(networkID string) ([]Location, error)
	CreateLocation(networkID string, location *LocationCreate) (*Location, error)
	UpdateLocation(networkID string, locationID string, update *LocationUpdate) (*Location, error)
	DeleteLocation(networkID string, locationID string) (*Location, error)
}

// Client represents the Forward platform client
type Client struct {
	httpClient *http.Client
	config     *config.ForwardConfig
}

// NewClient creates a new Forward platform client
func NewClient(config *config.ForwardConfig) ClientInterface {
	// Create TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.InsecureSkipVerify,
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
	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	OrgID       string `json:"orgId,omitempty"`
	CreatorID   string `json:"creatorId,omitempty"`
	Creator     string `json:"creator,omitempty"`
}

type NetworkUpdate struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
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

type PathSearchResponse struct {
	Paths              []Path                 `json:"paths"`
	ReturnPaths        []Path                 `json:"returnPaths,omitempty"`
	UnrecognizedValues map[string]interface{} `json:"unrecognizedValues,omitempty"`
	SnapshotID         string                 `json:"snapshotId"`
	SearchTimeMs       int                    `json:"searchTimeMs"`
	NumCandidatesFound int                    `json:"numCandidatesFound"`
}

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
	SortBy  []NQESortBy       `json:"sortBy,omitempty"`
	Filters []NQEColumnFilter `json:"columnFilters,omitempty"`
	Format  string            `json:"format,omitempty"`
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
	SnapshotID string                   `json:"snapshotId"`
	Items      []map[string]interface{} `json:"items"`
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
	TotalNumValues int                      `json:"totalNumValues"`
	Rows           []map[string]interface{} `json:"rows"`
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
	Name          string                 `json:"name"`
	Type          string                 `json:"type,omitempty"`
	Vendor        string                 `json:"vendor,omitempty"`
	OSVersion     string                 `json:"osVersion,omitempty"`
	Platform      string                 `json:"platform,omitempty"`
	Model         string                 `json:"model,omitempty"`
	ManagementIPs []string               `json:"managementIps,omitempty"`
	Hostname      string                 `json:"hostname,omitempty"`
	Version       string                 `json:"version,omitempty"`
	SerialNumber  string                 `json:"serialNumber,omitempty"`
	LocationID    string                 `json:"locationId,omitempty"`
	Interfaces    []DeviceInterface      `json:"interfaces,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
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
	ID                 string `json:"id"`
	ProcessingTrigger  string `json:"processingTrigger,omitempty"`
	TotalDevices       int    `json:"totalDevices,omitempty"`
	TotalEndpoints     int    `json:"totalEndpoints,omitempty"`
	TotalOtherSources  int    `json:"totalOtherSources,omitempty"`
	CreationDateMillis int64  `json:"creationDateMillis,omitempty"`
	ProcessedAtMillis  int64  `json:"processedAtMillis,omitempty"`
	IsDraft            bool   `json:"isDraft,omitempty"`
	State              string `json:"state,omitempty"`
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
	CreatedAt int64      `json:"createdAt"`
	OrgID     string     `json:"orgId"`
	CreatorID string     `json:"creatorId"`
	Snapshots []Snapshot `json:"snapshots"`
}

// Location types
type Location struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Latitude    *float64               `json:"latitude,omitempty"`
	Longitude   *float64               `json:"longitude,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

type LocationCreate struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Latitude    *float64               `json:"latitude,omitempty"`
	Longitude   *float64               `json:"longitude,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

type LocationUpdate struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Latitude    *float64               `json:"latitude,omitempty"`
	Longitude   *float64               `json:"longitude,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
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
	auth := base64.StdEncoding.EncodeToString([]byte(c.config.APIKey + ":" + c.config.APISecret))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the response body for error details
		errorBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		errorMsg := fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
		if readErr == nil && len(errorBody) > 0 {
			errorMsg += fmt.Sprintf(", response: %s", string(errorBody))
		}

		// Log additional debugging information for 400 errors
		if resp.StatusCode == 400 {
			debugLogger := logger.New()
			debugLogger.Debug("400 Bad Request - URL: %s%s, Method: %s, Request Body: %s",
				c.config.APIBaseURL, endpoint, method, string(reqBody))
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
	resp, err := c.makeRequest("POST", fmt.Sprintf("/api/networks?name=%s", name), nil)
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
	query := fmt.Sprintf("?dstIp=%s", params.DstIP)
	if params.From != "" {
		query += fmt.Sprintf("&from=%s", params.From)
	}
	if params.SrcIP != "" {
		query += fmt.Sprintf("&srcIp=%s", params.SrcIP)
	}
	if params.Intent != "" {
		query += fmt.Sprintf("&intent=%s", params.Intent)
	}
	if params.IPProto != nil {
		query += fmt.Sprintf("&ipProto=%d", *params.IPProto)
	}
	if params.SrcPort != "" {
		query += fmt.Sprintf("&srcPort=%s", params.SrcPort)
	}
	if params.DstPort != "" {
		query += fmt.Sprintf("&dstPort=%s", params.DstPort)
	}
	if params.IncludeNetworkFunctions {
		query += "&includeNetworkFunctions=true"
	}
	if params.MaxCandidates > 0 {
		query += fmt.Sprintf("&maxCandidates=%d", params.MaxCandidates)
	}
	if params.MaxResults > 0 {
		query += fmt.Sprintf("&maxResults=%d", params.MaxResults)
	}
	if params.MaxReturnPathResults > 0 {
		query += fmt.Sprintf("&maxReturnPathResults=%d", params.MaxReturnPathResults)
	}
	if params.MaxSeconds > 0 {
		query += fmt.Sprintf("&maxSeconds=%d", params.MaxSeconds)
	}
	if params.SnapshotID != "" {
		query += fmt.Sprintf("&snapshotId=%s", params.SnapshotID)
	}

	resp, err := c.makeRequest("GET", endpoint+query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pathResp PathSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&pathResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &pathResp, nil
}

func (c *Client) SearchPathsBulk(networkID string, requests []PathSearchParams) ([]PathSearchResponse, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/paths-bulk", networkID)

	resp, err := c.makeRequest("POST", endpoint, requests)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responses []PathSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return responses, nil
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

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE org queries: %w", err)
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

		queryDetail, err := c.GetNQEQueryByCommit(querySummary.LastCommitId, querySummary.Path, "org")
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

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE fwd queries: %w", err)
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

		queryDetail, err := c.GetNQEQueryByCommit(querySummary.LastCommitId, querySummary.Path, "fwd")
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
	endpoint := fmt.Sprintf("/api/nqe/repos/%s/commits/%s/queries?path=%s", repository, commitID, url.QueryEscape(path))

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get NQE query by commit: %w", err)
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
		query += fmt.Sprintf("offset=%d", params.Offset)
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
