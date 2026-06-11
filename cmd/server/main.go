package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/instancelock"
	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverVersion is reported to MCP clients during the initialize handshake.
const serverVersion = "3.0.0"

const serverInstructions = "MCP server for Forward Networks: network discovery, NQE queries, " +
	"path searches, configuration search/diff, snapshots, locations, and a knowledge-graph memory. " +
	"Start with list_networks to find a network ID (or set_default_network to pin one). " +
	"Use search_nqe_queries to discover queries by natural language, then run_nqe_query_by_id to execute. " +
	"Prefer search_paths_bulk for path analysis."

func main() {
	// Initialize logger
	logger := logger.New()

	// Load configuration
	cfg := config.LoadConfig()

	// Create logger
	logger.Info("Forward MCP Server starting...")

	// Acquire instance lock to prevent multiple servers
	lockDir := os.Getenv("FORWARD_LOCK_DIR")
	if lockDir == "" {
		lockDir = "/tmp"
	}
	instanceLock := instancelock.NewInstanceLock(lockDir)

	// Check if another instance is already running
	if running, pid, err := instancelock.CheckRunningInstance(lockDir); running {
		logger.Fatalf("Another instance of Forward MCP Server is already running (PID: %d)", pid)
	} else if err != nil {
		logger.Error("Warning: Could not check for running instances: %v", err)
	}

	// Try to acquire the lock
	logger.Debug("Acquiring instance lock at: %s", instanceLock.GetLockFilePath())
	if err := instanceLock.Acquire(3, 500*time.Millisecond); err != nil {
		logger.Fatalf("Failed to acquire instance lock: %v\nAnother instance may be running or starting up.", err)
	}
	logger.Debug("Instance lock acquired successfully")

	// Ensure lock is released on exit
	defer func() {
		logger.Debug("Releasing instance lock...")
		if err := instanceLock.Release(); err != nil {
			logger.Error("Failed to release instance lock: %v", err)
		} else {
			logger.Debug("Instance lock released successfully")
		}
	}()

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

	// SECURITY: TLS 1.3 is now enforced, InsecureSkipVerify has been removed
	logger.Info("Environment initialized - TLS verification: enabled (TLS 1.3 minimum)")

	// Security: Do not log sensitive configuration details even in debug mode
	// Use INFO level logging above for configuration visibility

	// Create Forward MCP service
	logger.Debug("Creating Forward MCP service...")
	forwardService := service.NewForwardMCPService(cfg, logger)

	// Create MCP server (official go-sdk); stdio transport is attached in Run below.
	logger.Debug("Creating MCP server...")
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "forward-mcp",
		Title:   "Forward Networks MCP Server",
		Version: serverVersion,
	}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the server over stdio; Run blocks until the client disconnects
	// (stdin EOF) or the context is cancelled.
	logger.Debug("Starting Forward Networks MCP server...")
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Run(ctx, &mcp.StdioTransport{})
	}()

	logger.Debug("MCP server is now running and waiting for connections...")

	// Wait for client disconnect, server error, or shutdown signal.
	var runErr error
	select {
	case runErr = <-serverErr:
		if runErr != nil {
			logger.Error("Server error: %v", runErr)
		} else {
			logger.Info("Client disconnected, shutting down...")
		}
	case sig := <-shutdown:
		logger.Info("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}

	// Shutdown the ForwardMCPService to stop background goroutines and close databases.
	if err := forwardService.Shutdown(30 * time.Second); err != nil {
		logger.Error("Error during service shutdown: %v", err)
	}

	// Close logger file if it exists
	if err := logger.Close(); err != nil {
		logger.Error("Error closing logger: %v", err)
	}

	logger.Info("Server shutdown complete")
	if runErr != nil {
		os.Exit(1)
	}
}
