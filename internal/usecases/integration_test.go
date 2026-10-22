package usecases

import (
	"context"
	"github.com/forward-mcp/internal/adapters/secondary/bloom"
	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/adapters/secondary/envconfig"
	"github.com/forward-mcp/internal/adapters/secondary/forwardapi"
	"github.com/forward-mcp/internal/adapters/secondary/queryindex"
	"github.com/forward-mcp/internal/adapters/secondary/semcache"
	"github.com/forward-mcp/internal/adapters/secondary/sqlite"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logger "github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/domain"
	"github.com/joho/godotenv"
)

// getTestNetworkID returns the network to run integration tests against,
// preferring FORWARD_DEFAULT_NETWORK_ID over the first network in the account.
func getTestNetworkID(networks []domain.Network) string {
	if id := os.Getenv("FORWARD_DEFAULT_NETWORK_ID"); id != "" {
		return id
	}
	return networks[0].ID
}

// skipIfNoAccess skips the test when the chosen network cannot be read with
// the configured credentials (HTTP 403 or 404). A wrong FORWARD_DEFAULT_NETWORK_ID
// or FORWARD_API_BASE_URL in the shell then reads as a bad environment, not as a
// broken build.
func skipIfNoAccess(t *testing.T, networkID string, err error) {
	t.Helper()
	if err != nil && (strings.Contains(err.Error(), "HTTP 403") || strings.Contains(err.Error(), "HTTP 404")) {
		t.Skipf("network %q is not accessible with these credentials (check FORWARD_DEFAULT_NETWORK_ID and FORWARD_API_BASE_URL): %v", networkID, err)
	}
}

// Default path-search endpoints: atl-dc1-ce01 gi2 and atl-dc1in-panos-fw1
// ethernet1/4 in the Skyforge "Demo Network", a routed link through a firewall
// permit that is delivered in both directions.
const (
	defaultTestSrcIP = "10.20.1.1"
	defaultTestDstIP = "10.20.1.2"
)

// getTestPathIPs returns the source and destination IPs for path search tests,
// preferring FORWARD_TEST_SRC_IP and FORWARD_TEST_DST_IP over the defaults.
func getTestPathIPs() (src, dst string) {
	src, dst = defaultTestSrcIP, defaultTestDstIP
	if v := os.Getenv("FORWARD_TEST_SRC_IP"); v != "" {
		src = v
	}
	if v := os.Getenv("FORWARD_TEST_DST_IP"); v != "" {
		dst = v
	}
	return src, dst
}

// requirePathsFound fails the test when a bulk path search returned no paths.
func requirePathsFound(t *testing.T, content, src, dst string) {
	t.Helper()
	if strings.Contains(content, "found 0 total paths") {
		t.Errorf("no paths found from %s to %s. Set FORWARD_TEST_SRC_IP and FORWARD_TEST_DST_IP to two addresses with a path in this network", src, dst)
	}
}

// getProjectRoot returns the absolute path to the project root.
func getProjectRoot() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

// setupIntegrationTest loads environment variables and creates a real service
func setupIntegrationTest(t *testing.T) *Service {
	rootDir := getProjectRoot()
	envPath := filepath.Join(rootDir, ".env")

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Skip(".env file not found, skipping integration tests")
	}
	_ = godotenv.Load(envPath)

	// Use the standard config loading which includes all TLS settings
	log := logger.New()
	cfg, err := envconfig.Load(log)
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	// Verify required credentials are set
	if cfg.Forward.APIKey == "" || cfg.Forward.APISecret == "" || cfg.Forward.APIBaseURL == "" {
		t.Skip("FORWARD_API_KEY, FORWARD_API_SECRET, and FORWARD_API_BASE_URL must be set to run integration tests")
	}

	// Set a longer timeout for integration tests
	if cfg.Forward.Timeout < 30 {
		cfg.Forward.Timeout = 30
	}

	embedder := embeddings.NewKeywordEmbeddingService()
	return New(cfg, log, Deps{
		API:        forwardapi.NewClient(&cfg.Forward, log),
		Cache:      semcache.NewSemanticCache(embedder, log, InstanceID(cfg), &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
		Bloom:      bloom.NewBloomSearchManager(log, InstanceID(cfg)),
	})
}

