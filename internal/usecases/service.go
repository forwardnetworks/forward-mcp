package usecases

import (
	"context"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/forward-mcp/internal/domain"
	"github.com/forward-mcp/internal/ports"
	"golang.org/x/sync/errgroup"
)

// WorkflowState represents the current state of a user workflow
type WorkflowState struct {
	CurrentStep   string                 `json:"current_step"`
	Parameters    map[string]interface{} `json:"parameters"`
	SelectedQuery string                 `json:"selected_query"`
	NetworkID     string                 `json:"network_id"`
	SnapshotID    string                 `json:"snapshot_id"`
}

// WorkflowManager manages user workflow states with automatic cleanup
type WorkflowManager struct {
	sessions     map[string]*WorkflowState
	lastAccessed map[string]time.Time
	mutex        sync.RWMutex
	maxSessions  int
	sessionTTL   time.Duration
	stopCleanup  chan struct{}
}

// NewWorkflowManager creates a new workflow manager with automatic session cleanup
func NewWorkflowManager(maxSessions int, sessionTTL time.Duration) *WorkflowManager {
	wm := &WorkflowManager{
		sessions:     make(map[string]*WorkflowState),
		lastAccessed: make(map[string]time.Time),
		maxSessions:  maxSessions,
		sessionTTL:   sessionTTL,
		stopCleanup:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go wm.cleanupLoop()

	return wm
}

// GetState gets the workflow state for a session and updates last access time
func (wm *WorkflowManager) GetState(sessionID string) *WorkflowState {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// Update last accessed time
	wm.lastAccessed[sessionID] = time.Now()

	if state, exists := wm.sessions[sessionID]; exists {
		return state
	}
	return &WorkflowState{
		CurrentStep: "start",
		Parameters:  make(map[string]interface{}),
	}
}

// SetState sets the workflow state for a session
func (wm *WorkflowManager) SetState(sessionID string, state *WorkflowState) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	wm.sessions[sessionID] = state
	wm.lastAccessed[sessionID] = time.Now()
}

// cleanupLoop runs periodically to clean up stale sessions
func (wm *WorkflowManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wm.cleanup()
		case <-wm.stopCleanup:
			return
		}
	}
}

// cleanup removes sessions that have exceeded their TTL
func (wm *WorkflowManager) cleanup() {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	now := time.Now()
	for sessionID, lastAccess := range wm.lastAccessed {
		if now.Sub(lastAccess) > wm.sessionTTL {
			delete(wm.sessions, sessionID)
			delete(wm.lastAccessed, sessionID)
		}
	}
}

// Close stops the cleanup goroutine
func (wm *WorkflowManager) Close() {
	close(wm.stopCleanup)
}

