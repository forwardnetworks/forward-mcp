package service

import (
	"encoding/json"
	"fmt"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/forward"
	mcp "github.com/metoro-io/mcp-golang"
)

// ForwardMCPService implements Forward Networks MCP tools using mcp-golang
type ForwardMCPService struct {
	forwardClient forward.ClientInterface
	config        *config.Config
}

// NewForwardMCPService creates a new Forward MCP service instance
func NewForwardMCPService(config *config.Config) *ForwardMCPService {
	return &ForwardMCPService{
		forwardClient: forward.NewClient(&config.Forward),
		config:        config,
	}
}

// RegisterTools registers all Forward Networks tools with the MCP server
func (s *ForwardMCPService) RegisterTools(server *mcp.Server) error {
	// Network Management Tools
	if err := server.RegisterTool("list_networks", "List all networks in the Forward platform", s.listNetworks); err != nil {
		return fmt.Errorf("failed to register list_networks tool: %w", err)
	}

	if err := server.RegisterTool("create_network", "Create a new network", s.createNetwork); err != nil {
		return fmt.Errorf("failed to register create_network tool: %w", err)
	}

	if err := server.RegisterTool("delete_network", "Delete a network", s.deleteNetwork); err != nil {
		return fmt.Errorf("failed to register delete_network tool: %w", err)
	}

	if err := server.RegisterTool("update_network", "Update a network's properties", s.updateNetwork); err != nil {
		return fmt.Errorf("failed to register update_network tool: %w", err)
	}

	// Path Search Tools
	if err := server.RegisterTool("search_paths", "Search for network paths by tracing packets through the network", s.searchPaths); err != nil {
		return fmt.Errorf("failed to register search_paths tool: %w", err)
	}

	// NQE Tools
	if err := server.RegisterTool("run_nqe_query", "Run a Network Query Engine (NQE) query on a snapshot", s.runNQEQuery); err != nil {
		return fmt.Errorf("failed to register run_nqe_query tool: %w", err)
	}

	if err := server.RegisterTool("list_nqe_queries", "List available NQE queries from the library", s.listNQEQueries); err != nil {
		return fmt.Errorf("failed to register list_nqe_queries tool: %w", err)
	}

	// Device Management Tools
	if err := server.RegisterTool("list_devices", "List devices in a network", s.listDevices); err != nil {
		return fmt.Errorf("failed to register list_devices tool: %w", err)
	}

	if err := server.RegisterTool("get_device_locations", "Get device location mappings", s.getDeviceLocations); err != nil {
		return fmt.Errorf("failed to register get_device_locations tool: %w", err)
	}

	// Snapshot Management Tools
	if err := server.RegisterTool("list_snapshots", "List snapshots for a network", s.listSnapshots); err != nil {
		return fmt.Errorf("failed to register list_snapshots tool: %w", err)
	}

	if err := server.RegisterTool("get_latest_snapshot", "Get the latest processed snapshot for a network", s.getLatestSnapshot); err != nil {
		return fmt.Errorf("failed to register get_latest_snapshot tool: %w", err)
	}

	// Location Management Tools
	if err := server.RegisterTool("list_locations", "List locations in a network", s.listLocations); err != nil {
		return fmt.Errorf("failed to register list_locations tool: %w", err)
	}

	if err := server.RegisterTool("create_location", "Create a new location", s.createLocation); err != nil {
		return fmt.Errorf("failed to register create_location tool: %w", err)
	}

	return nil
}