// Integration test for listing networks with real API
func TestIntegrationListNetworks(t *testing.T) {
	service := setupIntegrationTest(t)

	response, err := service.ListNetworks(context.Background(), ListNetworksArgs{})
	if err != nil {
		t.Fatalf("Failed to list networks: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Text == "" {
		t.Fatal("Expected text content, got none")
	}

	content := response.Text
	if content == "" {
		t.Fatal("Expected non-empty content")
	}

	t.Logf("Networks response: %s", content)
}

// Integration test for searching paths with real API (if networks exist)
func TestIntegrationSearchPaths(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for path search test")
	}

	// Use the first network for testing
	networkID := getTestNetworkID(networks)
	t.Logf("Testing path search on network %s", networkID)

	args := SearchPathsBulkArgs{
		NetworkID: networkID,
		Queries: []PathSearchQueryArgs{
			{
				DstIP: "8.8.8.8", // Use a common destination
			},
		},
		MaxResults: 1,
	}

	response, err := service.searchPathsBulk(context.Background(), args)
	if err != nil {
		// Path search might fail if no valid paths exist, which is OK
		t.Logf("Path search failed (this may be expected): %v", err)
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Text
	t.Logf("Path search response: %s", content)
}

// Integration test for path search between the test IPs (see getTestPathIPs)
func TestIntegrationSearchPathsSpecificIPs(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for path search test")
	}

	// Use the first network for testing
	networkID := getTestNetworkID(networks)
	srcIP, dstIP := getTestPathIPs()
	t.Logf("Testing path search on network %s", networkID)

	// Test scenarios between the test IPs
	testCases := []struct {
		name        string
		args        SearchPathsBulkArgs
		expectError bool
		description string
	}{
		{
			name: "Basic",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Basic path search with the test source and destination IPs",
		},
		{
			name: "With_Intent",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				Intent:     "PREFER_DELIVERED",
				MaxResults: 10,
			},
			expectError: false,
			description: "Path search with PREFER_DELIVERED intent",
		},
		{
			name: "With_Functions",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				IncludeNetworkFunctions: true,
				MaxResults:              3,
			},
			expectError: false,
			description: "Path search including network functions for detailed analysis",
		},
		{
			name: "TCP_80",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP:   srcIP,
						DstIP:   dstIP,
						IPProto: &[]int{6}[0], // TCP
						DstPort: "80",
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Path search for TCP traffic to port 80",
		},
		{
			name: "Reverse_Path",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: dstIP, // Reverse direction
						DstIP: srcIP,
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Reverse path search to test bidirectional connectivity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)

			response, err := service.searchPathsBulk(context.Background(), tc.args)

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !tc.expectError && err != nil {
				// For path search, some failures are expected if the network doesn't contain these specific IPs
				t.Logf("Path search failed (may be expected for this network): %v", err)

				// Log the specific error for debugging
				if strings.Contains(err.Error(), "404") {
					t.Logf("404 error suggests the IPs may not exist in this network topology")
				} else if strings.Contains(err.Error(), "timeout") {
					t.Logf("Timeout suggests network complexity or API performance issues")
				} else {
					t.Logf("Other error type: %v", err)
				}
				return
			}

			if response == nil {
				t.Fatal("Expected response, got nil")
			}

			if response.Text == "" {
				t.Fatal("Expected content in response")
			}

			content := response.Text
			t.Logf("Response for %s:\n%s", tc.name, content)

			// Validate response structure
			if !strings.Contains(content, "path search completed") {
				t.Errorf("Response doesn't contain expected completion message")
			}

			requirePathsFound(t, content, tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)

			// Additional validation for specific test cases
			switch tc.name {
			case "With_Functions":
				if !strings.Contains(strings.ToLower(content), "network") {
					t.Logf("Network functions may not be included or available")
				}
			case "TCP_80":
				// Could check for protocol-specific information if available
				t.Logf("TCP port 80 specific test completed")
			}
		})
	}
}

