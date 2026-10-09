package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forward-mcp/internal/adapters/primary/mcpserver"
	"github.com/forward-mcp/internal/adapters/secondary/bloom"
	"github.com/forward-mcp/internal/adapters/secondary/embeddings"
	"github.com/forward-mcp/internal/adapters/secondary/envconfig"
	"github.com/forward-mcp/internal/adapters/secondary/forwardapi"
	"github.com/forward-mcp/internal/adapters/secondary/instancelock"
	"github.com/forward-mcp/internal/adapters/secondary/queryindex"
	"github.com/forward-mcp/internal/adapters/secondary/semcache"
	"github.com/forward-mcp/internal/adapters/secondary/sqlite"
	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
	"github.com/forward-mcp/internal/ports"
	"github.com/forward-mcp/internal/usecases"
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
	logger := stderrlog.New()

	// Load configuration
	cfg, err := envconfig.Load(logger)
	if err != nil {
		logger.Fatalf("Configuration error: %v", err)
	}

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
	forwardService := usecases.New(cfg, logger, newDeps(cfg, logger))

	// Create MCP server (official go-sdk); stdio transport is attached in Run below.
	logger.Debug("Creating MCP server...")
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "forward-mcp",
		Title:   "Forward Networks MCP Server",
		Version: serverVersion,
	}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})

	// Serve the use cases as MCP tools, prompts and a resource
	logger.Debug("Registering tools, prompts and resources...")
	if err := mcpserver.Register(server, forwardService, logger); err != nil {
		logger.Fatalf("Failed to register MCP capabilities: %v", err)
	}
	logger.Debug("Tools, prompts and resources registered")

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

// newDeps builds the adapters the service runs on.
func newDeps(cfg *ports.Config, log ports.Logger) usecases.Deps {
	instanceID := usecases.InstanceID(cfg)
	embedder := embeddings.New(cfg.Forward.SemanticCache.EmbeddingProvider, os.Getenv("OPENAI_API_KEY"), log)

	// A store that cannot open stays a nil interface: the service checks for
	// nil and runs without it. A nil *NQEDatabase in the interface would not
	// compare equal to nil.
	var queryStore ports.QueryStore
	if db, err := sqlite.NewNQEDatabase(log, instanceID); err != nil {
		log.Error("Failed to create database: %v", err)
	} else {
		queryStore = db
	}
	var memory ports.MemoryStore
	if m, err := sqlite.NewMemorySystem(log, instanceID); err != nil {
		log.Error("Failed to create memory system: %v", err)
	} else {
		memory = m
	}

	return usecases.Deps{
		API:        forwardapi.NewClient(&cfg.Forward, log),
		Cache:      semcache.NewSemanticCache(embedder, log, instanceID, &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
		QueryStore: queryStore,
		Memory:     memory,
		Bloom:      bloom.NewBloomSearchManager(log, instanceID),
	}
}
