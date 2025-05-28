package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forward-mcp/internal/config"
	"github.com/joho/godotenv"
)

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
	cfg := config.LoadConfig()

	// Verify required credentials are set
	if cfg.Forward.APIKey == "" || cfg.Forward.APISecret == "" || cfg.Forward.APIBaseURL == "" {
		t.Skip("FORWARD_API_KEY, FORWARD_API_SECRET, and FORWARD_API_BASE_URL must be set to run integration tests")
	}

	// Set a longer timeout for integration tests
	if cfg.Forward.Timeout < 30 {
		cfg.Forward.Timeout = 30
	}

	return NewForwardMCPService(cfg)
}

// Integration test for listing networks with real API
func TestIntegrationListNetworks(t *testing.T) {
	service := setupIntegrationTest(t)

	response, err := service.listNetworks(ListNetworksArgs{})
	if err != nil {
		t.Fatalf("Failed to list networks: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if len(response.Content) != 1 {
		t.Fatalf("Expected 1 content item, got: %d", len(response.Content))
	}

	content := response.Content[0].TextContent.Text
	if content == "" {
		t.Fatal("Expected non-empty content")
	}

	t.Logf("Networks response: %s", content)
}

// Integration test for searching paths with real API (if networks exist)
func TestIntegrationSearchPaths(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks()
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for path search test")
	}

	// Use the first network for testing
	networkID := networks[0].ID

	args := SearchPathsArgs{
		NetworkID:  networkID,
		DstIP:      "8.8.8.8", // Use a common destination
		MaxResults: 1,
	}

	response, err := service.searchPaths(args)
	if err != nil {
		// Path search might fail if no valid paths exist, which is OK
		t.Logf("Path search failed (this may be expected): %v", err)
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	t.Logf("Path search response: %s", content)
}

// Integration test for running NQE query with real API (if networks exist)
func TestIntegrationRunNQEQuery(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks()
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for NQE query test")
	}

	// Use the first network for testing
	networkID := networks[0].ID

	args := RunNQEQueryArgs{
		NetworkID: networkID,
		Query:     "foreach device in network.devices select device.name",
		Limit:     5,
	}

	response, err := service.runNQEQuery(args)
	if err != nil {
		// NQE query might fail if no devices exist or query is invalid, which is OK for testing
		t.Logf("NQE query failed (this may be expected): %v", err)
		return
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	t.Logf("NQE query response: %s", content)
}

// Integration test for listing devices with real API (if networks exist)
func TestIntegrationListDevices(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks()
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for device listing test")
	}

	// Use the first network for testing
	networkID := networks[0].ID

	args := ListDevicesArgs{
		NetworkID: networkID,
		Limit:     5,
	}

	response, err := service.listDevices(args)
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	t.Logf("Devices response: %s", content)
}

// Integration test for getting snapshots with real API (if networks exist)
func TestIntegrationListSnapshots(t *testing.T) {
	service := setupIntegrationTest(t)

	// First get available networks
	networks, err := service.forwardClient.GetNetworks()
	if err != nil {
		t.Fatalf("Failed to get networks: %v", err)
	}

	if len(networks) == 0 {
		t.Skip("No networks available for snapshot listing test")
	}

	// Use the first network for testing
	networkID := networks[0].ID

	args := ListSnapshotsArgs{
		NetworkID: networkID,
	}

	response, err := service.listSnapshots(args)
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	t.Logf("Snapshots response: %s", content)
}