// Integration test helper to validate path search response structure
func TestIntegrationPathSearchResponseStructure(t *testing.T) {
	service := setupIntegrationTest(t)

	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for path search structure test")
	}

	networkID := getTestNetworkID(networks)
	srcIP, dstIP := getTestPathIPs()

	args := SearchPathsBulkArgs{
		NetworkID: networkID,
		Queries: []PathSearchQueryArgs{
			{
				SrcIP: srcIP,
				DstIP: dstIP,
			},
		},
		MaxResults: 1,
	}

	t.Logf("Testing path search response structure on network %s", networkID)

	// Test that the response has the expected structure even if no paths are found
	response, err := service.searchPathsBulk(context.Background(), args)
	if err != nil {
		skipIfNoAccess(t, networkID, err)
		t.Logf("Path search failed: %v", err)
		// Test that errors are properly formatted
		if !strings.Contains(err.Error(), "failed to search paths") {
			t.Errorf("Error doesn't have expected format: %v", err)
		}
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Text == "" {
		t.Fatal("Expected text content, got none")
	}

	content := response.Text
	if content == "" {
		t.Fatal("Expected non-empty content")
	}

	// Validate that the response contains expected elements
	expectedElements := []string{
		"path search completed",
		"found",
		"paths",
	}

	for _, element := range expectedElements {
		if !strings.Contains(content, element) {
			t.Errorf("Response missing expected element: %s", element)
		}
	}
	requirePathsFound(t, content, srcIP, dstIP)

	t.Logf("✅ Path search response structure is valid")
}

// Integration test for running NQE query with real API (if networks exist)
func TestIntegrationRunNQEQuery(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for NQE query test")
	}

	// Use the first network for testing
	networkID := getTestNetworkID(networks)

	// Use a known executable query ID - Device Basic Info
	args := RunNQEQueryByIDArgs{
		NetworkID: networkID,
		QueryID:   "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029", // Device Basic Info
		Options: &NQEQueryOptions{
			Limit: 5,
		},
	}

	response, err := service.RunNQEQueryByID(context.Background(), args)
	if err != nil {
		// NQE query might fail if no devices exist or query is invalid, which is OK for testing
		t.Logf("NQE query failed (this may be expected): %v", err)
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Text
	t.Logf("NQE query response: %s", content)
}

// Integration test for bulk path search between the test IPs (see getTestPathIPs)
func TestIntegrationPathSearchBulkVariants(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for path search test")
	}

	// Use the first network for testing
	networkID := getTestNetworkID(networks)
	srcIP, dstIP := getTestPathIPs()
	t.Logf("Testing path search on network %s", networkID)

	// Test scenarios between the test IPs
	testCases := []struct {
		name        string
		args        SearchPathsBulkArgs
		expectError bool
		description string
	}{
		{
			name: "Basic",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Basic path search with the test source and destination IPs",
		},
		{
			name: "With_Intent_PREFER_DELIVERED",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				Intent:     "PREFER_DELIVERED",
				MaxResults: 10,
			},
			expectError: false,
			description: "Path search with PREFER_DELIVERED intent to prioritize successful paths",
		},
		{
			name: "With_Functions",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				IncludeNetworkFunctions: true,
				MaxResults:              3,
			},
			expectError: false,
			description: "Path search including network functions for detailed hop-by-hop analysis",
		},
		{
			name: "TCP_443_HTTPS",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP:   srcIP,
						DstIP:   dstIP,
						IPProto: &[]int{6}[0], // TCP
						DstPort: "443",
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Path search for HTTPS traffic (TCP port 443) between the specific IPs",
		},
		{
			name: "TCP_80_HTTP",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP:   srcIP,
						DstIP:   dstIP,
						IPProto: &[]int{6}[0], // TCP
						DstPort: "80",
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Path search for HTTP traffic (TCP port 80) between the specific IPs",
		},
		{
			name: "Reverse_Path",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: dstIP, // Reverse direction
						DstIP: srcIP,
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Reverse path search to test bidirectional connectivity",
		},
		{
			name: "PREFER_VIOLATIONS",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: srcIP,
						DstIP: dstIP,
					},
				},
				Intent:     "PREFER_VIOLATIONS",
				MaxResults: 5,
			},
			expectError: false,
			description: "Path search with PREFER_VIOLATIONS to identify policy violations",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("🔍 Testing: %s", tc.description)
			t.Logf("   Source IP: %s → Destination IP: %s", tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)

			response, err := service.searchPathsBulk(context.Background(), tc.args)

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !tc.expectError && err != nil {
				// For path search, some failures are expected if the network doesn't contain these specific IPs
				t.Logf("⚠️  Path search failed (may be expected for this network): %v", err)

				// Log the specific error for debugging
				if strings.Contains(err.Error(), "404") {
					t.Logf("   💡 404 error suggests the IPs may not exist in this network topology")
				} else if strings.Contains(err.Error(), "timeout") {
					t.Logf("   💡 Timeout suggests network complexity or API performance issues")
				} else if strings.Contains(err.Error(), "400") {
					t.Logf("   💡 400 error suggests invalid request parameters")
				} else {
					t.Logf("   💡 Other error type: %v", err)
				}
				return
			}

			if response == nil {
				t.Fatal("Expected response, got nil")
			}

			if response.Text == "" {
				t.Fatal("Expected content in response")
			}

			content := response.Text
			t.Logf("✅ Response for %s:", tc.name)

			// Log a shortened version for readability
			if len(content) > 500 {
				t.Logf("   %s... (truncated)", content[:500])
			} else {
				t.Logf("   %s", content)
			}

			// Validate response structure
			if !strings.Contains(content, "path search completed") {
				t.Errorf("Response doesn't contain expected completion message")
			}

			requirePathsFound(t, content, tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)
			if !strings.Contains(content, `"hops"`) {
				t.Errorf("Response has no hop information")
			}

			// Additional validation for specific test cases
			switch tc.name {
			case "With_Functions":
				if strings.Contains(strings.ToLower(content), "network") || strings.Contains(content, "functions") {
					t.Logf("   🔧 Network functions information included")
				}
			case "TCP_443_HTTPS":
				t.Logf("   🔒 HTTPS (TCP 443) specific test completed")
			case "TCP_80_HTTP":
				t.Logf("   🌐 HTTP (TCP 80) specific test completed")
			case "PREFER_VIOLATIONS":
				if strings.Contains(strings.ToLower(content), "violation") {
					t.Logf("   ⚠️  Policy violations detected in path")
				}
			}
		})
	}
}