// Service implements Forward Networks MCP tools using mcp-golang
type Service struct {
	forwardClient   ports.ForwardAPI
	config          *domain.Config
	logger          ports.Logger
	instanceID      string // Unique identifier for this Forward Networks instance
	defaults        *ServiceDefaults
	networks        *sessionNetworks // default network per MCP session
	workflowManager *WorkflowManager
	semanticCache   ports.ResultCache
	queryIndex      ports.QueryIndex
	database        ports.QueryStore
	memorySystem    ports.MemoryStore  // Knowledge graph memory system
	apiTracker      *APIMemoryTracker  // API result tracking using memory system
	bloomManager    ports.BloomFilters // Bloom filters for efficient large result filtering
	rowQuerier      ports.RowQuerier
	skills          fs.FS // files of the MCP Skills extension; nil serves none
	// Context cancellation for graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// ServiceDefaults holds default values for the MCP service
type ServiceDefaults struct {
	SnapshotID string
	QueryLimit int
}

// Deps are the outside capabilities the service is built from. The caller
// chooses each adapter.
type Deps struct {
	API        ports.ForwardAPI
	Cache      ports.ResultCache
	QueryIndex ports.QueryIndex
	// QueryStore and Memory may be nil when their database cannot be opened;
	// the service then runs without them. Pass a nil interface, never a nil
	// pointer of a concrete type.
	QueryStore ports.QueryStore
	Memory     ports.MemoryStore
	Bloom      ports.BloomFilters
	Rows       ports.RowQuerier
	// Skills holds the files served over the MCP Skills extension: one
	// directory per skill, each with a SKILL.md. Nil serves no skills.
	Skills fs.FS
}

// New creates the service from its configuration and deps.
func New(cfg *domain.Config, logger ports.Logger, deps Deps) *Service {
	// Use configured instance ID or generate one based on API URL
	instanceID := cfg.Forward.InstanceID
	if instanceID == "" {
		instanceID = GenerateInstanceID(cfg.Forward.APIBaseURL)
		logger.Info("Using generated instance ID '%s' for partitioning (based on %s)", instanceID, cfg.Forward.APIBaseURL)
	} else {
		logger.Info("Using configured instance ID '%s' for partitioning", instanceID)
	}

	forwardClient := deps.API

	semanticCache := deps.Cache

	database := deps.QueryStore

	queryIndex := deps.QueryIndex

	memorySystem := deps.Memory

	// Create API memory tracker
	var apiTracker *APIMemoryTracker
	if memorySystem != nil {
		apiTracker = NewAPIMemoryTracker(memorySystem, logger, instanceID)
		logger.Info("API memory tracker initialized for tracking API results and relationships")
	}

	bloomManager := deps.Bloom

	// Create context for cancellation
	ctx, cancelFunc := context.WithCancel(context.Background())

	service := &Service{
		forwardClient: forwardClient,
		config:        cfg,
		logger:        logger,
		instanceID:    instanceID,
		defaults: &ServiceDefaults{
			SnapshotID: cfg.Forward.DefaultSnapshotID,
			QueryLimit: cfg.Forward.DefaultQueryLimit,
		},
		networks:        newSessionNetworks(cfg.Forward.DefaultNetworkID),
		workflowManager: NewWorkflowManager(1000, 24*time.Hour), // Max 1000 sessions, 24h TTL
		semanticCache:   semanticCache,
		queryIndex:      queryIndex,
		database:        database,
		memorySystem:    memorySystem,
		apiTracker:      apiTracker,
		bloomManager:    bloomManager,
		rowQuerier:      deps.Rows,
		skills:          deps.Skills,
		ctx:             ctx,
		cancelFunc:      cancelFunc,
	}

	// Set up database callback to automatically refresh query index when database is updated
	if database != nil && queryIndex != nil {
		database.AddUpdateCallback(func() {
			logger.Info("🔄 Database updated, automatically refreshing query index...")

			// Load updated queries from database
			queries, err := database.LoadQueries()
			if err != nil {
				logger.Warn("🔄 Failed to load updated queries for index refresh: %v", err)
				return
			}

			// Refresh query index with updated data
			if err := queryIndex.LoadFromQueries(queries); err != nil {
				logger.Warn("🔄 Failed to refresh query index after database update: %v", err)
			} else {
				logger.Info("🔄 Query index automatically refreshed with %d queries", len(queries))

				// Check embedding coverage after refresh
				stats := queryIndex.GetStatistics()
				embeddedCount := stats["embedded_queries"].(int)
				if embeddedCount > 0 && embeddedCount < len(queries) {
					coverage := stats["embedding_coverage"].(float64)
					logger.Info("🧠 AI embeddings coverage: %.1f%% (%d/%d queries)", coverage*100, embeddedCount, len(queries))
				}
			}
		})
		logger.Info("🔄 Database update callback registered for automatic query index refresh")
	}

	// Initialize query index with existing data synchronously
	if database != nil {
		// Try to load existing queries from database first
		logger.Info("🔄 Loading existing queries from database...")
		queries, err := database.LoadQueries()
		if err != nil {
			logger.Warn("🔄 Failed to load queries from database: %v", err)
			// Fallback to spec file
			if err := queryIndex.LoadFromSpec(); err != nil {
				logger.Warn("🔄 Failed to initialize query index from spec: %v", err)
			} else {
				logger.Info("🔄 Query index initialized from spec file as fallback")
			}
		} else if len(queries) > 0 {
			// Load existing queries into index
			if err := queryIndex.LoadFromQueries(queries); err != nil {
				logger.Error("🔄 Failed to load queries into index: %v", err)
				// Fallback to spec file
				if err := queryIndex.LoadFromSpec(); err != nil {
					logger.Warn("🔄 Failed to initialize query index from spec: %v", err)
				} else {
					logger.Info("🔄 Query index initialized from spec file as fallback")
				}
			} else {
				logger.Info("🔄 Query index initialized with %d existing queries from database", len(queries))
				// Count enhanced queries for informational purposes
				enhancedCount := 0
				for _, q := range queries {
					if q.SourceCode != "" || q.Description != "" {
						enhancedCount++
					}
				}
				if enhancedCount > 0 {
					logger.Info("🚀 Found %d queries with enhanced metadata (source code/descriptions)", enhancedCount)
				} else {
					logger.Info("💡 Tip: Run 'hydrate_database' with enhanced_mode for richer query metadata")
				}
			}
		} else {
			// Database is empty, load from spec file
			logger.Info("🔄 Database is empty, initializing from spec file...")
			if err := queryIndex.LoadFromSpec(); err != nil {
				logger.Warn("🔄 Failed to initialize query index from spec: %v", err)
			} else {
				logger.Info("🔄 Query index initialized from spec file")
				logger.Info("💡 Tip: Run 'hydrate_database' to populate with live data from API")
			}
		}
	} else {
		// No database, fallback to spec file loading
		logger.Info("🔄 No database available, loading from spec file...")
		if err := queryIndex.LoadFromSpec(); err != nil {
			logger.Warn("🔄 Failed to initialize query index from spec: %v", err)
		} else {
			logger.Info("🔄 Query index initialized from spec file")
		}
	}

	return service
}

// Shutdown gracefully shuts down the Service
func (s *Service) Shutdown(timeout time.Duration) error {
	s.logger.Info("Shutting down Service...")

	// Cancel the service context
	s.cancelFunc()

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use errgroup to shutdown components concurrently with timeout enforcement
	g, gctx := errgroup.WithContext(ctx)

	// Close database connection if it exists
	if s.database != nil {
		g.Go(func() error {
			if err := s.database.Close(); err != nil {
				s.logger.Error("Failed to close database: %v", err)
				return fmt.Errorf("failed to close database: %w", err)
			}
			return nil
		})
	}

	// Close memory system if it exists
	if s.memorySystem != nil {
		g.Go(func() error {
			if err := s.memorySystem.Close(); err != nil {
				s.logger.Error("Failed to close memory system: %v", err)
				return fmt.Errorf("failed to close memory system: %w", err)
			}
			return nil
		})
	}

	// Stop the result cache's cleanup goroutine
	if s.semanticCache != nil {
		s.semanticCache.Close()
	}

	// Stop workflow manager cleanup goroutine (non-blocking)
	if s.workflowManager != nil {
		s.workflowManager.Close()
	}

	// Wait for all shutdown operations to complete or timeout
	if err := g.Wait(); err != nil {
		// Check if timeout occurred
		if gctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("shutdown timed out after %v: %w", timeout, err)
		}
		return fmt.Errorf("shutdown error: %w", err)
	}

	s.logger.Info("Service shutdown complete")
	return nil
}

