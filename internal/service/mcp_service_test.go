package service

import (
	"strings"
	"testing"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/forward"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// MockForwardClient implements the ClientInterface for testing
type MockForwardClient struct {
	networks        []forward.Network
	devices         []forward.Device
	snapshots       []forward.Snapshot
	locations       []forward.Location
	nqeQueries      []forward.NQEQuery
	deviceLocations map[string]string
	pathResponse    *forward.PathSearchResponse
	nqeResult       *forward.NQERunResult
	shouldError     bool
	errorMessage    string
}

// NewMockForwardClient creates a new mock client with sample data
func NewMockForwardClient() *MockForwardClient {
	return &MockForwardClient{
		networks: []forward.Network{
			{
				ID:        "network-123",
				Name:      "Test Network",
				CreatedAt: 1745580296533,
				Creator:   "admin",
				OrgID:     "101",
			},
			{
				ID:        "network-456",
				Name:      "Production Network",
				CreatedAt: 1745950510200,
				Creator:   "admin",
				OrgID:     "101",
			},
		},
		devices: []forward.Device{
			{
				Name:          "router-1",
				Type:          "ROUTER",
				Hostname:      "rtr1.example.com",
				Platform:      "cisco_ios",
				Vendor:        "CISCO",
				Model:         "ISR4331",
				OSVersion:     "16.9.04",
				ManagementIPs: []string{"192.168.1.1"},
				LocationID:    "location-1",
			},
			{
				Name:          "switch-1",
				Type:          "SWITCH",
				Hostname:      "sw1.example.com",
				Platform:      "cisco_nxos",
				Vendor:        "CISCO",
				Model:         "N9K-C93180YC-EX",
				OSVersion:     "9.3(5)",
				ManagementIPs: []string{"192.168.1.2"},
				LocationID:    "location-2",
			},
		},
		snapshots: []forward.Snapshot{
			{
				ID:                 "snapshot-123",
				NetworkID:          "network-123",
				State:              "PROCESSED",
				ProcessingTrigger:  "REPROCESS",
				TotalDevices:       1232,
				TotalEndpoints:     56,
				CreationDateMillis: 1740478621913,
				ProcessedAtMillis:  1745953554303,
				IsDraft:            false,
			},
		},
		locations: []forward.Location{
			{
				ID:          "location-1",
				Name:        "Data Center 1",
				Description: "Primary data center",
			},
			{
				ID:          "location-2",
				Name:        "Data Center 2",
				Description: "Secondary data center",
			},
		},
		nqeQueries: []forward.NQEQuery{
			{
				ID:        "query-123",
				Name:      "All Devices",
				Directory: "/L3/Basic/",
				Query:     "foreach device in network.devices select device.name",
			},
		},
		deviceLocations: map[string]string{
			"router-1": "location-1",
			"switch-1": "location-2",
		},
		pathResponse: &forward.PathSearchResponse{
			Paths: []forward.Path{
				{
					Hops: []forward.Hop{
						{
							Device: "router-1",
							Action: "forward",
						},
						{
							Device: "switch-1",
							Action: "deliver",
						},
					},
					Outcome:     "delivered",
					OutcomeType: "success",
				},
			},
			SnapshotID:         "snapshot-123",
			SearchTimeMs:       100,
			NumCandidatesFound: 1,
		},
		nqeResult: &forward.NQERunResult{
			SnapshotID: "snapshot-123",
			Items: []map[string]interface{}{
				{"device_name": "router-1", "platform": "Cisco IOS"},
				{"device_name": "switch-1", "platform": "Cisco NX-OS"},
			},
		},
	}
}

// SetError configures the mock to return an error
func (m *MockForwardClient) SetError(shouldError bool, message string) {
	m.shouldError = shouldError
	m.errorMessage = message
}

// Mock implementations of ClientInterface methods
func (m *MockForwardClient) SendChatRequest(req *forward.ChatRequest) (*forward.ChatResponse, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return &forward.ChatResponse{Response: "Mock response", Model: "test-model"}, nil
}

func (m *MockForwardClient) GetAvailableModels() ([]string, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return []string{"model-1", "model-2"}, nil
}

func (m *MockForwardClient) GetNetworks() ([]forward.Network, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.networks, nil
}

func (m *MockForwardClient) CreateNetwork(name string) (*forward.Network, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	newNetwork := forward.Network{
		ID:   "new-network-id",
		Name: name,
	}
	m.networks = append(m.networks, newNetwork)
	return &newNetwork, nil
}

func (m *MockForwardClient) DeleteNetwork(networkID string) (*forward.Network, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	for i, network := range m.networks {
		if network.ID == networkID {
			deleted := m.networks[i]
			m.networks = append(m.networks[:i], m.networks[i+1:]...)
			return &deleted, nil
		}
	}
	return nil, &MockError{"network not found"}
}

func (m *MockForwardClient) UpdateNetwork(networkID string, update *forward.NetworkUpdate) (*forward.Network, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	for i := range m.networks {
		if m.networks[i].ID == networkID {
			if update.Name != nil {
				m.networks[i].Name = *update.Name
			}
			if update.Description != nil {
				m.networks[i].Description = *update.Description
			}
			return &m.networks[i], nil
		}
	}
	return nil, &MockError{"network not found"}
}

func (m *MockForwardClient) SearchPaths(networkID string, params *forward.PathSearchParams) (*forward.PathSearchResponse, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.pathResponse, nil
}

func (m *MockForwardClient) SearchPathsBulk(networkID string, requests []forward.PathSearchParams) ([]forward.PathSearchResponse, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	responses := make([]forward.PathSearchResponse, len(requests))
	for i := range responses {
		responses[i] = *m.pathResponse
	}
	return responses, nil
}

func (m *MockForwardClient) RunNQEQuery(params *forward.NQEQueryParams) (*forward.NQERunResult, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.nqeResult, nil
}

func (m *MockForwardClient) GetNQEQueries(dir string) ([]forward.NQEQuery, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.nqeQueries, nil
}

func (m *MockForwardClient) DiffNQEQuery(before, after string, request *forward.NQEDiffRequest) (*forward.NQEDiffResult, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return &forward.NQEDiffResult{TotalNumValues: 2, Rows: []map[string]interface{}{{"diff": "example"}}}, nil
}

func (m *MockForwardClient) GetDevices(networkID string, params *forward.DeviceQueryParams) (*forward.DeviceResponse, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return &forward.DeviceResponse{
		Devices:    m.devices,
		TotalCount: len(m.devices),
	}, nil
}

func (m *MockForwardClient) GetDeviceLocations(networkID string) (map[string]string, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.deviceLocations, nil
}

func (m *MockForwardClient) UpdateDeviceLocations(networkID string, locations map[string]string) error {
	if m.shouldError {
		return &MockError{m.errorMessage}
	}
	m.deviceLocations = locations
	return nil
}

func (m *MockForwardClient) GetSnapshots(networkID string) ([]forward.Snapshot, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.snapshots, nil
}

func (m *MockForwardClient) GetLatestSnapshot(networkID string) (*forward.Snapshot, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	if len(m.snapshots) > 0 {
		return &m.snapshots[0], nil
	}
	return nil, &MockError{"no snapshots found"}
}

func (m *MockForwardClient) DeleteSnapshot(snapshotID string) error {
	if m.shouldError {
		return &MockError{m.errorMessage}
	}
	return nil
}

func (m *MockForwardClient) GetLocations(networkID string) ([]forward.Location, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	return m.locations, nil
}

func (m *MockForwardClient) CreateLocation(networkID string, location *forward.LocationCreate) (*forward.Location, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	newLocation := forward.Location{
		ID:          "new-location-id",
		Name:        location.Name,
		Description: location.Description,
		Latitude:    location.Latitude,
		Longitude:   location.Longitude,
	}
	m.locations = append(m.locations, newLocation)
	return &newLocation, nil
}

func (m *MockForwardClient) UpdateLocation(networkID string, locationID string, update *forward.LocationUpdate) (*forward.Location, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	for i := range m.locations {
		if m.locations[i].ID == locationID {
			if update.Name != nil {
				m.locations[i].Name = *update.Name
			}
			if update.Description != nil {
				m.locations[i].Description = *update.Description
			}
			return &m.locations[i], nil
		}
	}
	return nil, &MockError{"location not found"}
}

func (m *MockForwardClient) DeleteLocation(networkID string, locationID string) (*forward.Location, error) {
	if m.shouldError {
		return nil, &MockError{m.errorMessage}
	}
	for i, location := range m.locations {
		if location.ID == locationID {
			deleted := m.locations[i]
			m.locations = append(m.locations[:i], m.locations[i+1:]...)
			return &deleted, nil
		}
	}
	return nil, &MockError{"location not found"}
}

// MockError implements the error interface
type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}

