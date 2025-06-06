package main

import (
	"encoding/json"
	"fmt"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	log := logger.New()

	// Create Forward MCP service
	forwardService := service.NewForwardMCPService(cfg, log)

	// Create MCP server with stdio transport
	transport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(transport)

	// Register all Forward Networks tools
	if err := forwardService.RegisterTools(server); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}

	// Register prompt workflows
	if err := forwardService.RegisterPrompts(server); err != nil {
		log.Fatalf("Failed to register prompts: %v", err)
	}

	// Register contextual resources
	if err := forwardService.RegisterResources(server); err != nil {
		log.Fatalf("Failed to register resources: %v", err)
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

	// Example: NQE Query Usage
	fmt.Println("\nNQE Query Examples:")
	fmt.Println("===================")
	fmt.Println("1. Running NQE Queries by String:")
	fmt.Println("   - Use this method when you want to execute a custom query")
	fmt.Println("   - The query should be in Forward Networks Query Language (NQL)")
	fmt.Println("   - Results can be formatted as JSON objects for better readability")
	fmt.Println("   - Common use cases: device information, interface details, routing tables")

	// Example 1: Basic device query
	nqeArgs1 := service.RunNQEQueryByStringArgs{
		NetworkID: "network-123",
		Query:     "foreach device in network.devices select {deviceName: device.name, platform: device.platform}",
		Options: &service.NQEQueryOptions{
			Limit: 10,
		},
	}

	nqeJSON1, _ := json.MarshalIndent(nqeArgs1, "", "  ")
	fmt.Printf("\nBasic device query example:\n%s\n", string(nqeJSON1))

	// Example 2: Interface query with filtering
	nqeArgs2 := service.RunNQEQueryByStringArgs{
		NetworkID: "network-123",
		Query:     "foreach interface in network.interfaces where interface.operStatus == 'up' select {deviceName: interface.device.name, interfaceName: interface.name, ipAddress: interface.ipv4Address}",
		Options: &service.NQEQueryOptions{
			Limit: 20,
		},
	}

	nqeJSON2, _ := json.MarshalIndent(nqeArgs2, "", "  ")
	fmt.Printf("\nInterface query example:\n%s\n", string(nqeJSON2))

	// Example 3: Routing table query
	nqeArgs3 := service.RunNQEQueryByStringArgs{
		NetworkID: "network-123",
		Query:     "foreach route in network.routes where route.protocol == 'ospf' select {deviceName: route.device.name, prefix: route.prefix, nextHop: route.nextHop, metric: route.metric}",
		Options: &service.NQEQueryOptions{
			Limit: 50,
		},
	}

	nqeJSON3, _ := json.MarshalIndent(nqeArgs3, "", "  ")
	fmt.Printf("\nRouting table query example:\n%s\n", string(nqeJSON3))

	fmt.Println("\n2. Running NQE Queries by ID:")
	fmt.Println("   - Use this method when you want to run a predefined query from the library")
	fmt.Println("   - First, list available queries using list_nqe_queries")
	fmt.Println("   - Then use the query ID to execute the query")
	fmt.Println("   - Common use cases: standard reports, compliance checks, network audits")

	// Example: List and run predefined query
	listArgs := service.ListNQEQueriesArgs{
		Directory: "/L3/Basic/",
	}

	listJSON, _ := json.MarshalIndent(listArgs, "", "  ")
	fmt.Printf("\nList NQE queries example:\n%s\n", string(listJSON))

	// Example: Run predefined query by ID
	nqeArgs4 := service.RunNQEQueryByIDArgs{
		NetworkID: "network-123",
		QueryID:   "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029",
		Options: &service.NQEQueryOptions{
			Limit: 10,
		},
	}

	nqeJSON4, _ := json.MarshalIndent(nqeArgs4, "", "  ")
	fmt.Printf("\nRun predefined query example:\n%s\n", string(nqeJSON4))

	fmt.Println("\nNQE Query Tips:")
	fmt.Println("1. Always use JSON object format in select statements for better readability")
	fmt.Println("2. Use where clauses to filter results")
	fmt.Println("3. Use limit and offset in options to paginate results")
	fmt.Println("4. Check the Forward Networks documentation for available query fields")
	fmt.Println("5. Use list_nqe_queries to discover predefined queries")
}