// Network Management Tool Implementations
func (s *ForwardMCPService) listNetworks(args ListNetworksArgs) (*mcp.ToolResponse, error) {
	networks, err := s.forwardClient.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	result, _ := json.MarshalIndent(networks, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d networks:\n%s", len(networks), string(result)))), nil
}

func (s *ForwardMCPService) createNetwork(args CreateNetworkArgs) (*mcp.ToolResponse, error) {
	network, err := s.forwardClient.CreateNetwork(args.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Network created successfully:\n%s", string(result)))), nil
}

func (s *ForwardMCPService) deleteNetwork(args DeleteNetworkArgs) (*mcp.ToolResponse, error) {
	network, err := s.forwardClient.DeleteNetwork(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Network deleted successfully:\n%s", string(result)))), nil
}

func (s *ForwardMCPService) updateNetwork(args UpdateNetworkArgs) (*mcp.ToolResponse, error) {
	update := &forward.NetworkUpdate{}
	if args.Name != "" {
		update.Name = &args.Name
	}
	if args.Description != "" {
		update.Description = &args.Description
	}

	network, err := s.forwardClient.UpdateNetwork(args.NetworkID, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Network updated successfully:\n%s", string(result)))), nil
}

// Path Search Tool Implementations
func (s *ForwardMCPService) searchPaths(args SearchPathsArgs) (*mcp.ToolResponse, error) {
	params := &forward.PathSearchParams{
		DstIP:                   args.DstIP,
		SrcIP:                   args.SrcIP,
		From:                    args.From,
		Intent:                  args.Intent,
		SrcPort:                 args.SrcPort,
		DstPort:                 args.DstPort,
		MaxResults:              args.MaxResults,
		IncludeNetworkFunctions: args.IncludeNetworkFunctions,
		SnapshotID:              args.SnapshotID,
	}

	if args.IPProto != 0 {
		params.IPProto = &args.IPProto
	}

	response, err := s.forwardClient.SearchPaths(args.NetworkID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to search paths: %w", err)
	}

	result, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Path search completed. Found %d paths:\n%s", len(response.Paths), string(result)))), nil
}

// NQE Tool Implementations
func (s *ForwardMCPService) runNQEQuery(args RunNQEQueryArgs) (*mcp.ToolResponse, error) {
	params := &forward.NQEQueryParams{
		NetworkID:  args.NetworkID,
		Query:      args.Query,
		QueryID:    args.QueryID,
		SnapshotID: args.SnapshotID,
	}

	if args.Limit > 0 || args.Offset > 0 {
		params.Options = &forward.NQEQueryOptions{
			Limit:  args.Limit,
			Offset: args.Offset,
		}
	}

	result, err := s.forwardClient.RunNQEQuery(params)
	if err != nil {
		return nil, fmt.Errorf("failed to run NQE query: %w", err)
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("NQE query completed. Found %d items:\n%s", len(result.Items), string(resultJSON)))), nil
}

func (s *ForwardMCPService) listNQEQueries(args ListNQEQueriesArgs) (*mcp.ToolResponse, error) {
	queries, err := s.forwardClient.GetNQEQueries(args.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list NQE queries: %w", err)
	}

	result, _ := json.MarshalIndent(queries, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d NQE queries:\n%s", len(queries), string(result)))), nil
}

// Device Management Tool Implementations
func (s *ForwardMCPService) listDevices(args ListDevicesArgs) (*mcp.ToolResponse, error) {
	params := &forward.DeviceQueryParams{
		SnapshotID: args.SnapshotID,
		Limit:      args.Limit,
		Offset:     args.Offset,
	}

	response, err := s.forwardClient.GetDevices(args.NetworkID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	result, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d devices (total: %d):\n%s", len(response.Devices), response.TotalCount, string(result)))), nil
}

func (s *ForwardMCPService) getDeviceLocations(args GetDeviceLocationsArgs) (*mcp.ToolResponse, error) {
	locations, err := s.forwardClient.GetDeviceLocations(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device locations: %w", err)
	}

	result, _ := json.MarshalIndent(locations, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Device locations:\n%s", string(result)))), nil
}

// Snapshot Management Tool Implementations
func (s *ForwardMCPService) listSnapshots(args ListSnapshotsArgs) (*mcp.ToolResponse, error) {
	snapshots, err := s.forwardClient.GetSnapshots(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	result, _ := json.MarshalIndent(snapshots, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d snapshots:\n%s", len(snapshots), string(result)))), nil
}

func (s *ForwardMCPService) getLatestSnapshot(args GetLatestSnapshotArgs) (*mcp.ToolResponse, error) {
	snapshot, err := s.forwardClient.GetLatestSnapshot(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	result, _ := json.MarshalIndent(snapshot, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Latest snapshot:\n%s", string(result)))), nil
}

// Location Management Tool Implementations
func (s *ForwardMCPService) listLocations(args ListLocationsArgs) (*mcp.ToolResponse, error) {
	locations, err := s.forwardClient.GetLocations(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}

	result, _ := json.MarshalIndent(locations, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d locations:\n%s", len(locations), string(result)))), nil
}

func (s *ForwardMCPService) createLocation(args CreateLocationArgs) (*mcp.ToolResponse, error) {
	location := &forward.LocationCreate{
		Name:        args.Name,
		Description: args.Description,
		Latitude:    args.Latitude,
		Longitude:   args.Longitude,
	}

	newLocation, err := s.forwardClient.CreateLocation(args.NetworkID, location)
	if err != nil {
		return nil, fmt.Errorf("failed to create location: %w", err)
	}

	result, _ := json.MarshalIndent(newLocation, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Location created successfully:\n%s", string(result)))), nil
}
