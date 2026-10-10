package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	forwardmcp "github.com/forward-mcp"
	"github.com/forward-mcp/internal/adapters/primary/httpserver"
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
	"golang.org/x/sync/errgroup"
)

// serverVersion is reported to MCP clients during the initialize handshake.
const serverVersion = ports.Version

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

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Determine which transport(s) to start
	httpEnabled := cfg.HTTP.Enabled
	stdinAvailable := true

	// Check if stdin is available (not a TTY means piped input)
	if fileInfo, _ := os.Stdin.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		// TTY mode - only start HTTP if enabled, skip stdio
		if httpEnabled {
			logger.Debug("Running in HTTP-only mode (TTY detected, HTTP enabled)")
			stdinAvailable = false
		} else {
			logger.Debug("Running in interactive mode (TTY detected)")
			logger.Debug("Server is ready and waiting for MCP protocol messages on stdin...")
			logger.Debug("Send MCP messages as JSON to interact with the server")
		}
	} else {
		logger.Debug("Running in pipe mode (stdin redirected)")
	}

	// Start server transports using errgroup for coordinated lifecycle
	g, gctx := errgroup.WithContext(ctx)

	// Start the HTTP server (Streamable HTTP + legacy SSE) if enabled
	var httpSrv *httpserver.Server
	if httpEnabled {
		httpSrv = httpserver.New(&cfg.HTTP, logger)
		if err := httpSrv.Start(gctx, server); err != nil {
			logger.Fatalf("Failed to start HTTP server: %v", err)
		}

		logger.Info("HTTP transport started on %s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		logger.Info("HTTP auth mode: %s", cfg.HTTP.AuthMode)
		if cfg.HTTP.TLSCertFile != "" {
			logger.Info("TLS enabled: https://%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		}

		// Add HTTP server to errgroup for coordinated shutdown
		g.Go(func() error {
			<-gctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()
			return httpSrv.Stop(shutdownCtx)
		})
	}

	// Start stdio server if available and not in HTTP-only mode
	var stdioDone chan error
	if stdinAvailable && !httpEnabled {
		logger.Debug("Starting stdio transport...")
		stdioDone = make(chan error, 1)
		g.Go(func() error {
			err := server.Run(gctx, &mcp.StdioTransport{})
			stdioDone <- err
			return err
		})
		logger.Debug("Stdio transport started")
	}

	if !httpEnabled && !stdinAvailable {
		logger.Fatalf("No transport available: HTTP disabled and stdin is a TTY. " +
			"Either enable HTTP with FORWARD_HTTP_ENABLED=true or pipe data to stdin.")
	}

	logger.Info("Forward Networks MCP server is running...")
	if httpEnabled {
		logger.Info("  - Streamable HTTP: %s:%d%s (legacy SSE: %s)", cfg.HTTP.Host, cfg.HTTP.Port, httpserver.MCPPath, httpserver.LegacySSEPath)
		logger.Info("  - Health: http://%s:%d/health", cfg.HTTP.Host, cfg.HTTP.Port)
	}
	if stdinAvailable && !httpEnabled {
		logger.Info("  - stdio: waiting for MCP messages")
	}

	// Wait for shutdown signal or server error
	select {
	case <-shutdown:
		logger.Info("Received shutdown signal, stopping gracefully...")
		cancel()
	case err := <-stdioDone:
		if err != nil {
			logger.Error("Stdio server error: %v", err)
		} else {
			logger.Info("Client disconnected, shutting down...")
		}
		cancel()
	}

	// Wait for all servers to stop (with timeout)
	done := make(chan error)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Error("Server shutdown error: %v", err)
		}
	case <-time.After(35 * time.Second):
		logger.Error("Server shutdown timeout exceeded")
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

	deps := usecases.Deps{
		API:        forwardapi.NewClient(&cfg.Forward, log),
		Cache:      semcache.NewSemanticCache(embedder, log, instanceID, &cfg.Forward.SemanticCache),
		QueryIndex: queryindex.NewNQEQueryIndex(embedder, log),
		QueryStore: queryStore,
		Memory:     memory,
		Bloom:      bloom.NewBloomSearchManager(log, instanceID),
		Rows:       sqlite.RowQuerier{},
		Skills:     forwardmcp.Skills(),
	}

	// Auto-hydrate database and embeddings on first run (background, non-blocking)
	if queryStore != nil {
		go autoHydrateDatabase(queryStore, deps.API, deps.QueryIndex, log)
	}

	return deps
}

// autoHydrateDatabase checks if the query database is sparse and loads from the API if needed.
// Also auto-generates keyword embeddings if they don't exist.
// Runs in the background without blocking server startup.
func autoHydrateDatabase(queryStore ports.QueryStore, api ports.ForwardAPI, queryIndex ports.QueryIndex, log ports.Logger) {
	if queryStore == nil {
		return
	}

	// Create background context with timeout for the hydration operation
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Check current database state
	queries, err := queryStore.LoadQueries()
	queryCount := 0
	if err == nil {
		queryCount = len(queries)
	}

	// Hydrate if database is sparse (< 100 queries)
	if err != nil || queryCount < 100 {
		log.Info("Auto-hydrating query database: found %d queries, fetching from API...", queryCount)

		// Fetch fresh queries from API (both org and fwd queries)
		freshQueries, err := api.GetNQEAllQueriesEnhanced(ctx, nil)
		if err != nil {
			log.Error("Auto-hydration failed: %v", err)
			return
		}

		// Save to database
		if err := queryStore.SaveQueries(freshQueries); err != nil {
			log.Error("Auto-hydration: failed to save queries: %v", err)
			return
		}

		log.Info("Auto-hydration complete: loaded and saved %d queries", len(freshQueries))
		queries = freshQueries
	} else {
		log.Debug("Database has %d queries, skipping auto-hydration", queryCount)
	}

	// Auto-generate embeddings if they don't exist
	if queryIndex != nil && len(queries) > 0 {
		log.Info("Initializing query index with %d queries...", len(queries))

		// Load queries into the index
		if err := queryIndex.LoadFromQueries(queries); err != nil {
			log.Error("Failed to load queries into index: %v", err)
			return
		}

		// Generate embeddings for queries that don't have them
		// (uses keyword provider by default - free, fast, no API key needed)
		log.Info("Generating keyword embeddings for queries without embeddings...")
		if err := queryIndex.GenerateEmbeddings(); err != nil {
			log.Error("Failed to generate embeddings: %v", err)
			return
		}

		log.Info("Query index initialized successfully")
	}
}