// Test helper to create a service with mock client
func createTestService() *ForwardMCPService {
	cfg := &config.Config{
		Forward: config.ForwardConfig{
			APIKey:     "test-key",
			APISecret:  "test-secret",
			APIBaseURL: "https://test.example.com",
			Timeout:    10,
		},
	}

	service := &ForwardMCPService{
		forwardClient: NewMockForwardClient(),
		config:        cfg,
	}

	return service
}

// Network Management Tests
func TestListNetworks(t *testing.T) {
	service := createTestService()

	response, err := service.listNetworks(ListNetworksArgs{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
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

	// Verify the response contains network information
	if !contains(content, "Test Network") {
		t.Error("Expected response to contain 'Test Network'")
	}
}

func TestCreateNetwork(t *testing.T) {
	service := createTestService()

	args := CreateNetworkArgs{
		Name: "New Test Network",
	}

	response, err := service.createNetwork(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "New Test Network") {
		t.Error("Expected response to contain new network name")
	}
}

func TestDeleteNetwork(t *testing.T) {
	service := createTestService()

	args := DeleteNetworkArgs{
		NetworkID: "network-123",
	}

	response, err := service.deleteNetwork(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "deleted successfully") {
		t.Error("Expected response to indicate successful deletion")
	}
}

// Path Search Tests
func TestSearchPaths(t *testing.T) {
	service := createTestService()

	args := SearchPathsArgs{
		NetworkID:  "network-123",
		DstIP:      "10.0.0.100",
		SrcIP:      "10.0.0.1",
		Intent:     "PREFER_DELIVERED",
		MaxResults: 5,
	}

	response, err := service.searchPaths(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Path search completed") {
		t.Error("Expected response to indicate path search completion")
	}
}

// NQE Tests
func TestRunNQEQuery(t *testing.T) {
	service := createTestService()

	args := RunNQEQueryArgs{
		NetworkID: "network-123",
		Query:     "foreach device in network.devices select device.name",
		Limit:     10,
	}

	response, err := service.runNQEQuery(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "NQE query completed") {
		t.Error("Expected response to indicate NQE query completion")
	}
}

func TestListNQEQueries(t *testing.T) {
	service := createTestService()

	args := ListNQEQueriesArgs{
		Directory: "/L3/Basic/",
	}

	response, err := service.listNQEQueries(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "All Devices") {
		t.Error("Expected response to contain query names")
	}
}

// Device Management Tests
func TestListDevices(t *testing.T) {
	service := createTestService()

	args := ListDevicesArgs{
		NetworkID: "network-123",
		Limit:     10,
	}

	response, err := service.listDevices(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "router-1") {
		t.Error("Expected response to contain device names")
	}
}

func TestGetDeviceLocations(t *testing.T) {
	service := createTestService()

	args := GetDeviceLocationsArgs{
		NetworkID: "network-123",
	}

	response, err := service.getDeviceLocations(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Device locations") {
		t.Error("Expected response to contain device location information")
	}
}

// Error Handling Tests
func TestErrorHandling(t *testing.T) {
	service := createTestService()
	mockClient := service.forwardClient.(*MockForwardClient)

	// Test error in listNetworks
	mockClient.SetError(true, "API connection failed")

	_, err := service.listNetworks(ListNetworksArgs{})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !contains(err.Error(), "failed to list networks") {
		t.Error("Expected error message to indicate network listing failure")
	}
}

// Integration test with mcp-golang
func TestMCPIntegration(t *testing.T) {
	// Create a test config
	cfg := &config.Config{
		Forward: config.ForwardConfig{
			APIKey:     "test-key",
			APISecret:  "test-secret",
			APIBaseURL: "https://test.example.com",
			Timeout:    10,
		},
	}

	// Create service with mock client
	service := &ForwardMCPService{
		forwardClient: NewMockForwardClient(),
		config:        cfg,
	}

	// Create MCP server
	transport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(transport)

	// Register tools
	err := service.RegisterTools(server)
	if err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	// Test that server was created successfully
	if server == nil {
		t.Fatal("Expected server to be created")
	}
}

// Comprehensive test for RegisterTools function
func TestRegisterToolsComprehensive(t *testing.T) {
	cfg := &config.Config{
		Forward: config.ForwardConfig{
			APIKey:     "test-key",
			APISecret:  "test-secret",
			APIBaseURL: "https://test.example.com",
			Timeout:    10,
		},
	}

	service := &ForwardMCPService{
		forwardClient: NewMockForwardClient(),
		config:        cfg,
	}

	// Create MCP server
	transport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(transport)

	// Test successful registration
	err := service.RegisterTools(server)
	if err != nil {
		t.Fatalf("Expected no error registering tools, got: %v", err)
	}

	// Test the individual tools exist (we can't directly test the internal registration
	// but we can test that the service methods work which indicates proper registration)
	testCases := []struct {
		name string
		test func() error
	}{
		{"list_networks", func() error {
			_, err := service.listNetworks(ListNetworksArgs{})
			return err
		}},
		{"create_network", func() error {
			_, err := service.createNetwork(CreateNetworkArgs{Name: "test"})
			return err
		}},
		{"update_network", func() error {
			_, err := service.updateNetwork(UpdateNetworkArgs{NetworkID: "network-123", Name: "updated"})
			return err
		}},
		{"search_paths", func() error {
			_, err := service.searchPaths(SearchPathsArgs{NetworkID: "network-123", DstIP: "10.0.0.1"})
			return err
		}},
		{"run_nqe_query", func() error {
			_, err := service.runNQEQuery(RunNQEQueryArgs{NetworkID: "network-123", Query: "test query"})
			return err
		}},
		{"list_nqe_queries", func() error {
			_, err := service.listNQEQueries(ListNQEQueriesArgs{})
			return err
		}},
		{"list_devices", func() error {
			_, err := service.listDevices(ListDevicesArgs{NetworkID: "network-123"})
			return err
		}},
		{"get_device_locations", func() error {
			_, err := service.getDeviceLocations(GetDeviceLocationsArgs{NetworkID: "network-123"})
			return err
		}},
		{"list_snapshots", func() error {
			_, err := service.listSnapshots(ListSnapshotsArgs{NetworkID: "network-123"})
			return err
		}},
		{"get_latest_snapshot", func() error {
			_, err := service.getLatestSnapshot(GetLatestSnapshotArgs{NetworkID: "network-123"})
			return err
		}},
		{"list_locations", func() error {
			_, err := service.listLocations(ListLocationsArgs{NetworkID: "network-123"})
			return err
		}},
		{"create_location", func() error {
			_, err := service.createLocation(CreateLocationArgs{NetworkID: "network-123", Name: "test location"})
			return err
		}},
	}

	// Test that all tool functions work (indicating they were registered properly)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.test()
			if err != nil {
				t.Fatalf("Tool %s failed: %v", tc.name, err)
			}
		})
	}

	// Test delete_network separately since it modifies state
	t.Run("delete_network", func(t *testing.T) {
		_, err := service.deleteNetwork(DeleteNetworkArgs{NetworkID: "network-456"})
		if err != nil {
			t.Fatalf("Tool delete_network failed: %v", err)
		}
	})
}

