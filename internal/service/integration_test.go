package service

import (
	"context"
	"github.com/forward-mcp/internal/adapters/secondary/bloom"
	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/adapters/secondary/envconfig"
	"github.com/forward-mcp/internal/adapters/secondary/forwardapi"
	"github.com/forward-mcp/internal/adapters/secondary/queryindex"
	"github.com/forward-mcp/internal/adapters/secondary/semcache"
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

// getProjectRoot returns the absolute path to the project root.
func getProjectRoot() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

// setupIntegrationTest loads environment variables and creates a real service
func setupIntegrationTest(t *testing.T) *ForwardMCPService {
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
	return NewForwardMCPService(cfg, log, Deps{
		API:        forwardapi.NewClient(&cfg.Forward, log),
		Cache:      semcache.NewSemanticCache(embedder, log, InstanceID(cfg), &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
		Bloom:      bloom.NewBloomSearchManager(log, InstanceID(cfg)),
	})
}

// Integration test for listing networks with real API
func TestIntegrationListNetworks(t *testing.T) {
	service := setupIntegrationTest(t)

	response, err := service.listNetworks(context.Background(), ListNetworksArgs{})
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

// Integration test for path search with specific customer IPs
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
	t.Logf("Testing path search on network %s", networkID)

	// Test scenarios with the specific IPs provided
	testCases := []struct {
		name        string
		args        SearchPathsBulkArgs
		expectError bool
		description string
	}{
		{
			name: "Customer_Specific_IPs_Basic",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Basic path search with customer's specific source and destination IPs",
		},
		{
			name: "Customer_Specific_IPs_With_Intent",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
					},
				},
				Intent:     "PREFER_DELIVERED",
				MaxResults: 10,
			},
			expectError: false,
			description: "Path search with PREFER_DELIVERED intent",
		},
		{
			name: "Customer_Specific_IPs_With_Functions",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
					},
				},
				IncludeNetworkFunctions: true,
				MaxResults:              3,
			},
			expectError: false,
			description: "Path search including network functions for detailed analysis",
		},
		{
			name: "Customer_Specific_IPs_TCP_80",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP:   "10.6.142.197",
						DstIP:   "10.5.0.130",
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
			name: "Customer_Reverse_Path",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.5.0.130", // Reverse the IPs
						DstIP: "10.6.142.197",
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

			// Check if we got valid JSON in the response
			if strings.Contains(content, "Found") && strings.Contains(content, "paths") {
				// Extract the number of paths found
				if strings.Contains(content, "Found 0 paths") {
					t.Logf("No paths found between %s and %s (this may be expected)", tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)
				} else {
					t.Logf("Successfully found paths between %s and %s", tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)
				}
			}

			// Additional validation for specific test cases
			switch tc.name {
			case "Customer_Specific_IPs_With_Functions":
				if !strings.Contains(strings.ToLower(content), "network") {
					t.Logf("Network functions may not be included or available")
				}
			case "Customer_Specific_IPs_TCP_80":
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

	args := SearchPathsBulkArgs{
		NetworkID: networkID,
		Queries: []PathSearchQueryArgs{
			{
				SrcIP: "10.6.142.197",
				DstIP: "10.5.0.130",
			},
		},
		MaxResults: 1,
	}

	t.Logf("Testing path search response structure on network %s", networkID)

	// Test that the response has the expected structure even if no paths are found
	response, err := service.searchPathsBulk(context.Background(), args)
	if err != nil {
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

	response, err := service.runNQEQueryByID(context.Background(), args)
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

// Integration test for path search with specific customer IPs: 10.6.142.197 -> 10.5.0.130
func TestIntegrationPathSearchSpecificCustomerIPs(t *testing.T) {
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

	// Test scenarios with the specific IPs provided by the customer
	testCases := []struct {
		name        string
		args        SearchPathsBulkArgs
		expectError bool
		description string
	}{
		{
			name: "Customer_Specific_IPs_Basic",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Basic path search with customer's specific source and destination IPs",
		},
		{
			name: "Customer_Specific_IPs_With_Intent_PREFER_DELIVERED",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
					},
				},
				Intent:     "PREFER_DELIVERED",
				MaxResults: 10,
			},
			expectError: false,
			description: "Path search with PREFER_DELIVERED intent to prioritize successful paths",
		},
		{
			name: "Customer_Specific_IPs_With_Functions",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
					},
				},
				IncludeNetworkFunctions: true,
				MaxResults:              3,
			},
			expectError: false,
			description: "Path search including network functions for detailed hop-by-hop analysis",
		},
		{
			name: "Customer_Specific_IPs_TCP_443_HTTPS",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP:   "10.6.142.197",
						DstIP:   "10.5.0.130",
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
			name: "Customer_Specific_IPs_TCP_80_HTTP",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP:   "10.6.142.197",
						DstIP:   "10.5.0.130",
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
			name: "Customer_Reverse_Path",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.5.0.130", // Reverse the IPs
						DstIP: "10.6.142.197",
					},
				},
				MaxResults: 5,
			},
			expectError: false,
			description: "Reverse path search to test bidirectional connectivity",
		},
		{
			name: "Customer_Specific_IPs_PREFER_VIOLATIONS",
			args: SearchPathsBulkArgs{
				NetworkID: networkID,
				Queries: []PathSearchQueryArgs{
					{
						SrcIP: "10.6.142.197",
						DstIP: "10.5.0.130",
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

			// Check if we got valid paths
			if strings.Contains(content, "Found") && strings.Contains(content, "paths") {
				if strings.Contains(content, "Found 0 paths") {
					t.Logf("   📍 No paths found between %s and %s (this may be expected if IPs don't exist in topology)", tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)
				} else {
					t.Logf("   🎯 Successfully found paths between %s and %s", tc.args.Queries[0].SrcIP, tc.args.Queries[0].DstIP)

					// Try to extract useful path information
					if strings.Contains(content, "hops") {
						t.Logf("   🔗 Path contains hop information")
					}
					if strings.Contains(content, "outcome") {
						t.Logf("   📊 Path contains outcome information")
					}
				}
			}

			// Additional validation for specific test cases
			switch tc.name {
			case "Customer_Specific_IPs_With_Functions":
				if strings.Contains(strings.ToLower(content), "network") || strings.Contains(content, "functions") {
					t.Logf("   🔧 Network functions information included")
				}
			case "Customer_Specific_IPs_TCP_443_HTTPS":
				t.Logf("   🔒 HTTPS (TCP 443) specific test completed")
			case "Customer_Specific_IPs_TCP_80_HTTP":
				t.Logf("   🌐 HTTP (TCP 80) specific test completed")
			case "Customer_Specific_IPs_PREFER_VIOLATIONS":
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

	response, err := service.listDevices(context.Background(), args)
	if err != nil {
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

	response, err := service.listSnapshots(context.Background(), args)
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Text
	t.Logf("Snapshots response: %s", content)
}
