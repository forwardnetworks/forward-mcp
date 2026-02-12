package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/instancelock"
	"github.com/forward-mcp/internal/logger"
	"github.com/forward-mcp/internal/service"
	ssetransport "github.com/forward-mcp/internal/transport/sse"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

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

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// registerMCPServer sets up tools, prompts, and resources on an MCP server
	registerMCPServer := func(server *mcp.Server) error {
		if err := forwardService.RegisterTools(server); err != nil {
			return fmt.Errorf("failed to register tools: %w", err)
		}
		if err := forwardService.RegisterPrompts(server); err != nil {
			return fmt.Errorf("failed to register prompts: %w", err)
		}
		if err := forwardService.RegisterResources(server); err != nil {
			return fmt.Errorf("failed to register resources: %w", err)
		}
		return nil
	}

	switch cfg.Server.Transport {
	case "sse":
		runSSEMode(cfg, logger, forwardService, registerMCPServer, shutdown)
	default:
		runStdioMode(logger, forwardService, registerMCPServer, shutdown)
	}
}

func runStdioMode(logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Close() error
}, forwardService *service.ForwardMCPService, registerMCPServer func(*mcp.Server) error, shutdown chan os.Signal) {
	// Create MCP server with stdio transport for Claude Desktop compatibility
	logger.Debug("Creating MCP server with stdio transport...")
	stdioTransport := stdio.NewStdioServerTransport()
	server := mcp.NewServer(stdioTransport)

	// Register all tools, prompts, resources
	logger.Debug("Registering Forward Networks tools...")
	if err := registerMCPServer(server); err != nil {
		logger.Fatalf("%v", err)
	}
	logger.Debug("Tools, prompts, and resources registered successfully!")

	// Check if we're in a TTY (interactive mode) or pipe mode
	if fileInfo, _ := os.Stdin.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		logger.Debug("Running in interactive mode (TTY detected)")
		logger.Debug("Server is ready and waiting for MCP protocol messages on stdin...")
		logger.Debug("Send MCP messages as JSON to interact with the server")
	} else {
		logger.Debug("Running in pipe mode (stdin redirected)")
	}

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

func runSSEMode(cfg *config.Config, logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Close() error
}, forwardService *service.ForwardMCPService, registerMCPServer func(*mcp.Server) error, shutdown chan os.Signal) {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("Starting MCP server in SSE mode on %s...", addr)

	// serviceFactory is called for each new SSE client session
	serviceFactory := func(tr transport.Transport) error {
		server := mcp.NewServer(tr)
		if err := registerMCPServer(server); err != nil {
			return err
		}
		// Serve starts the MCP protocol (connects transport, registers handlers)
		go server.Serve()
		return nil
	}

	sseServer := ssetransport.NewSSEServer(addr, "", serviceFactory)

	serverErr := make(chan error, 1)
	go func() {
		if err := sseServer.Start(); err != nil {
			serverErr <- err
		}
	}()

	logger.Info("SSE server is running on %s", addr)
	logger.Info("  SSE endpoint: GET /sse")
	logger.Info("  Message endpoint: POST /message?sessionId=<id>")

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		logger.Fatalf("Server error: %v", err)
	case sig := <-shutdown:
		logger.Info("Received signal %v, shutting down gracefully...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := sseServer.Shutdown(ctx); err != nil {
			logger.Error("Error during SSE server shutdown: %v", err)
		}

		if err := forwardService.Shutdown(30 * time.Second); err != nil {
			logger.Error("Error during service shutdown: %v", err)
		}

		if err := logger.Close(); err != nil {
			logger.Error("Error closing logger: %v", err)
		}

		logger.Info("Server shutdown complete")
		os.Exit(0)
	}
}