// Benchmark tests
func BenchmarkListNetworks(b *testing.B) {
	service := createTestService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.listNetworks(ListNetworksArgs{})
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkSearchPaths(b *testing.B) {
	service := createTestService()
	args := SearchPathsArgs{
		NetworkID: "network-123",
		DstIP:     "10.0.0.100",
		SrcIP:     "10.0.0.1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.searchPaths(args)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Test missing functions for better coverage
func TestUpdateNetwork(t *testing.T) {
	service := createTestService()

	args := UpdateNetworkArgs{
		NetworkID:   "network-123",
		Name:        "Updated Test Network",
		Description: "Updated description",
	}

	response, err := service.updateNetwork(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Network updated successfully") {
		t.Error("Expected response to indicate successful update")
	}

	if !contains(content, "Updated Test Network") {
		t.Error("Expected response to contain updated network name")
	}
}

func TestListSnapshots(t *testing.T) {
	service := createTestService()

	args := ListSnapshotsArgs{
		NetworkID: "network-123",
	}

	response, err := service.listSnapshots(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "snapshot") {
		t.Error("Expected response to contain snapshot information")
	}
}

func TestGetLatestSnapshot(t *testing.T) {
	service := createTestService()

	args := GetLatestSnapshotArgs{
		NetworkID: "network-123",
	}

	response, err := service.getLatestSnapshot(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Latest snapshot") {
		t.Error("Expected response to contain latest snapshot information")
	}
}

func TestListLocations(t *testing.T) {
	service := createTestService()

	args := ListLocationsArgs{
		NetworkID: "network-123",
	}

	response, err := service.listLocations(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Data Center") {
		t.Error("Expected response to contain location information")
	}
}

func TestCreateLocation(t *testing.T) {
	service := createTestService()

	lat := 37.7749
	lng := -122.4194
	args := CreateLocationArgs{
		NetworkID:   "network-123",
		Name:        "Test Data Center",
		Description: "A test data center location",
		Latitude:    &lat,
		Longitude:   &lng,
	}

	response, err := service.createLocation(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Location created successfully") {
		t.Error("Expected response to indicate successful creation")
	}

	if !contains(content, "Test Data Center") {
		t.Error("Expected response to contain new location name")
	}
}

// Test NewForwardMCPService for coverage
func TestNewForwardMCPService(t *testing.T) {
	cfg := &config.Config{
		Forward: config.ForwardConfig{
			APIKey:     "test-key",
			APISecret:  "test-secret",
			APIBaseURL: "https://test.example.com",
			Timeout:    10,
		},
	}

	service := NewForwardMCPService(cfg)
	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}

	if service.config != cfg {
		t.Error("Expected service config to match provided config")
	}

	if service.forwardClient == nil {
		t.Error("Expected forward client to be initialized")
	}
}

// Test TLS configuration options
func TestNewForwardMCPServiceWithTLS(t *testing.T) {
	cfg := &config.Config{
		Forward: config.ForwardConfig{
			APIKey:             "test-key",
			APISecret:          "test-secret",
			APIBaseURL:         "https://test.example.com",
			Timeout:            10,
			InsecureSkipVerify: true, // Test TLS skip verification
		},
	}

	service := NewForwardMCPService(cfg)
	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}

	if service.config != cfg {
		t.Error("Expected service config to match provided config")
	}

	if service.forwardClient == nil {
		t.Error("Expected forward client to be initialized")
	}

	// Note: We can't easily test the internal TLS configuration without exposing
	// the HTTP client, but we can verify the service is created successfully
	// with TLS options set
}

// Test edge cases for better coverage
func TestUpdateNetworkPartial(t *testing.T) {
	service := createTestService()

	// Test with only name update
	args := UpdateNetworkArgs{
		NetworkID: "network-123",
		Name:      "Only Name Updated",
	}

	response, err := service.updateNetwork(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Only Name Updated") {
		t.Error("Expected response to contain updated name")
	}
}

func TestSearchPathsWithIPProto(t *testing.T) {
	service := createTestService()

	args := SearchPathsArgs{
		NetworkID: "network-123",
		DstIP:     "10.0.0.100",
		SrcIP:     "10.0.0.1",
		IPProto:   6, // TCP
		SrcPort:   "80",
		DstPort:   "443",
	}

	response, err := service.searchPaths(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "Path search completed") {
		t.Error("Expected response to indicate path search completion")
	}
}

func TestRunNQEQueryWithOptions(t *testing.T) {
	service := createTestService()

	args := RunNQEQueryArgs{
		NetworkID: "network-123",
		Query:     "foreach device in network.devices select device.name",
		Limit:     5,
		Offset:    10,
	}

	response, err := service.runNQEQuery(args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	content := response.Content[0].TextContent.Text
	if !contains(content, "NQE query completed") {
		t.Error("Expected response to indicate NQE query completion")
	}
}