// Integration test for listing devices with real API (if networks exist)
func TestIntegrationListDevices(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for device listing test")
	}

	// Use the first network for testing
	networkID := getTestNetworkID(networks)

	args := ListDevicesArgs{
		NetworkID: networkID,
		Limit:     5,
	}

	response, err := service.ListDevices(context.Background(), args)
	if err != nil {
		skipIfNoAccess(t, networkID, err)
		t.Fatalf("Failed to list devices: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Text
	t.Logf("Devices response: %s", content)
}

// Integration test for getting snapshots with real API (if networks exist)
func TestIntegrationListSnapshots(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for snapshot listing test")
	}

	// Use the first network for testing
	networkID := getTestNetworkID(networks)

	args := ListSnapshotsArgs{
		NetworkID: networkID,
	}

	response, err := service.ListSnapshots(context.Background(), args)
	if err != nil {
		skipIfNoAccess(t, networkID, err)
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Text
	t.Logf("Snapshots response: %s", content)
}

// Integration test for all_results: the full result set is saved to the
// memory system, and the response says so only because it was.
func TestIntegrationAllResultsStoredInMemory(t *testing.T) {
	service := setupIntegrationTest(t)

	// The memory store lives in a temp directory, never in ~/.forward-mcp.
	memory, err := sqlite.NewMemorySystemAt(filepath.Join(t.TempDir(), "memory.db"), service.logger, service.instanceID)
	if err != nil {
		t.Fatalf("memory store: %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	service.memorySystem = memory

	networks, err := service.forwardClient.GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}
	if len(networks) == 0 {
		t.Skip("No networks available for all_results test")
	}
	networkID := getTestNetworkID(networks)

	tests := []struct {
		name       string
		entityName string
		call       func() (*Result, error)
		want       string // must appear in the stored data
	}{
		{
			name:       "list_networks",
			entityName: "network_list",
			call: func() (*Result, error) {
				return service.ListNetworks(context.Background(), ListNetworksArgs{AllResults: true})
			},
			want: networkID,
		},
		{
			name:       "list_snapshots",
			entityName: "snapshot_list",
			call: func() (*Result, error) {
				return service.ListSnapshots(context.Background(), ListSnapshotsArgs{NetworkID: networkID, AllResults: true})
			},
			want: `"id"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, err := tc.call()
			if err != nil {
				skipIfNoAccess(t, networkID, err)
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !strings.Contains(response.Text, "Stored") {
				t.Errorf("response does not report the results as stored:\n%s", response.Text)
			}

			entity, err := memory.GetEntityByName(tc.entityName)
			if err != nil {
				t.Fatalf("no %s entity in memory: %v", tc.entityName, err)
			}
			observations, err := memory.GetObservations(entity.ID, "data")
			if err != nil {
				t.Fatalf("observations for %s: %v", tc.entityName, err)
			}
			if len(observations) != 1 {
				t.Fatalf("got %d data observations for %s, want 1", len(observations), tc.entityName)
			}
			if !strings.Contains(observations[0].Content, tc.want) {
				t.Errorf("stored %s data does not contain %s", tc.entityName, tc.want)
			}
		})
	}
}
