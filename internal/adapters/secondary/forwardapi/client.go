package forwardapi

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

	"github.com/forward-mcp/internal/ports"
)

// Client represents the Forward platform client
type Client struct {
	httpClient *http.Client
	config     *ports.ForwardConfig
	log        ports.Logger
}

// NewClient creates a new Forward platform client
func NewClient(config *ports.ForwardConfig, log ports.Logger) *Client {
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
		log:    log,
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

// Helper method to make authenticated requests
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.config.APIBaseURL+endpoint, bytes.NewBuffer(reqBody))
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
			debugLogger := c.log
			debugLogger.Debug("API Error Response: Status=%d, Endpoint=%s, Body=%s", resp.StatusCode, endpoint, string(errorBody))

			// Provide sanitized message to LLM
			errorMsg += ". Check server logs for details"
		}

		// Log additional debugging information for 400 errors
		// Security: Do not log request bodies as they may contain sensitive data
		if resp.StatusCode == 400 {
			debugLogger := c.log
			debugLogger.Debug("400 Bad Request - URL: %s%s, Method: %s, Body Size: %d bytes",
				c.config.APIBaseURL, endpoint, method, len(reqBody))
			return nil, fmt.Errorf("bad request (HTTP 400): the API rejected the request parameters. Please verify all required fields are provided and have valid values")
		}

		return nil, fmt.Errorf("%s", errorMsg)
	}

	return resp, nil
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
		if debugLogger := c.log; debugLogger != nil {
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

// Network operations
func (c *Client) GetNetworks(ctx context.Context) ([]Network, error) {
	resp, err := c.makeRequest(ctx, "GET", "/api/networks", nil)
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

func (c *Client) CreateNetwork(ctx context.Context, name string) (*Network, error) {
	resp, err := c.makeRequest(ctx, "POST", fmt.Sprintf("/api/networks?name=%s", url.QueryEscape(name)), nil)
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

func (c *Client) DeleteNetwork(ctx context.Context, networkID string) (*Network, error) {
	resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/api/networks/%s", networkID), nil)
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

func (c *Client) UpdateNetwork(ctx context.Context, networkID string, update *NetworkUpdate) (*Network, error) {
	resp, err := c.makeRequest(ctx, "PATCH", fmt.Sprintf("/api/networks/%s", networkID), update)
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
func (c *Client) SearchPaths(ctx context.Context, networkID string, params *PathSearchParams) (*PathSearchResponse, error) {
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

	resp, err := c.makeRequest(ctx, "GET", endpoint+query, nil)
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

func (c *Client) SearchPathsBulk(ctx context.Context, networkID string, request *PathSearchBulkRequest, snapshotID string) ([]PathSearchBulkResponse, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/paths-bulk", networkID)

	// Add snapshotId as query parameter if provided (optional for bulk API)
	if snapshotID != "" && snapshotID != "latest" {
		endpoint += fmt.Sprintf("?snapshotId=%s", snapshotID)
	}

	// Debug logging
	debugLogger := c.log
	debugLogger.Debug("SearchPathsBulk - URL: %s, snapshotID: %s", endpoint, snapshotID)

	resp, err := c.makeRequest(ctx, "POST", endpoint, request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responses []PathSearchBulkResponse
	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Debug logging
	bulkLogger := c.log
	bulkLogger.Debug("SearchPathsBulk decoded %d responses", len(responses))
	if len(responses) > 0 {
		bulkLogger.Debug("First response: %+v", responses[0])
	}

	return responses, nil
}

// NQE operations
func (c *Client) RunNQEQueryByString(ctx context.Context, params *NQEQueryParams) (*NQERunResult, error) {
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
	debugLogger := c.log
	if requestBodyJSON, err := json.Marshal(requestBody); err == nil {
		debugLogger.Debug("NQE String Query Request - URL: %s%s, Body: %s", endpoint, query, string(requestBodyJSON))
	}

	resp, err := c.makeRequest(ctx, "POST", endpoint+query, requestBody)
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

func (c *Client) RunNQEQueryByID(ctx context.Context, params *NQEQueryParams) (*NQERunResult, error) {
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

	resp, err := c.makeRequest(ctx, "POST", endpoint+query, requestBody)
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

func (c *Client) GetNQEQueries(ctx context.Context, dir string) ([]NQEQuery, error) {
	// DEPRECATED: This method uses the legacy static API endpoint.
	// Use GetNQEAllQueriesEnhanced() for the new database-backed approach.
	warnLogger := c.log
	warnLogger.Warn("DEPRECATED: GetNQEQueries() uses legacy static API. Consider using database-backed query discovery instead.")

	endpoint := "/api/nqe/queries"
	if dir != "" {
		endpoint += fmt.Sprintf("?dir=%s", dir)
	}

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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
	debugLogger := c.log
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

func (c *Client) GetNQEOrgQueries(ctx context.Context) ([]NQEQuery, error) {
	endpoint := "/api/nqe/repos/org/commits/head/queries"

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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
	debugLogger := c.log
	debugLogger.Debug("Found %d NQE org queries", len(queries))
	if len(queries) > 0 {
		// Log first query as sample
		if sample, err := json.Marshal(queries[0]); err == nil {
			debugLogger.Debug("Sample query: %s", string(sample))
		}
	}

	return queries, nil
}

func (c *Client) GetNQEOrgQueriesEnhanced(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
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

	debugLogger := c.log
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

		queryDetail, err := c.GetNQEQueryByCommit(ctx, querySummary.LastCommitId, querySummary.Path, "org")
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

func (c *Client) GetNQEFwdQueries(ctx context.Context) ([]NQEQuery, error) {
	endpoint := "/api/nqe/repos/fwd/commits/head/queries"

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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
	debugLogger := c.log
	debugLogger.Debug("Found %d NQE fwd queries", len(queries))
	if len(queries) > 0 {
		// Log first query as sample
		if sample, err := json.Marshal(queries[0]); err == nil {
			debugLogger.Debug("Sample query: %s", string(sample))
		}
	}

	return queries, nil
}

func (c *Client) GetNQEFwdQueriesEnhanced(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
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

	debugLogger := c.log
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

		queryDetail, err := c.GetNQEQueryByCommit(ctx, querySummary.LastCommitId, querySummary.Path, "fwd")
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

func (c *Client) GetNQEAllQueriesEnhanced(ctx context.Context, existingCommitIDs map[string]string) ([]NQEQueryDetail, error) {
	debugLogger := c.log
	debugLogger.Info("🔄 Loading queries from BOTH repositories (org + fwd)...")

	// Load from org repository
	debugLogger.Info("📡 Fetching org repository queries...")
	orgQueries, err := c.GetNQEOrgQueriesEnhanced(ctx, existingCommitIDs)
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
	fwdQueries, err := c.GetNQEFwdQueriesEnhanced(ctx, existingCommitIDs)
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

func (c *Client) GetNQEQueryByCommit(ctx context.Context, commitID string, path string, repository string) (*NQEQueryDetail, error) {
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

func (c *Client) DiffNQEQuery(ctx context.Context, before, after string, request *NQEDiffRequest) (*NQEDiffResult, error) {
	endpoint := fmt.Sprintf("/api/nqe-diffs/%s/%s", before, after)

	resp, err := c.makeRequest(ctx, "POST", endpoint, request)
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
func (c *Client) GetDevices(ctx context.Context, networkID string, params *DeviceQueryParams) (*DeviceResponse, error) {
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

	resp, err := c.makeRequest(ctx, "GET", endpoint+query, nil)
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

func (c *Client) GetDeviceLocations(ctx context.Context, networkID string) (map[string]string, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/atlas", networkID)

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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

func (c *Client) UpdateDeviceLocations(ctx context.Context, networkID string, locations map[string]string) error {
	endpoint := fmt.Sprintf("/api/networks/%s/atlas", networkID)

	resp, err := c.makeRequest(ctx, "PATCH", endpoint, locations)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Snapshot operations
func (c *Client) GetSnapshots(ctx context.Context, networkID string) ([]Snapshot, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/snapshots", networkID)

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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

func (c *Client) GetLatestSnapshot(ctx context.Context, networkID string) (*Snapshot, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/snapshots/latestProcessed", networkID)

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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

func (c *Client) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	endpoint := fmt.Sprintf("/api/snapshots/%s", snapshotID)

	resp, err := c.makeRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Location operations
func (c *Client) GetLocations(ctx context.Context, networkID string) ([]Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations", networkID)

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
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

func (c *Client) CreateLocation(ctx context.Context, networkID string, location *LocationCreate) (*Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations", networkID)

	resp, err := c.makeRequest(ctx, "POST", endpoint, location)
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
func (c *Client) CreateLocationsBulk(ctx context.Context, networkID string, locations []LocationBulkPatch) error {
	endpoint := fmt.Sprintf("/api/networks/%s/locations", networkID)

	resp, err := c.makeRequest(ctx, "PATCH", endpoint, locations)
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

func (c *Client) UpdateLocation(ctx context.Context, networkID string, locationID string, update *LocationUpdate) (*Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations/%s", networkID, locationID)

	resp, err := c.makeRequest(ctx, "PATCH", endpoint, update)
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

func (c *Client) DeleteLocation(ctx context.Context, networkID string, locationID string) (*Location, error) {
	endpoint := fmt.Sprintf("/api/networks/%s/locations/%s", networkID, locationID)

	resp, err := c.makeRequest(ctx, "DELETE", endpoint, nil)
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

// Client implements the ForwardAPI port.
var _ ports.ForwardAPI = (*Client)(nil)
