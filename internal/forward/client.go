package forward

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/forward-mcp/internal/config"
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
	RunNQEQuery(params *NQEQueryParams) (*NQERunResult, error)
	GetNQEQueries(dir string) ([]NQEQuery, error)
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
	FilterText string `json:"filterText"`
}

type NQERunResult struct {
	SnapshotID string                   `json:"snapshotId"`
	Items      []map[string]interface{} `json:"items"`
}

type NQEQuery struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Directory   string `json:"directory"`
	Description string `json:"description,omitempty"`
	Query       string `json:"query"`
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
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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
func (c *Client) RunNQEQuery(params *NQEQueryParams) (*NQERunResult, error) {
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

	resp, err := c.makeRequest("POST", endpoint+query, params)
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
	endpoint := "/api/nqe/queries"
	if dir != "" {
		endpoint += fmt.Sprintf("?dir=%s", dir)
	}

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var queries []NQEQuery
	if err := json.NewDecoder(resp.Body).Decode(&queries); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return queries, nil
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
