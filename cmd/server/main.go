package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func main() {
	// Initialize logger
	logger := logger.New()

	// Load configuration
	cfg := config.LoadConfig()

	// Create logger
	logger.Info("Forward MCP Server starting...")

	// Log essential environment configuration at INFO level
	logger.Info("Environment initialized - API: %s", cfg.Forward.APIBaseURL)
	if cfg.Forward.APIKey != "" {
		logger.Info("Environment initialized - API credentials: configured")
	} else {
		logger.Info("Environment initialized - API credentials: missing")
	}

	if cfg.Forward.DefaultNetworkID != "" {
		logger.Info("Environment initialized - Default network: %s", cfg.Forward.DefaultNetworkID)
	} else {
		logger.Info("Environment initialized - Default network: not set")
	}

	if cfg.Forward.InsecureSkipVerify {
		logger.Info("Environment initialized - TLS verification: disabled")
	} else {
		logger.Info("Environment initialized - TLS verification: enabled")
	}

	logger.Debug("Config loaded - API URL: %s", cfg.Forward.APIBaseURL)
	logger.Debug("API Key present: %v", cfg.Forward.APIKey != "")
	logger.Debug("TLS Skip Verify: %v", cfg.Forward.InsecureSkipVerify)

	// Create Forward MCP service
	logger.Debug("Creating Forward MCP service...")
	forwardService := service.NewForwardMCPService(cfg, logger)

	// Create MCP server with stdio transport for Claude Desktop compatibility
	logger.Debug("Creating MCP server with stdio transport...")
	transport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(transport)

	// Register all Forward Networks tools
	logger.Debug("Registering Forward Networks tools...")
	if err := forwardService.RegisterTools(server); err != nil {
		logger.Fatalf("Failed to register tools: %v", err)
	}
	logger.Debug("Tools registered successfully!")

	// Register prompt workflows following MCP best practices
	logger.Debug("Registering prompt workflows...")
	if err := forwardService.RegisterPrompts(server); err != nil {
		logger.Fatalf("Failed to register prompts: %v", err)
	}
	logger.Debug("Prompt workflows registered successfully!")

	// Register contextual resources following MCP best practices
	logger.Debug("Registering contextual resources...")
	if err := forwardService.RegisterResources(server); err != nil {
		logger.Fatalf("Failed to register resources: %v", err)
	}
	logger.Debug("Contextual resources registered successfully!")

	// Check if we're in a TTY (interactive mode) or pipe mode
	if fileInfo, _ := os.Stdin.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		logger.Debug("Running in interactive mode (TTY detected)")
		logger.Debug("Server is ready and waiting for MCP protocol messages on stdin...")
		logger.Debug("Send MCP messages as JSON to interact with the server")
	} else {
		logger.Debug("Running in pipe mode (stdin redirected)")
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start the server in a goroutine
	logger.Debug("Starting Forward Networks MCP server...")
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Serve(); err != nil {
			serverErr <- err
		}
	}()

	logger.Debug("MCP server is now running and waiting for connections...")

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		logger.Fatalf("Server error: %v", err)
	case sig := <-shutdown:
		logger.Info("Received signal %v, shutting down gracefully...", sig)

		// Shutdown the ForwardMCPService first to stop background goroutines
		if err := forwardService.Shutdown(30 * time.Second); err != nil {
			logger.Error("Error during service shutdown: %v", err)
		}

		// Close logger file if it exists
		if err := logger.Close(); err != nil {
			logger.Error("Error closing logger: %v", err)
		}

		logger.Info("Server shutdown complete")
		os.Exit(0)
	}
}
