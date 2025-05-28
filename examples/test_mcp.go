package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/service"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Create Forward MCP service
	forwardService := service.NewForwardMCPService(cfg)

	// Create MCP server with stdio transport
	transport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(transport)

	// Register all Forward Networks tools
	if err := forwardService.RegisterTools(server); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}

	// List all registered tools (for demonstration)
	fmt.Println("Forward Networks MCP Server")
	fmt.Println("===========================")
	fmt.Println("Registered tools:")

	// Note: In a real implementation, you would get this from the server
	// This is just for demonstration purposes
	tools := []string{
		"list_networks",
		"create_network",
		"delete_network",
		"update_network",
		"search_paths",
		"run_nqe_query",
		"list_nqe_queries",
		"list_devices",
		"get_device_locations",
		"list_snapshots",
		"get_latest_snapshot",
		"list_locations",
		"create_location",
	}

	for i, tool := range tools {
		fmt.Printf("%d. %s\n", i+1, tool)
	}

	fmt.Println("\nServer ready to accept MCP connections via stdio transport.")
	fmt.Println("Configure Claude Desktop to use this server for Forward Networks integration.")

	// Example tool argument structures
	fmt.Println("\nExample tool usage:")

	// Example: search_paths arguments
	searchArgs := service.SearchPathsArgs{
		NetworkID:  "network-123",
		DstIP:      "10.0.0.100",
		SrcIP:      "10.0.0.1",
		Intent:     "PREFER_DELIVERED",
		MaxResults: 5,
	}

	argsJSON, _ := json.MarshalIndent(searchArgs, "", "  ")
	fmt.Printf("\nsearch_paths example arguments:\n%s\n", string(argsJSON))

	// Example: run_nqe_query arguments
	nqeArgs := service.RunNQEQueryArgs{
		NetworkID: "network-123",
		Query:     "foreach device in network.devices select device.name, device.platform",
		Limit:     10,
	}

	nqeJSON, _ := json.MarshalIndent(nqeArgs, "", "  ")
	fmt.Printf("\nrun_nqe_query example arguments:\n%s\n", string(nqeJSON))
}
