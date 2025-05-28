package main

import (
	"log"
	"os"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/service"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func main() {
	// Initialize configuration
	log.Println("Loading configuration...")
	cfg := config.LoadConfig()

	// Debug configuration
	log.Printf("Config loaded - API URL: %s", cfg.Forward.APIBaseURL)
	log.Printf("API Key present: %v", cfg.Forward.APIKey != "")
	log.Printf("TLS Skip Verify: %v", cfg.Forward.InsecureSkipVerify)

	// Create Forward MCP service
	log.Println("Creating Forward MCP service...")
	forwardService := service.NewForwardMCPService(cfg)

	// Create MCP server with stdio transport for Claude Desktop compatibility
	log.Println("Creating MCP server with stdio transport...")
	transport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(transport)

	// Register all Forward Networks tools
	log.Println("Registering Forward Networks tools...")
	if err := forwardService.RegisterTools(server); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}
	log.Println("Tools registered successfully!")

	// Check if we're in a TTY (interactive mode) or pipe mode
	if fileInfo, _ := os.Stdin.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		log.Println("Running in interactive mode (TTY detected)")
		log.Println("Server is ready and waiting for MCP protocol messages on stdin...")
		log.Println("Send MCP messages as JSON to interact with the server")
	} else {
		log.Println("Running in pipe mode (stdin redirected)")
	}

	// Channel to keep the server running
	done := make(chan struct{})

	// Start the MCP server
	log.Println("Starting Forward Networks MCP server...")
	if err := server.Serve(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("MCP server is now running and waiting for connections...")

	// Block forever to keep the server running
	<-done
}