// Validation helper functions - provide LLM-friendly error messages

// validateNetworkID validates that a network ID is provided, with helpful guidance
func (s *Service) validateNetworkID(networkID string) error {
	if networkID == "" {
		return fmt.Errorf("network_id is required. Use the list_networks tool to find available networks, or use set_default_network to configure a default network")
	}
	return nil
}

// validateQueryID validates that a query ID is provided
func (s *Service) validateQueryID(queryID string) error {
	if queryID == "" {
		return fmt.Errorf("query_id is required. Use the list_nqe_queries or search_nqe_queries tool to find available queries and their IDs")
	}
	return nil
}

// validateEntityName validates that an entity name is provided
func (s *Service) validateEntityName(name string) error {
	if name == "" {
		return fmt.Errorf("entity name is required and cannot be empty")
	}
	return nil
}

// validateEntityType validates that an entity type is provided
func (s *Service) validateEntityType(entityType string) error {
	if entityType == "" {
		return fmt.Errorf("entity type is required and cannot be empty. Common types include: 'user', 'network', 'device', 'project', 'session'")
	}
	return nil
}

// validateNonEmpty validates that a required string field is not empty
func (s *Service) validateNonEmpty(fieldName, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required and cannot be empty", fieldName)
	}
	return nil
}

// getNetworkID returns networkID, or the caller's session default when it is empty.
func (s *Service) getNetworkID(ctx context.Context, networkID string) string {
	if networkID != "" {
		return networkID
	}
	if s.networks != nil {
		return s.networks.get(ctx)
	}
	return ""
}

// Helper function to get snapshot ID with fallback to default
func (s *Service) getSnapshotID(snapshotID string) string {
	if snapshotID != "" {
		return snapshotID
	}
	if s.defaults != nil {
		return s.defaults.SnapshotID
	}
	return ""
}

// Helper function to get query limit with fallback to default
func (s *Service) getQueryLimit(limit int) int {
	if limit > 0 {
		return limit
	}
	if s.defaults != nil {
		return s.defaults.QueryLimit
	}
	return 1000 // Default fallback if no defaults are set
}

// Helper function to log tool calls with detailed information (legacy compatibility)
func (s *Service) logToolCall(toolName string, args interface{}, err error) {
	// Use zero duration for legacy calls - timing will be handled at a higher level
	s.logger.LogToolCall(toolName, args, 0, err)
}

// Enhanced function to log tool calls with performance metrics
func (s *Service) logToolCallWithTiming(toolName string, args interface{}, duration time.Duration, err error) {
	s.logger.LogToolCall(toolName, args, duration, err)
}

// Wrapper function to time and log tool execution
func (s *Service) timeAndLogTool(toolName string, args interface{}, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	s.logToolCallWithTiming(toolName, args, duration, err)
	return err
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
