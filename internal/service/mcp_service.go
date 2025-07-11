package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/forward-mcp/internal/config"
	"github.com/forward-mcp/internal/forward"
	"github.com/forward-mcp/internal/logger"
	mcp "github.com/metoro-io/mcp-golang"
)

// WorkflowState represents the current state of a user workflow
type WorkflowState struct {
	CurrentStep   string                 `json:"current_step"`
	Parameters    map[string]interface{} `json:"parameters"`
	SelectedQuery string                 `json:"selected_query"`
	NetworkID     string                 `json:"network_id"`
	SnapshotID    string                 `json:"snapshot_id"`
}

// WorkflowManager manages user workflow states
type WorkflowManager struct {
	sessions map[string]*WorkflowState
	mutex    sync.RWMutex
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		sessions: make(map[string]*WorkflowState),
	}
}

// GetState gets the workflow state for a session
func (wm *WorkflowManager) GetState(sessionID string) *WorkflowState {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()
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
}

// ForwardMCPService implements Forward Networks MCP tools using mcp-golang
type ForwardMCPService struct {
	forwardClient   forward.ClientInterface
	config          *config.Config
	logger          *logger.Logger
	instanceID      string // Unique identifier for this Forward Networks instance
	defaults        *ServiceDefaults
	workflowManager *WorkflowManager
	semanticCache   *SemanticCache
	queryIndex      *NQEQueryIndex
	database        *NQEDatabase
	memorySystem    *MemorySystem     // Knowledge graph memory system
	apiTracker      *APIMemoryTracker // API result tracking using memory system
	// Context cancellation for graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// ServiceDefaults holds default values for the MCP service
type ServiceDefaults struct {
	NetworkID  string
	SnapshotID string
	QueryLimit int
}

// NewForwardMCPService creates a new Forward MCP service
func NewForwardMCPService(cfg *config.Config, logger *logger.Logger) *ForwardMCPService {
	// Generate instance ID for partitioning database and cache by Forward Networks instance
	instanceID := GenerateInstanceID(cfg.Forward.APIBaseURL)
	logger.Info("Using instance ID '%s' for partitioning (based on %s)", instanceID, cfg.Forward.APIBaseURL)

	// Create Forward Networks client
	forwardClient := forward.NewClient(&cfg.Forward)

	// Create embedding service based on config
	var embeddingService EmbeddingService
	if cfg.Forward.SemanticCache.EmbeddingProvider == "openai" {
		if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
			embeddingService = NewOpenAIEmbeddingService(openaiKey)
		} else {
			embeddingService = NewKeywordEmbeddingService()
			logger.Warn("OpenAI provider selected but OPENAI_API_KEY not set - using keyword embedding service")
		}
	} else {
		embeddingService = NewKeywordEmbeddingService()
	}

	// Create semantic cache with instance partitioning
	semanticCache := NewSemanticCache(embeddingService, logger, instanceID, &cfg.Forward.SemanticCache)

	// Create database with instance partitioning
	database, err := NewNQEDatabase(logger, instanceID)
	if err != nil {
		logger.Error("Failed to create database: %v", err)
		// Continue without database - will fall back to spec file
		database = nil
	}

	// Create query index
	queryIndex := NewNQEQueryIndex(embeddingService, logger)

	// Create memory system
	memorySystem, err := NewMemorySystem(logger, instanceID)
	if err != nil {
		logger.Error("Failed to create memory system: %v", err)
		// Continue without memory system
		memorySystem = nil
	}

	// Create API memory tracker
	var apiTracker *APIMemoryTracker
	if memorySystem != nil {
		apiTracker = NewAPIMemoryTracker(memorySystem, logger, instanceID)
		logger.Info("API memory tracker initialized for tracking API results and relationships")
	}

	// Create context for cancellation
	ctx, cancelFunc := context.WithCancel(context.Background())

	service := &ForwardMCPService{
		forwardClient: forwardClient,
		config:        cfg,
		logger:        logger,
		instanceID:    instanceID,
		defaults: &ServiceDefaults{
			NetworkID:  cfg.Forward.DefaultNetworkID,
			SnapshotID: cfg.Forward.DefaultSnapshotID,
			QueryLimit: cfg.Forward.DefaultQueryLimit,
		},
		workflowManager: NewWorkflowManager(),
		semanticCache:   semanticCache,
		queryIndex:      queryIndex,
		database:        database,
		memorySystem:    memorySystem,
		apiTracker:      apiTracker,
		ctx:             ctx,
		cancelFunc:      cancelFunc,
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
			}
		} else {
			// No existing queries, load from spec file
			logger.Info("🔄 No existing queries found, loading from spec file...")
			if err := queryIndex.LoadFromSpec(); err != nil {
				logger.Warn("🔄 Failed to initialize query index from spec: %v", err)
			} else {
				logger.Info("🔄 Query index initialized from spec file")
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

// Shutdown gracefully shuts down the ForwardMCPService
func (s *ForwardMCPService) Shutdown(timeout time.Duration) error {
	s.logger.Info("Shutting down ForwardMCPService...")

	// Cancel the context
	s.cancelFunc()

	// Close database connection if it exists
	if s.database != nil {
		if err := s.database.Close(); err != nil {
			s.logger.Error("Failed to close database: %v", err)
			return fmt.Errorf("failed to close database: %w", err)
		}
	}

	// Close memory system if it exists
	if s.memorySystem != nil {
		if err := s.memorySystem.Close(); err != nil {
			s.logger.Error("Failed to close memory system: %v", err)
			return fmt.Errorf("failed to close memory system: %w", err)
		}
	}

	s.logger.Info("ForwardMCPService shutdown complete")
	return nil
}

// Helper function to get network ID with fallback to default
func (s *ForwardMCPService) getNetworkID(networkID string) string {
	if networkID != "" {
		return networkID
	}
	if s.defaults != nil {
		return s.defaults.NetworkID
	}
	return ""
}

// Helper function to get snapshot ID with fallback to default
func (s *ForwardMCPService) getSnapshotID(snapshotID string) string {
	if snapshotID != "" {
		return snapshotID
	}
	if s.defaults != nil {
		return s.defaults.SnapshotID
	}
	return ""
}

// Helper function to get query limit with fallback to default
func (s *ForwardMCPService) getQueryLimit(limit int) int {
	if limit > 0 {
		return limit
	}
	if s.defaults != nil {
		return s.defaults.QueryLimit
	}
	return 1000 // Default fallback if no defaults are set
}

// Helper function to log tool calls with detailed information (legacy compatibility)
func (s *ForwardMCPService) logToolCall(toolName string, args interface{}, err error) {
	// Use zero duration for legacy calls - timing will be handled at a higher level
	s.logger.LogToolCall(toolName, args, 0, err)
}

// Enhanced function to log tool calls with performance metrics
func (s *ForwardMCPService) logToolCallWithTiming(toolName string, args interface{}, duration time.Duration, err error) {
	s.logger.LogToolCall(toolName, args, duration, err)
}

// Wrapper function to time and log tool execution
func (s *ForwardMCPService) timeAndLogTool(toolName string, args interface{}, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	s.logToolCallWithTiming(toolName, args, duration, err)
	return err
}

// RegisterTools registers all Forward Networks tools with the MCP server
func (s *ForwardMCPService) RegisterTools(server *mcp.Server) error {
	// Network Management Tools
	if err := server.RegisterTool("list_networks",
		"List all networks in the Forward platform. Returns network IDs, names, and descriptions. Use this to discover available networks or find network IDs for other operations.",
		s.listNetworks); err != nil {
		return fmt.Errorf("failed to register list_networks tool: %w", err)
	}

	if err := server.RegisterTool("create_network",
		"Create a new network in the Forward platform. Requires a network name. Returns the new network with ID for subsequent operations.",
		s.createNetwork); err != nil {
		return fmt.Errorf("failed to register create_network tool: %w", err)
	}

	if err := server.RegisterTool("delete_network",
		"Delete a network from the Forward platform. Requires network_id. WARNING: This permanently deletes all associated data.",
		s.deleteNetwork); err != nil {
		return fmt.Errorf("failed to register delete_network tool: %w", err)
	}

	if err := server.RegisterTool("update_network",
		"Update network properties in the Forward platform. Requires network_id and at least one property to update (name or description).",
		s.updateNetwork); err != nil {
		return fmt.Errorf("failed to register update_network tool: %w", err)
	}

	// Path Search Tools
	if err := server.RegisterTool("search_paths",
		"Search for network paths by tracing packets through the network. Requires network_id from, or src_ip and dst_ip. Use for connectivity verification, troubleshooting, and routing analysis. Can specify source IP, ports, and protocols for detailed path tracing.",
		s.searchPaths); err != nil {
		return fmt.Errorf("failed to register search_paths tool: %w", err)
	}

	// NQE Tools
	if err := server.RegisterTool("run_nqe_query_by_id",
		"Run a Network Query Engine (NQE) query using a predefined query ID from the library. Use for standard reports, compliance checks, and consistent analysis. First use list_nqe_queries to discover available queries and their IDs.",
		s.runNQEQueryByID); err != nil {
		return fmt.Errorf("failed to register run_nqe_query_by_id tool: %w", err)
	}

	if err := server.RegisterTool("list_nqe_queries",
		"List available NQE queries from the Forward Networks query library. Use to discover predefined queries for reports and analysis. Can filter by directory (/L3/Basic/, /L3/Advanced/, /L3/Security/). Returns query IDs for use with run_nqe_query_by_id.",
		s.listNQEQueries); err != nil {
		return fmt.Errorf("failed to register list_nqe_queries tool: %w", err)
	}

	// First-Class Query Tools - Most Important Network Operations
	if err := server.RegisterTool("get_device_basic_info",
		"Get basic device information including names, platforms, and management IPs. Essential for device inventory and discovery. Uses predefined Device Basic Info query.",
		s.getDeviceBasicInfo); err != nil {
		return fmt.Errorf("failed to register get_device_basic_info tool: %w", err)
	}

	if err := server.RegisterTool("get_device_hardware",
		"Get device hardware information including models, serial numbers, and hardware details. Critical for hardware inventory and lifecycle management.",
		s.getDeviceHardware); err != nil {
		return fmt.Errorf("failed to register get_device_hardware tool: %w", err)
	}

	if err := server.RegisterTool("get_hardware_support",
		"Get hardware support status including end-of-life and support dates. Essential for compliance and planning hardware refreshes.",
		s.getHardwareSupport); err != nil {
		return fmt.Errorf("failed to register get_hardware_support tool: %w", err)
	}

	if err := server.RegisterTool("get_os_support",
		"Get operating system support status including OS versions and support dates. Critical for security compliance and OS upgrade planning.",
		s.getOSSupport); err != nil {
		return fmt.Errorf("failed to register get_os_support tool: %w", err)
	}

	if err := server.RegisterTool("search_configs",
		"Search device configurations for specific patterns, commands, or settings.\n\nTo create a block pattern, use triple backticks (```) to start and end the pattern, and indent lines to show hierarchy. Example:\n\npattern = ```\ninterface\n  zone-member security\n  ip address {ip:string}\n```\n\nEach line is a line pattern. Indentation defines parent/child relationships. Use curly braces for variable extraction (e.g., {ip:string}). For more, see the data extraction guide.",
		s.searchConfigs); err != nil {
		return fmt.Errorf("failed to register search_configs tool: %w", err)
	}

	if err := server.RegisterTool("get_config_diff",
		"Compare network configurations between snapshots to identify changes. Essential for change tracking and troubleshooting configuration drift.",
		s.getConfigDiff); err != nil {
		return fmt.Errorf("failed to register get_config_diff tool: %w", err)
	}

	// Device Management Tools
	if err := server.RegisterTool("list_devices",
		"List devices in a network. Requires network_id. Returns basic device inventory with names, types, and status. Supports pagination with limit and offset. Use for device discovery and inventory management.",
		s.listDevices); err != nil {
		return fmt.Errorf("failed to register list_devices tool: %w", err)
	}

	if err := server.RegisterTool("get_device_locations",
		"Get device location mappings for a network. Requires network_id. Shows which devices are assigned to which physical locations. Use for topology planning and device organization.",
		s.getDeviceLocations); err != nil {
		return fmt.Errorf("failed to register get_device_locations tool: %w", err)
	}

	// Snapshot Management Tools
	if err := server.RegisterTool("list_snapshots",
		"List network configuration snapshots. Requires network_id. Shows historical network states with timestamps and status. Use to view configuration history and find specific snapshots for queries.",
		s.listSnapshots); err != nil {
		return fmt.Errorf("failed to register list_snapshots tool: %w", err)
	}

	if err := server.RegisterTool("get_latest_snapshot",
		"Get the latest processed snapshot for a network. Requires network_id. Returns the most recent network state. Use to ensure queries run against current configuration.",
		s.getLatestSnapshot); err != nil {
		return fmt.Errorf("failed to register get_latest_snapshot tool: %w", err)
	}

	// Location Management Tools
	if err := server.RegisterTool("list_locations",
		"List locations in a network. Requires network_id. Returns physical locations with names and coordinates. Use to view network topology and organize devices by location.",
		s.listLocations); err != nil {
		return fmt.Errorf("failed to register list_locations tool: %w", err)
	}

	if err := server.RegisterTool("create_location",
		"Create a new location in a network. Requires network_id and location name. Optional description and coordinates. Use to set up new sites or data centers for device organization.",
		s.createLocation); err != nil {
		return fmt.Errorf("failed to register create_location tool: %w", err)
	}

	// Default Settings Management Tools
	if err := server.RegisterTool("get_default_settings",
		"View current default settings for network operations. Shows the default network ID, snapshot ID, and query limits configured for this session.",
		s.getDefaultSettings); err != nil {
		return fmt.Errorf("failed to register get_default_settings tool: %w", err)
	}

	if err := server.RegisterTool("set_default_network",
		"Set the default network for all operations. Accepts either a network ID or network name. This will be used when network_id is not specified in other tools.",
		s.setDefaultNetwork); err != nil {
		return fmt.Errorf("failed to register set_default_network tool: %w", err)
	}

	// Semantic Cache and AI Enhancement Tools
	if err := server.RegisterTool("get_cache_stats",
		"View semantic cache performance statistics including hit rates, total queries, and cache efficiency metrics.",
		s.getCacheStats); err != nil {
		return fmt.Errorf("failed to register get_cache_stats tool: %w", err)
	}

	if err := server.RegisterTool("suggest_similar_queries",
		"Get suggestions for similar NQE queries based on semantic similarity to your query intent. Helps discover relevant existing queries.",
		s.suggestSimilarQueries); err != nil {
		return fmt.Errorf("failed to register suggest_similar_queries tool: %w", err)
	}

	if err := server.RegisterTool("clear_cache",
		"Clear expired entries from the semantic cache to free up memory and improve performance.",
		s.clearCache); err != nil {
		return fmt.Errorf("failed to register clear_cache tool: %w", err)
	}

	// AI-Powered Query Discovery Tools
	if err := server.RegisterTool("search_nqe_queries",
		"🧠 AI-powered search through 6000+ predefined NQE queries using natural language. Describe what you want to analyze (e.g., 'AWS security issues', 'BGP routing problems', 'interface utilization') and get relevant query suggestions with similarity scores. Use this for EXPLORATION when you want to see what queries are available for a topic. For actionable results that can be immediately executed, use 'find_executable_query' instead.",
		s.searchNQEQueries); err != nil {
		return fmt.Errorf("failed to register search_nqe_queries tool: %w", err)
	}

	if err := server.RegisterTool("find_executable_query",
		"🎯 BEST TOOL for query discovery! Smart query discovery that finds executable NQE queries for your needs. Uses AI semantic search across 6000+ queries, then maps results to actually runnable queries with real Forward Networks IDs. Use this when user asks 'I want to do X, what query should I run?' or wants actionable results. Returns queries you can immediately execute with 'run_nqe_query_by_id'. Always try this first before search_nqe_queries.",
		s.findExecutableQuery); err != nil {
		return fmt.Errorf("failed to register find_executable_query tool: %w", err)
	}

	if err := server.RegisterTool("initialize_query_index",
		"Initialize or rebuild the AI-powered NQE query index from the spec file. REQUIRED before using search_nqe_queries or find_executable_query. Run this once at startup or when you get 'query index is empty' errors. Can generate embeddings for semantic search if OpenAI API key is available.",
		s.initializeQueryIndex); err != nil {
		return fmt.Errorf("failed to register initialize_query_index tool: %w", err)
	}

	if err := server.RegisterTool("get_query_index_stats",
		"View statistics about the AI-powered NQE query index including total queries, categories, and embedding coverage.",
		s.getQueryIndexStats); err != nil {
		return fmt.Errorf("failed to register get_query_index_stats tool: %w", err)
	}

	if err := server.RegisterTool("test_semantic_cache", "Test the semantic cache with a query, network_id, and snapshot_id.", s.testSemanticCache); err != nil {
		return fmt.Errorf("failed to register test_semantic_cache tool: %w", err)
	}

	if err := server.RegisterTool("run_semantic_nqe_query",
		"Finds the most relevant NQE query using semantic search and executes it. Provide a natural language description of what you want to analyze.",
		s.runSemanticNQEQuery); err != nil {
		return fmt.Errorf("failed to register run_semantic_nqe_query tool: %w", err)
	}

	// Database Hydration Tools
	if err := server.RegisterTool("hydrate_database",
		"Hydrate the NQE database by loading queries from the Forward Networks API. Use this to refresh the database with latest query metadata and ensure optimal performance for search operations.",
		s.hydrateDatabase); err != nil {
		return fmt.Errorf("failed to register hydrate_database tool: %w", err)
	}

	if err := server.RegisterTool("refresh_query_index",
		"Refresh the query index from the current database content. Use this after hydrating the database to ensure the search index reflects the latest data.",
		s.refreshQueryIndex); err != nil {
		return fmt.Errorf("failed to register refresh_query_index tool: %w", err)
	}

	if err := server.RegisterTool("get_database_status",
		"Get the current status of the database and query index including query counts, last update times, and performance metrics.",
		s.getDatabaseStatus); err != nil {
		return fmt.Errorf("failed to register get_database_status tool: %w", err)
	}

	// Memory Management Tools
	if err := server.RegisterTool("create_entity",
		"Create a new entity in the knowledge graph memory system. Entities represent people, networks, devices, projects, or any other important concept to remember.",
		s.createEntity); err != nil {
		return fmt.Errorf("failed to register create_entity tool: %w", err)
	}

	if err := server.RegisterTool("create_relation",
		"Create a relation between two entities in the knowledge graph. Relations represent how entities are connected (e.g., 'owns', 'manages', 'depends_on').",
		s.createRelation); err != nil {
		return fmt.Errorf("failed to register create_relation tool: %w", err)
	}

	if err := server.RegisterTool("add_observation",
		"Add an observation to an entity. Observations are additional facts, notes, preferences, or behaviors associated with an entity.",
		s.addObservation); err != nil {
		return fmt.Errorf("failed to register add_observation tool: %w", err)
	}

	if err := server.RegisterTool("search_entities",
		"Search for entities in the knowledge graph by name, type, or observation content. Use this to find information you've stored about people, networks, or concepts.",
		s.searchEntities); err != nil {
		return fmt.Errorf("failed to register search_entities tool: %w", err)
	}

	if err := server.RegisterTool("get_entity",
		"Retrieve a specific entity by ID or name. Use this to get detailed information about a specific person, network, device, or concept.",
		s.getEntity); err != nil {
		return fmt.Errorf("failed to register get_entity tool: %w", err)
	}

	if err := server.RegisterTool("get_relations",
		"Get all relations for a specific entity. Use this to understand how an entity is connected to others in the knowledge graph.",
		s.getRelations); err != nil {
		return fmt.Errorf("failed to register get_relations tool: %w", err)
	}

	if err := server.RegisterTool("get_observations",
		"Get all observations for a specific entity. Use this to retrieve all stored facts, notes, and preferences about an entity.",
		s.getObservations); err != nil {
		return fmt.Errorf("failed to register get_observations tool: %w", err)
	}

	if err := server.RegisterTool("delete_entity",
		"Delete an entity and all its relations and observations. Use with caution as this permanently removes all stored information about the entity.",
		s.deleteEntity); err != nil {
		return fmt.Errorf("failed to register delete_entity tool: %w", err)
	}

	if err := server.RegisterTool("delete_relation",
		"Delete a specific relation between entities. Use this to remove connections that are no longer relevant.",
		s.deleteRelation); err != nil {
		return fmt.Errorf("failed to register delete_relation tool: %w", err)
	}

	if err := server.RegisterTool("delete_observation",
		"Delete a specific observation from an entity. Use this to remove outdated or incorrect information.",
		s.deleteObservation); err != nil {
		return fmt.Errorf("failed to register delete_observation tool: %w", err)
	}

	if err := server.RegisterTool("get_memory_stats",
		"Get statistics about the memory system including counts of entities, relations, and observations by type.",
		s.getMemoryStats); err != nil {
		return fmt.Errorf("failed to register get_memory_stats tool: %w", err)
	}

	// API Analytics Tools
	if err := server.RegisterTool("get_query_analytics",
		"Get analytics about query patterns and performance for a specific network. Shows query counts, execution times, result patterns, and usage trends from the memory system.",
		s.getQueryAnalytics); err != nil {
		return fmt.Errorf("failed to register get_query_analytics tool: %w", err)
	}

	return nil
}

// RegisterPrompts registers workflow prompts with the MCP server
func (s *ForwardMCPService) RegisterPrompts(server *mcp.Server) error {
	// Register Smart Query Discovery Workflow prompt
	if err := server.RegisterPrompt("smart_query_workflow",
		"🧠 Smart Query Discovery Workflow - Best practices for using AI-powered query discovery to find and execute network analysis queries. Guides LLMs through the optimal workflow: initialization → discovery → execution.",
		func(args SmartQueryWorkflowArgs) (*mcp.PromptResponse, error) {
			return s.smartQueryWorkflow(args)
		}); err != nil {
		return fmt.Errorf("failed to register smart_query_workflow prompt: %w", err)
	}

	// Register NQE Query Discovery workflow as a prompt
	if err := server.RegisterPrompt("nqe_discovery", "Interactive NQE query discovery workflow to help find and run network queries", func(args NQEDiscoveryArgs) (*mcp.PromptResponse, error) {
		response, err := s.nqeQueryDiscoveryWorkflow(args)
		if err != nil {
			return nil, err
		}
		// Convert ToolResponse to PromptResponse
		if len(response.Content) > 0 {
			return mcp.NewPromptResponse("NQE Query Discovery", mcp.NewPromptMessage(response.Content[0], mcp.RoleAssistant)), nil
		}
		return mcp.NewPromptResponse("NQE Query Discovery", mcp.NewPromptMessage(mcp.NewTextContent("Welcome to NQE Query Discovery!"), mcp.RoleAssistant)), nil
	}); err != nil {
		return fmt.Errorf("failed to register nqe_discovery prompt: %w", err)
	}

	// Register Network Discovery workflow as a prompt
	if err := server.RegisterPrompt("network_discovery", "Interactive network discovery workflow to explore available networks and devices", func(args NetworkDiscoveryArgs) (*mcp.PromptResponse, error) {
		response, err := s.networkDiscoveryWorkflow(args)
		if err != nil {
			return nil, err
		}
		// Convert ToolResponse to PromptResponse
		if len(response.Content) > 0 {
			return mcp.NewPromptResponse("Network Discovery", mcp.NewPromptMessage(response.Content[0], mcp.RoleAssistant)), nil
		}
		return mcp.NewPromptResponse("Network Discovery", mcp.NewPromptMessage(mcp.NewTextContent("Network discovery workflow"), mcp.RoleAssistant)), nil
	}); err != nil {
		return fmt.Errorf("failed to register network_discovery prompt: %w", err)
	}

	s.logger.Info("MCP ready - Forward Networks tools registered")
	return nil
}

// RegisterResources registers contextual resources with the MCP server
func (s *ForwardMCPService) RegisterResources(server *mcp.Server) error {
	// Register network context as a resource
	if err := server.RegisterResource("forward://network/context", "network_context", "Current network context including available networks and queries", "application/json", func() (*mcp.ResourceResponse, error) {
		context, err := s.getNetworkContext(NetworkContextArgs{})
		if err != nil {
			return nil, fmt.Errorf("failed to get network context: %w", err)
		}

		contextStr, ok := context.(string)
		if !ok {
			return nil, fmt.Errorf("network context is not a string")
		}

		return mcp.NewResourceResponse(mcp.NewTextEmbeddedResource("forward://network/context", contextStr, "application/json")), nil
	}); err != nil {
		return fmt.Errorf("failed to register network_context resource: %w", err)
	}

	s.logger.Debug("Successfully registered MCP resources")
	return nil
}

// nqeQueryDiscoveryWorkflow implements the NQE query discovery workflow
func (s *ForwardMCPService) nqeQueryDiscoveryWorkflow(args NQEDiscoveryArgs) (*mcp.ToolResponse, error) {
	sessionID := fmt.Sprintf("session_%v", args.SessionID) // In practice, extract from context
	state := s.workflowManager.GetState(sessionID)

	switch state.CurrentStep {
	case "start":
		return s.startQueryDiscovery(sessionID)
	case "category_selected":
		return s.listQueriesInCategory(sessionID, state.Parameters["directory"].(string))
	case "query_selected":
		return s.collectQueryParameters(sessionID)
	case "parameters_collected":
		return s.executeSelectedQuery(sessionID)
	default:
		return s.startQueryDiscovery(sessionID)
	}
}

// networkDiscoveryWorkflow implements the network discovery workflow
func (s *ForwardMCPService) networkDiscoveryWorkflow(args NetworkDiscoveryArgs) (*mcp.ToolResponse, error) {
	networks, err := s.forwardClient.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	promptText := "Available networks:\n"
	for i, network := range networks {
		promptText += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, network.Name, network.ID)
	}
	promptText += "\nWhat would you like to do?\n1. Select a network to explore\n2. Create a new network\n3. Search for specific devices"

	return mcp.NewToolResponse(mcp.NewTextContent(promptText)), nil
}

// getNetworkContext provides contextual network information as a resource
func (s *ForwardMCPService) getNetworkContext(args NetworkContextArgs) (interface{}, error) {
	networks, err := s.forwardClient.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to get network context: %w", err)
	}

	context := map[string]interface{}{
		"networks":          networks,
		"timestamp":         "current",
		"available_queries": []string{"/L3/Basic/", "/L3/Advanced/", "/L3/Security/"},
	}

	contextJSON, _ := json.MarshalIndent(context, "", "  ")
	return string(contextJSON), nil
}

// startQueryDiscovery begins the NQE query discovery workflow
func (s *ForwardMCPService) startQueryDiscovery(sessionID string) (*mcp.ToolResponse, error) {
	state := &WorkflowState{
		CurrentStep: "category_selection",
		Parameters:  make(map[string]interface{}),
	}
	s.workflowManager.SetState(sessionID, state)

	promptText := "Welcome to NQE Query Discovery!\n\nSelect a query category:\n1. Basic (/L3/Basic/) - Device inventory, basic connectivity\n2. Advanced (/L3/Advanced/) - Complex routing, performance analysis\n3. Security (/L3/Security/) - Security policies, compliance\n\nWhich category interests you?"
	return mcp.NewToolResponse(mcp.NewTextContent(promptText)), nil
}

// listQueriesInCategory lists available queries in the selected category
func (s *ForwardMCPService) listQueriesInCategory(sessionID, directory string) (*mcp.ToolResponse, error) {
	queries, err := s.forwardClient.GetNQEQueries(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to get queries: %w", err)
	}

	state := s.workflowManager.GetState(sessionID)
	state.CurrentStep = "query_selection"
	state.Parameters["directory"] = directory
	s.workflowManager.SetState(sessionID, state)

	promptText := fmt.Sprintf("Available queries in %s:\n", directory)
	for i, query := range queries {
		promptText += fmt.Sprintf("%d. %s (ID: %s)\n   Purpose: %s\n", i+1, query.Path, query.QueryID, query.Intent)
	}
	promptText += "\nWhich query would you like to run?"

	return mcp.NewToolResponse(mcp.NewTextContent(promptText)), nil
}

// collectQueryParameters collects parameters needed for the selected query
func (s *ForwardMCPService) collectQueryParameters(sessionID string) (*mcp.ToolResponse, error) {
	state := s.workflowManager.GetState(sessionID)

	// Check if we have network_id
	if _, exists := state.Parameters["network_id"]; !exists {
		return s.promptForParameter(sessionID, "network_id")
	}

	// Check if we have snapshot_id
	if _, exists := state.Parameters["snapshot_id"]; !exists {
		return s.promptForParameter(sessionID, "snapshot_id")
	}

	// All parameters collected, ready to execute
	state.CurrentStep = "ready_to_execute"
	s.workflowManager.SetState(sessionID, state)

	return mcp.NewToolResponse(mcp.NewTextContent("All parameters collected! Ready to execute query. Proceed?")), nil
}

// executeSelectedQuery executes the query with collected parameters
// This function is part of the workflow system that is now activated via MCP prompt registration
func (s *ForwardMCPService) executeSelectedQuery(sessionID string) (*mcp.ToolResponse, error) {
	state := s.workflowManager.GetState(sessionID)

	params := &forward.NQEQueryParams{
		NetworkID:  state.NetworkID,
		QueryID:    state.SelectedQuery,
		SnapshotID: state.SnapshotID,
	}

	result, err := s.forwardClient.RunNQEQueryByID(params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	promptText := fmt.Sprintf("Query executed successfully! Found %d results:\n%s\n\nWhat would you like to do next?\n1. Export results\n2. Run another query\n3. Get more details\n4. Exit", len(result.Items), string(resultJSON))

	return mcp.NewToolResponse(mcp.NewTextContent(promptText)), nil
}

// Network Observability Tool Implementations
func (s *ForwardMCPService) listNetworks(args ListNetworksArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("list_networks", args, nil)

	networks, err := s.forwardClient.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	result := MarshalCompactJSONString(networks)
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d networks:\n%s", len(networks), result))), nil
}

func (s *ForwardMCPService) createNetwork(args CreateNetworkArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("create_network", args, nil)
	network, err := s.forwardClient.CreateNetwork(args.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Network created successfully:\n%s", string(result)))), nil
}

func (s *ForwardMCPService) deleteNetwork(args DeleteNetworkArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("delete_network", args, nil)
	network, err := s.forwardClient.DeleteNetwork(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete network: %w", err)
	}

	result, _ := json.MarshalIndent(network, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Network deleted successfully:\n%s", string(result)))), nil
}

func (s *ForwardMCPService) updateNetwork(args UpdateNetworkArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("update_network", args, nil)
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
	s.logToolCall("search_paths", args, nil)

	// Use defaults if not specified (like other functions do)
	networkID := s.getNetworkID(args.NetworkID)
	snapshotID := s.getSnapshotID(args.SnapshotID)

	// If no snapshot ID is available, fetch the latest snapshot for the network
	if snapshotID == "" || snapshotID == "latest" {
		s.logger.Info("searchPaths - No snapshot ID provided or in defaults, fetching latest snapshot for network %s", networkID)

		snapshot, err := s.forwardClient.GetLatestSnapshot(networkID)
		if err != nil {
			s.logger.Error("Failed to fetch latest snapshot for network %s: %v", networkID, err)
			return nil, fmt.Errorf("failed to get latest snapshot for network %s: %w", networkID, err)
		}

		if snapshot != nil && snapshot.ID != "" {
			snapshotID = snapshot.ID
			s.logger.Info("searchPaths - Using latest snapshot ID: %s", snapshotID)
		} else {
			s.logger.Warn("No valid snapshot found for network %s", networkID)
			return nil, fmt.Errorf("no valid snapshot found for network %s - ensure the network has been processed", networkID)
		}
	}

	s.logger.Debug("Path search: networkID=%s, snapshotID=%s, srcIP=%s, dstIP=%s",
		networkID, snapshotID, args.SrcIP, args.DstIP)

	params := &forward.PathSearchParams{
		DstIP:                   args.DstIP,
		SrcIP:                   args.SrcIP,
		From:                    args.From,
		Intent:                  args.Intent,
		SrcPort:                 args.SrcPort,
		DstPort:                 args.DstPort,
		MaxResults:              args.MaxResults,
		IncludeNetworkFunctions: args.IncludeNetworkFunctions,
		SnapshotID:              snapshotID, // Now uses latest snapshot if not provided
	}

	if args.IPProto != 0 {
		params.IPProto = &args.IPProto
	}

	response, err := s.forwardClient.SearchPaths(networkID, params)
	if err != nil {
		s.logger.Error("Path search failed: %v", err)
		return nil, fmt.Errorf("failed to search paths: %w", err)
	}

	// Track path search in memory system
	if s.apiTracker != nil {
		if trackErr := s.apiTracker.TrackPathSearch(networkID, args.SrcIP, args.DstIP, response); trackErr != nil {
			s.logger.Debug("Failed to track path search in memory system: %v", trackErr)
		}
	}

	s.logger.Debug("Path search completed: found %d paths, searchTime=%dms, candidates=%d, snapshotID=%s",
		len(response.Paths), response.SearchTimeMs, response.NumCandidatesFound, response.SnapshotID)

	result := MarshalCompactJSONString(response)

	// Enhanced response with debugging info
	debugInfo := ""
	if response.SnapshotID == "" {
		debugInfo += "\n⚠️  Warning: No snapshot ID in response - this might indicate an issue\n"
	}
	if response.SearchTimeMs == 0 {
		debugInfo += "\n⚠️  Warning: Search time was 0ms - this suggests no real search occurred\n"
	}
	if response.NumCandidatesFound == 0 && args.SrcIP != "" {
		debugInfo += fmt.Sprintf("\n💡 No candidates found for source IP %s - this IP might not exist in the network topology\n", args.SrcIP)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Path search completed. Found %d paths:%s\n%s", len(response.Paths), debugInfo, result))), nil
}

// Helper function to convert service NQEQueryOptions to forward NQEQueryOptions
func (s *ForwardMCPService) convertNQEQueryOptions(options *NQEQueryOptions) *forward.NQEQueryOptions {
	if options == nil {
		return nil
	}

	// Apply default limit if not specified
	limit := options.Limit
	if limit == 0 {
		limit = s.getQueryLimit(0)
	}

	forwardOptions := &forward.NQEQueryOptions{
		Limit:  limit,
		Offset: options.Offset,
		Format: options.Format,
	}

	if options.SortBy != nil {
		forwardOptions.SortBy = make([]forward.NQESortBy, len(options.SortBy))
		for i, sort := range options.SortBy {
			forwardOptions.SortBy[i] = forward.NQESortBy{
				ColumnName: sort.ColumnName,
				Order:      sort.Order,
			}
		}
	}

	if options.Filters != nil {
		forwardOptions.Filters = make([]forward.NQEColumnFilter, len(options.Filters))
		for i, filter := range options.Filters {
			forwardOptions.Filters[i] = forward.NQEColumnFilter{
				ColumnName: filter.ColumnName,
				Value:      filter.Value,
			}
		}
	}

	return forwardOptions
}

// NQE Tool Implementations
func (s *ForwardMCPService) runNQEQueryByID(args RunNQEQueryByIDArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("run_nqe_query_by_id", args, nil)

	// Validate query ID against database index if available
	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	if totalQueries > 0 {
		if entry, err := s.queryIndex.GetQueryByID(args.QueryID); err != nil {
			s.logger.Warn("Query ID %s not found in database index - may be deprecated or invalid", args.QueryID)
			// Continue execution anyway in case it's a newer query not yet in the database
		} else {
			s.logger.Debug("Executing validated query: %s (Path: %s)", entry.QueryID, entry.Path)
		}
	}

	// Use defaults if not specified
	networkID := s.getNetworkID(args.NetworkID)
	snapshotID := s.getSnapshotID(args.SnapshotID)

	// Create cache key from query parameters
	cacheKey := fmt.Sprintf("query_id:%s|params:%v", args.QueryID, args.Parameters)

	// Try to get result from cache first
	if s.config.Forward.SemanticCache.Enabled && s.semanticCache != nil {
		if cachedResult, found := s.semanticCache.Get(cacheKey, networkID, snapshotID); found {
			s.logger.Debug("Cache hit for NQE query %s", args.QueryID)

			return mcp.NewToolResponse(mcp.NewTextContent(MarshalCompactJSONString(cachedResult))), nil
		}
	}

	params := &forward.NQEQueryParams{
		NetworkID:  networkID,
		QueryID:    args.QueryID,
		SnapshotID: snapshotID,
		Parameters: args.Parameters,
		Options:    s.convertNQEQueryOptions(args.Options),
	}

	// Ensure we have options even if none were provided
	if params.Options == nil {
		params.Options = &forward.NQEQueryOptions{
			Limit: s.getQueryLimit(0),
		}
	}

	// Track execution time for API memory tracking
	start := time.Now()
	result, err := s.forwardClient.RunNQEQueryByID(params)
	executionTime := time.Since(start)

	if err != nil {
		s.logToolCall("run_nqe_query_by_id", args, err)

		// Check for specific NQE query errors and provide helpful messages
		errorStr := err.Error()
		if strings.Contains(errorStr, "Invalid module path") {
			return nil, fmt.Errorf("query contains outdated module imports (this is a data quality issue in the Forward Networks repository) - query ID: %s. Try using find_executable_query to discover alternative queries", args.QueryID)
		}
		if strings.Contains(errorStr, "NQE_RUNTIME_ERROR") {
			return nil, fmt.Errorf("query execution failed due to code issues (this may be a data quality issue) - query ID: %s. Try using find_executable_query to find working alternatives. Error: %w", args.QueryID, err)
		}
		return nil, fmt.Errorf("failed to run NQE query: %w", err)
	}

	// Track the query execution in memory system
	if s.apiTracker != nil {
		if trackErr := s.apiTracker.TrackNetworkQuery(args.QueryID, networkID, snapshotID, result, executionTime); trackErr != nil {
			s.logger.Debug("Failed to track query execution in memory system: %v", trackErr)
		}
	}

	// Store result in cache for future use
	if s.config.Forward.SemanticCache.Enabled && s.semanticCache != nil {
		if cacheErr := s.semanticCache.Put(cacheKey, networkID, snapshotID, result); cacheErr != nil {
			s.logger.Warn("Failed to cache NQE query result for %s: %v", args.QueryID, cacheErr)
		} else {
			s.logger.Debug("Cached result for NQE query %s (items: %d)", args.QueryID, len(result.Items))
		}
	}

	resultJSON := MarshalCompactJSONString(result)
	s.logger.Debug("NQE query completed with %d items", len(result.Items))

	response := fmt.Sprintf("NQE query completed. Found %d items:\n%s\n\n", len(result.Items), resultJSON)

	// Add helpful suggestions for predefined queries
	response += "Would you like to:\n" +
		"1. Run a different predefined query?\n" +
		"2. Create a custom query?\n" +
		"3. Export these results?"

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

func (s *ForwardMCPService) listNQEQueries(args ListNQEQueriesArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("list_nqe_queries", args, nil)

	// Check if query index is ready
	if err := s.checkQueryIndexReady("list_nqe_queries"); err != nil {
		return nil, err
	}

	// Check if query index is initialized
	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	if totalQueries == 0 {
		s.logger.Info("Query index empty, initializing from database...")

		// Try to initialize from database first, then fallback to spec
		if s.database != nil {
			queries, err := s.database.loadWithSmartCachingContext(s.ctx, s.forwardClient, s.logger)
			if err != nil {
				s.logger.Warn("Database loading failed, falling back to spec file: %v", err)
				if err := s.queryIndex.LoadFromSpec(); err != nil {
					return nil, fmt.Errorf("failed to initialize query index: %w", err)
				}
			} else {
				if err := s.queryIndex.LoadFromQueries(queries); err != nil {
					return nil, fmt.Errorf("failed to load queries into index: %w", err)
				}
			}
		} else {
			if err := s.queryIndex.LoadFromSpec(); err != nil {
				return nil, fmt.Errorf("failed to initialize query index: %w", err)
			}
		}
		s.logger.Info("Query index initialized successfully")
	}

	// Use database-backed query index instead of direct API calls
	filteredEntries := s.queryIndex.FilterQueriesByDirectory(args.Directory)

	// Convert NQEQueryIndexEntry to forward.NQEQuery for compatibility
	var queries []forward.NQEQuery
	for _, entry := range filteredEntries {
		queries = append(queries, entry.ConvertToNQEQuery())
	}

	// Format the response with proper JSON structure
	result := MarshalCompactJSONString(queries)

	s.logger.Debug("Found %d valid NQE queries from database index", len(queries))

	// Build a helpful response message
	response := fmt.Sprintf("Found %d NQE queries (from database cache):\n%s\n\n", len(queries), result)

	// Add helpful suggestions based on the results
	if len(queries) == 0 {
		response += "No queries found in the specified directory. Try these common directories:\n" +
			"- /L3/Basic/: Basic network queries\n" +
			"- /L3/Advanced/: Advanced network analysis\n" +
			"- /L3/Security/: Security-related queries\n\n" +
			"Would you like to:\n" +
			"1. Try a different directory?\n" +
			"2. Create a custom query?\n" +
			"3. List all available directories?"
	} else {
		response += "To run a query:\n" +
			"1. Copy the 'queryId' field from the query you want to run\n" +
			"2. Use run_nqe_query_by_id with that queryId\n" +
			"3. Optionally specify limit, offset, or other options\n\n" +
			"Would you like to:\n" +
			"1. Run one of these queries?\n" +
			"2. See more details about a specific query?\n" +
			"3. Try a different directory?"
	}

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// Device Management Tool Implementations
func (s *ForwardMCPService) listDevices(args ListDevicesArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("list_devices", args, nil)

	// Apply default limit if not specified
	limit := args.Limit
	if limit == 0 {
		limit = s.getQueryLimit(0)
	}

	params := &forward.DeviceQueryParams{
		SnapshotID: args.SnapshotID,
		Limit:      limit,
		Offset:     args.Offset,
	}

	response, err := s.forwardClient.GetDevices(args.NetworkID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	// Track device discovery in memory system
	if s.apiTracker != nil {
		if trackErr := s.apiTracker.TrackDeviceDiscovery(args.NetworkID, response.Devices); trackErr != nil {
			s.logger.Debug("Failed to track device discovery in memory system: %v", trackErr)
		}
	}

	result := MarshalCompactJSONString(response)
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d devices (total: %d):\n%s", len(response.Devices), response.TotalCount, result))), nil
}

func (s *ForwardMCPService) getDeviceLocations(args GetDeviceLocationsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_device_locations", args, nil)
	locations, err := s.forwardClient.GetDeviceLocations(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device locations: %w", err)
	}

	result, _ := json.MarshalIndent(locations, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Device locations:\n%s", string(result)))), nil
}

// Snapshot Management Tool Implementations
func (s *ForwardMCPService) listSnapshots(args ListSnapshotsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("list_snapshots", args, nil)
	snapshots, err := s.forwardClient.GetSnapshots(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	result, _ := json.MarshalIndent(snapshots, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d snapshots:\n%s", len(snapshots), string(result)))), nil
}

func (s *ForwardMCPService) getLatestSnapshot(args GetLatestSnapshotArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_latest_snapshot", args, nil)
	snapshot, err := s.forwardClient.GetLatestSnapshot(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	result, _ := json.MarshalIndent(snapshot, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Latest snapshot:\n%s", string(result)))), nil
}

// Location Management Tool Implementations
func (s *ForwardMCPService) listLocations(args ListLocationsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("list_locations", args, nil)
	locations, err := s.forwardClient.GetLocations(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}

	result, _ := json.MarshalIndent(locations, "", "  ")
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d locations:\n%s", len(locations), string(result)))), nil
}

func (s *ForwardMCPService) createLocation(args CreateLocationArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("create_location", args, nil)
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

// resolveNetworkIDByName resolves a network name to its networkId using a case-insensitive match.
func (s *ForwardMCPService) resolveNetworkIDByName(name string) (string, error) {
	networks, err := s.forwardClient.GetNetworks()
	if err != nil {
		return "", err
	}
	var matches []forward.Network
	for _, n := range networks {
		if strings.EqualFold(n.Name, name) {
			matches = append(matches, n)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	} else if len(matches) > 1 {
		return "", fmt.Errorf("multiple networks found with the name '%s'", name)
	}
	return "", fmt.Errorf("no network found with the name '%s'", name)
}

// First-Class Query Tool Implementations - Critical Network Operations
// These wrap the most important predefined queries as dedicated tools

func (s *ForwardMCPService) getDeviceBasicInfo(args GetDeviceBasicInfoArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_device_basic_info", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_ac651cb2901b067fe7dbfb511613ab44776d8029", // Device Basic Info
		Options:    args.Options,
	}

	return s.runNQEQueryByID(queryArgs)
}

func (s *ForwardMCPService) getDeviceHardware(args GetDeviceHardwareArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_device_hardware", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_7ec4a8148b48a91271f342c512b2af1cdb276744", // Device Hardware
		Options:    args.Options,
	}

	return s.runNQEQueryByID(queryArgs)
}

func (s *ForwardMCPService) getHardwareSupport(args GetHardwareSupportArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_hardware_support", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_f0984b777b940b4376ed3ec4317ad47437426e7c", // Hardware Support
		Options:    args.Options,
	}

	return s.runNQEQueryByID(queryArgs)
}

func (s *ForwardMCPService) getOSSupport(args GetOSSupportArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_os_support", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_fc33d9fd70ba19a18455b0e4d26ca8420003d9cc", // OS Support
		Options:    args.Options,
	}

	return s.runNQEQueryByID(queryArgs)
}

func (s *ForwardMCPService) searchConfigs(args SearchConfigsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("search_configs", args, nil)

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    "FQ_e636c47826ad7144f09eaf6bc14dfb0b560e7cc9", // Config Search
		Parameters: map[string]interface{}{
			"searchPattern": args.SearchTerm,
		},
		Options: args.Options,
	}

	return s.runNQEQueryByID(queryArgs)
}

func (s *ForwardMCPService) getConfigDiff(args GetConfigDiffArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_config_diff", args, nil)

	params := map[string]interface{}{}
	if args.AfterSnapshot != "" {
		params["compareSnapshotId"] = args.AfterSnapshot
	}

	queryArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.BeforeSnapshot,
		QueryID:    "FQ_51f090cbea069b4049eb283716ab3bbb3f578aea", // Config Diff
		Parameters: params,
		Options:    args.Options,
	}

	return s.runNQEQueryByID(queryArgs)
}

// Default Settings Management Tool Implementations

func (s *ForwardMCPService) getDefaultSettings(args GetDefaultSettingsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_default_settings", args, nil)

	// Get network name if possible
	networkName := "Not set"
	if s.defaults.NetworkID != "" {
		networks, err := s.forwardClient.GetNetworks()
		if err == nil {
			for _, network := range networks {
				if network.ID == s.defaults.NetworkID {
					networkName = fmt.Sprintf("%s (%s)", network.Name, network.ID)
					break
				}
			}
		}
	}

	settings := map[string]interface{}{
		"default_network_id":   s.defaults.NetworkID,
		"default_network_name": networkName,
		"default_snapshot_id":  s.defaults.SnapshotID,
		"default_query_limit":  s.defaults.QueryLimit,
		"environment_source":   "Loaded from environment variables and config files",
	}

	result := MarshalCompactJSONString(settings)

	response := fmt.Sprintf("Current default settings:\n%s\n\n", result)
	response += "To change defaults:\n"
	response += "• Use set_default_network to change the default network\n"
	response += "• Update environment variables (FORWARD_DEFAULT_NETWORK_ID, etc.)\n"
	response += "• Modify your .env file or config.json\n\n"

	if s.defaults.NetworkID == "" {
		response += " No default network is set. Consider setting FORWARD_DEFAULT_NETWORK_ID in your environment."
	}

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

func (s *ForwardMCPService) setDefaultNetwork(args SetDefaultNetworkArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("set_default_network", args, nil)

	var networkID string
	var networkName string

	// Try to resolve the network identifier (could be ID or name)
	if args.NetworkIdentifier == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("Please provide either a network ID or network name.")), nil
	}

	// First, try as network ID by listing networks and checking if it exists
	networks, err := s.forwardClient.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	// Check if it's a direct network ID match
	for _, network := range networks {
		if network.ID == args.NetworkIdentifier {
			networkID = network.ID
			networkName = network.Name
			break
		}
	}

	// If not found as ID, try to resolve as name
	if networkID == "" {
		resolvedID, err := s.resolveNetworkIDByName(args.NetworkIdentifier)
		if err != nil {
			// List available networks for user reference
			availableNetworks := "Available networks:\n"
			for i, network := range networks {
				availableNetworks += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, network.Name, network.ID)
			}

			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Network '%s' not found.\n\n%s\nPlease use either a valid network ID or exact network name.", args.NetworkIdentifier, availableNetworks))), nil
		}

		networkID = resolvedID
		// Find the network name
		for _, network := range networks {
			if network.ID == networkID {
				networkName = network.Name
				break
			}
		}
	}

	// Update the default (for this session)
	s.defaults.NetworkID = networkID

	response := "Default network updated successfully!\n\n"
	response += fmt.Sprintf("New default: %s (ID: %s)\n\n", networkName, networkID)
	response += "This change applies to the current session. To make it permanent:\n"
	response += fmt.Sprintf("• Set FORWARD_DEFAULT_NETWORK_ID=%s in your environment\n", networkID)
	response += "• Or update your .env file or config.json\n\n"
	response += "All subsequent tool calls will now use this network by default when network_id is not specified."

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// Semantic Cache and AI Enhancement Tool Implementations

// getCacheStats returns semantic cache performance statistics
func (s *ForwardMCPService) getCacheStats(args GetCacheStatsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_cache_stats", args, nil)

	stats := s.semanticCache.GetStats()

	statsJSON := MarshalCompactJSONString(stats)

	summary := fmt.Sprintf("Semantic Cache Performance Statistics:\n%s\n\nCache Summary:\n", statsJSON)
	summary += fmt.Sprintf("• Total Queries: %v\n", stats["total_queries"])
	summary += fmt.Sprintf("• Hit Rate: %v\n", stats["hit_rate_percent"])
	summary += fmt.Sprintf("• Active Entries: %v/%v\n", stats["total_entries"], stats["max_entries"])
	summary += fmt.Sprintf("• Similarity Threshold: %v\n", stats["threshold"])

	return mcp.NewToolResponse(mcp.NewTextContent(summary)), nil
}

// suggestSimilarQueries provides intelligent query suggestions based on cache history
func (s *ForwardMCPService) suggestSimilarQueries(args SuggestSimilarQueriesArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("suggest_similar_queries", args, nil)

	if args.Query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}

	similarQueries, err := s.semanticCache.FindSimilarQueries(args.Query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar queries: %w", err)
	}

	if len(similarQueries) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("No similar queries found for: '%s'\n\nTry running some NQE queries first to build up the cache.", args.Query))), nil
	}

	response := fmt.Sprintf("Similar queries found for: '%s'\n\n", args.Query)
	for i, entry := range similarQueries {
		response += fmt.Sprintf("%d. (%.1f%% similarity) %s\n", i+1, entry.SimilarityScore*100, entry.Query)
		if entry.NetworkID != "" {
			response += fmt.Sprintf("   Network: %s", entry.NetworkID)
			if entry.SnapshotID != "" {
				response += fmt.Sprintf(", Snapshot: %s", entry.SnapshotID)
			}
			response += "\n"
		}
		response += fmt.Sprintf("   Used %d times, last accessed: %s\n\n", entry.AccessCount, entry.LastAccessed.Format("2006-01-02 15:04:05"))
	}

	response += "You can use these suggestions to refine your query or explore related network analysis patterns."

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// clearCache removes expired or all cache entries
func (s *ForwardMCPService) clearCache(args ClearCacheArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("clear_cache", args, nil)

	var removed int
	var operation string

	if args.ClearAll {
		// For simplicity, we'll implement a full clear by creating a new cache
		// In production, you might want a more sophisticated approach
		stats := s.semanticCache.GetStats()
		totalEntries := stats["total_entries"].(int)

		// Reinitialize the cache
		var embeddingService EmbeddingService
		if s.config.Forward.SemanticCache.EmbeddingProvider == "openai" {
			if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
				embeddingService = NewOpenAIEmbeddingService(openaiKey)
			} else {
				embeddingService = NewMockEmbeddingService()
			}
		} else if s.config.Forward.SemanticCache.EmbeddingProvider == "keyword" {
			embeddingService = NewKeywordEmbeddingService()
		} else {
			embeddingService = NewMockEmbeddingService()
		}
		s.semanticCache = NewSemanticCache(embeddingService, s.logger, s.instanceID, &s.config.Forward.SemanticCache)

		removed = totalEntries
		operation = "Cleared all cache entries"
	} else {
		removed = s.semanticCache.ClearExpired()
		operation = "Cleared expired cache entries"
	}

	response := fmt.Sprintf("%s: %d entries removed\n\n", operation, removed)

	// Show updated stats
	newStats := s.semanticCache.GetStats()
	response += "Updated cache status:\n"
	response += fmt.Sprintf("• Active entries: %v\n", newStats["total_entries"])
	response += fmt.Sprintf("• Hit rate: %v\n", newStats["hit_rate_percent"])

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// AI-Powered Query Discovery Tool Implementations

// searchNQEQueries performs AI-powered search through the NQE query library
func (s *ForwardMCPService) searchNQEQueries(args SearchNQEQueriesArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("search_nqe_queries", args, nil)

	// Check if query index is ready
	if err := s.checkQueryIndexReady("search_nqe_queries"); err != nil {
		return nil, err
	}

	if args.Query == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("Please provide a search query describing what you want to analyze (e.g., 'AWS security vulnerabilities', 'BGP routing issues', 'interface statistics')")), nil
	}

	// Set default limit
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}

	// Initialize query index if needed
	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	if totalQueries == 0 {
		s.logger.Info("Query index empty, initializing...")
		if err := s.queryIndex.LoadFromSpec(); err != nil {
			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Failed to initialize query index: %v\n\n**Manual Fix:** Run this command:\n```json\n{\"tool\": \"initialize_query_index\", \"arguments\": {\"generate_embeddings\": false}}\n```", err))), nil
		}
		s.logger.Info("Query index initialized successfully")
	}

	// Use keyword-based search directly
	results, err := s.queryIndex.searchWithKeywords(args.Query, limit)
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Search failed: %v", err))), nil
	}

	// Apply category/subcategory filters if specified
	var filteredResults []*QuerySearchResult
	categoryFilterApplied := args.Category != ""
	subcategoryFilterApplied := args.Subcategory != ""

	for _, result := range results {
		if categoryFilterApplied && !strings.EqualFold(result.Category, args.Category) {
			continue
		}
		if subcategoryFilterApplied && !strings.EqualFold(result.Subcategory, args.Subcategory) {
			continue
		}
		filteredResults = append(filteredResults, result)
	}

	if len(filteredResults) == 0 {
		response := fmt.Sprintf("No exact matches found for: '%s'", args.Query)
		if categoryFilterApplied || subcategoryFilterApplied {
			response += fmt.Sprintf(" (filtered by category: %s, subcategory: %s)", args.Category, args.Subcategory)
			response += "\n\n **Try:**\n• Using broader search terms\n• Removing category filters\n• Running 'get_query_index_stats' to see available categories"
		} else {
			// No filters applied but still no results - provide helpful suggestions
			response += "\n\n**Search Tips:**\n"
			response += "• Try using more general terms (e.g. 'security' instead of 'security vulnerabilities')\n"
			response += "• Break down complex queries into simpler parts\n"
			response += "• Check common categories: Security, L3, Cloud, Interfaces\n"
			response += "• Use related terms (e.g. 'routing' for 'BGP')\n"
			response += "\n**Available Tools:**\n"
			response += "• Run 'get_query_index_stats' to see all categories\n"
			response += "• Try 'find_executable_query' for a different search approach\n"
			response += "• Use 'list_nqe_queries' to browse by directory"
		}
		return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
	}

	// Build response with search type indicator
	searchType := "Keyword-based"
	response := fmt.Sprintf("%s search found %d relevant NQE queries for: '%s'\n\n", searchType, len(filteredResults), args.Query)

	for i, result := range filteredResults {
		response += fmt.Sprintf("**%d. %s** (%.1f%% match)\n", i+1, result.Path, result.SimilarityScore*100)
		response += fmt.Sprintf("   **Intent:** %s\n", result.Intent)
		response += fmt.Sprintf("   **Category:** %s", result.Category)
		if result.Subcategory != "" {
			response += fmt.Sprintf(" → %s", result.Subcategory)
		}
		response += "\n"

		if result.QueryID != "" {
			response += fmt.Sprintf("   **Query ID:** `%s`\n", result.QueryID)
		}

		if args.IncludeCode && result.Code != "" {
			// Truncate code if too long
			code := result.Code
			if len(code) > 300 {
				code = code[:300] + "..."
			}
			response += fmt.Sprintf("   **Code Preview:** ```nqe\n%s\n```\n", code)
		}

		response += "\n"
	}

	response += "**Next Steps:**\n"
	response += "• Use `run_nqe_query_by_id` with a Query ID to execute the query\n"
	response += "• Add `\"include_code\": true` to see NQE source code\n"
	response += "• Try different search terms for more options"

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// initializeQueryIndex builds or rebuilds the AI-powered query index
func (s *ForwardMCPService) initializeQueryIndex(args InitializeQueryIndexArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("initialize_query_index", args, nil)

	response := "🔧 Initializing AI-powered NQE query index...\n\n"

	// Check if spec file exists using robust path resolution
	specPath, err := findSpecFile("NQELibrary.json")
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("NQE spec file not found. Searched in multiple locations but could not locate 'NQELibrary.json'. Error: %v\n\n💡 **Troubleshooting:**\n• Ensure the spec file exists in the 'spec' directory\n• Check that the MCP server is running from the correct directory\n• Verify file permissions", err))), nil
	}

	response += fmt.Sprintf("📁 Found spec file at: %s\n", specPath)

	// Load queries from spec
	response += "📖 Loading NQE queries from spec file...\n"
	if err := s.queryIndex.LoadFromSpec(); err != nil {
		return nil, fmt.Errorf("failed to load query index: %w", err)
	}

	stats := s.queryIndex.GetStatistics()
	totalQueries := stats["total_queries"].(int)
	embeddedQueries := stats["embedded_queries"].(int)

	response += fmt.Sprintf("Loaded %d NQE queries successfully\n", totalQueries)

	if embeddedQueries > 0 {
		coverage := stats["embedding_coverage"].(float64)
		response += fmt.Sprintf("Found %d cached embeddings (%.1f%% coverage) for offline AI search\n", embeddedQueries, coverage*100)
	}
	response += "\n"

	// Generate embeddings if requested
	if args.GenerateEmbeddings {
		if _, ok := s.queryIndex.embeddingService.(*MockEmbeddingService); ok {
			response += "Cannot generate embeddings: OpenAI API key not configured\n"
			response += "Set OPENAI_API_KEY environment variable to enable embedding generation\n"
			response += "Current functionality limited to keyword-based search\n\n"
		} else {
			response += "Generating AI embeddings for semantic search...\n"
			response += "   This will take several minutes for thousands of queries\n"
			response += "   Embeddings will be cached for offline use\n\n"

			if err := s.queryIndex.GenerateEmbeddings(); err != nil {
				if strings.Contains(err.Error(), "cannot generate real embeddings") {
					response += "Embedding generation failed: OpenAI API key required\n"
					response += "   Set FORWARD_EMBEDDING_PROVIDER=keyword for basic functionality\n\n"
				} else {
					return nil, fmt.Errorf("failed to generate embeddings: %w", err)
				}
			} else {
				updatedStats := s.queryIndex.GetStatistics()
				newEmbeddedCount := updatedStats["embedded_queries"].(int)
				newCoverage := updatedStats["embedding_coverage"].(float64)

				response += fmt.Sprintf("Generated and cached %d embeddings (%.1f%% coverage)\n", newEmbeddedCount, newCoverage*100)
				response += "Embeddings saved to spec/nqe-embeddings.json for offline use\n\n"
			}
		}
	}

	// Show final statistics
	finalStats := s.queryIndex.GetStatistics()
	response += "📊 **Query Index Status:**\n"
	response += fmt.Sprintf("• Total queries: %d\n", finalStats["total_queries"].(int))

	if categories, ok := finalStats["categories"].(map[string]int); ok {
		response += "• Categories:\n"
		categoryCount := 0
		for category, count := range categories {
			if category != "" && categoryCount < 5 { // Show top 5 categories
				response += fmt.Sprintf("  - %s: %d queries\n", category, count)
				categoryCount++
			}
		}
		if len(categories) > 5 {
			response += fmt.Sprintf("  - ... and %d more categories\n", len(categories)-5)
		}
	}

	finalEmbedded := finalStats["embedded_queries"].(int)
	if finalEmbedded > 0 {
		finalCoverage := finalStats["embedding_coverage"].(float64)
		response += fmt.Sprintf("• AI embeddings: %d queries (%.1f%% coverage) 🧠\n", finalEmbedded, finalCoverage*100)
		response += "  → Full semantic search available\n"
	} else {
		response += "• AI embeddings: None available\n"
		response += "  → Using keyword-based search fallback\n"
	}

	response += "\n**Query index ready!**\n"
	if finalEmbedded > 0 {
		response += "Use `search_nqe_queries` for AI-powered semantic search\n"
		response += "Works offline with cached embeddings (no OpenAI API calls needed)\n"
	} else {
		response += "Use `search_nqe_queries` for keyword-based search\n"
		response += "Generate embeddings with OpenAI for better semantic matching\n"
	}

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// getQueryIndexStats returns statistics about the query index
func (s *ForwardMCPService) getQueryIndexStats(args GetQueryIndexStatsArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("get_query_index_stats", args, nil)

	stats := s.queryIndex.GetStatistics()

	response := "📊 **NQE Query Index Statistics**\n\n"

	totalQueries := stats["total_queries"].(int)
	if totalQueries == 0 {
		response += "Query index is empty\n"
		response += "Run `initialize_query_index` to load queries from the spec file"
		return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
	}

	response += fmt.Sprintf("📚 **Total Queries:** %d\n", totalQueries)

	if embeddedCount, ok := stats["embedded_queries"].(int); ok {
		coverage := stats["embedding_coverage"].(float64)
		response += fmt.Sprintf("**AI Embeddings:** %d queries (%.1f%% coverage)\n", embeddedCount, coverage*100)

		if embeddedCount == 0 {
			response += "Run `initialize_query_index` with `generate_embeddings: true` for AI search\n"
		}
	}

	if categories, ok := stats["categories"].(map[string]int); ok && args.Detailed {
		response += "\n**Query Categories:**\n"

		// Sort categories by count
		type categoryCount struct {
			name  string
			count int
		}
		sortedCategories := make([]categoryCount, 0, len(categories))
		for category, count := range categories {
			if category != "" {
				sortedCategories = append(sortedCategories, categoryCount{category, count})
			}
		}
		sort.Slice(sortedCategories, func(i, j int) bool {
			return sortedCategories[i].count > sortedCategories[j].count
		})

		// Display categories with subcategories
		if subcategories, ok := stats["subcategories"].(map[string]map[string]int); ok {
			for _, cat := range sortedCategories {
				response += fmt.Sprintf("• **%s** (%d queries)\n", cat.name, cat.count)

				if subCats, exists := subcategories[cat.name]; exists && len(subCats) > 0 {
					// Sort subcategories
					type subCategoryCount struct {
						name  string
						count int
					}
					sortedSubCats := make([]subCategoryCount, 0, len(subCats))
					for subCat, count := range subCats {
						if subCat != "" {
							sortedSubCats = append(sortedSubCats, subCategoryCount{subCat, count})
						}
					}
					sort.Slice(sortedSubCats, func(i, j int) bool {
						return sortedSubCats[i].count > sortedSubCats[j].count
					})

					// Show top 5 subcategories
					for i, subCat := range sortedSubCats {
						if i >= 5 {
							response += fmt.Sprintf("    ... and %d more subcategories\n", len(sortedSubCats)-5)
							break
						}
						response += fmt.Sprintf("    - %s (%d queries)\n", subCat.name, subCat.count)
					}
				}
			}
		}
	} else if categories, ok := stats["categories"].(map[string]int); ok {
		response += fmt.Sprintf("\n📂 **Categories:** %d total", len(categories))
		if len(categories) > 0 {
			response += " (use `detailed: true` to see breakdown)"
		}
		response += "\n"
	}

	response += "\n🔍 **Available Tools:**\n"
	response += "• `search_nqe_queries` - AI-powered query search\n"
	response += "• `initialize_query_index` - Rebuild index with latest spec\n"
	response += "• `get_query_index_stats` - View these statistics"

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

// findExecutableQuery performs intelligent query discovery using semantic search + executable mapping
func (s *ForwardMCPService) findExecutableQuery(args FindExecutableQueryArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("find_executable_query", args, nil)

	if args.Query == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("Please describe what you want to analyze (e.g., 'show me all BGP neighbors', 'find devices with high CPU', 'check configuration compliance')")), nil
	}

	// Set default limit for semantic search
	semanticLimit := 20 // Search more broadly first
	if args.Limit > 0 {
		semanticLimit = args.Limit * 3 // Search 3x more to have options for mapping
	}

	// Track if auto-initialization happened
	var autoInitResponse string

	// Step 1: Use semantic search to find relevant queries from full database
	semanticResults, err := s.queryIndex.SearchQueries(args.Query, semanticLimit)
	if err != nil {
		if strings.Contains(err.Error(), "query index is empty") {
			// Auto-initialize the query index for better user experience
			s.logger.Info("Query index empty in find_executable_query, auto-initializing...")

			autoInitResponse = "🔧 Query index not initialized. Auto-initializing now...\n\n"

			if err := s.queryIndex.LoadFromSpec(); err != nil {
				return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Auto-initialization failed: %v\n\n**Manual Fix:** Run this command:\n```json\n{\"tool\": \"initialize_query_index\", \"arguments\": {\"generate_embeddings\": false}}\n```", err))), nil
			}

			autoInitResponse += "Query index loaded successfully! Retrying your search...\n\n"

			// Retry the search after initialization
			semanticResults, err = s.queryIndex.SearchQueries(args.Query, semanticLimit)
			if err != nil {
				return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("%sSearch failed after auto-initialization: %v", autoInitResponse, err))), nil
			}

			// Continue with search results processing
		} else if strings.Contains(err.Error(), "no embeddings available") {
			return mcp.NewToolResponse(mcp.NewTextContent("No embeddings available for AI search. Run 'initialize_query_index' with 'generate_embeddings: true' for best results, or use keyword-based search.")), nil
		} else {
			return nil, fmt.Errorf("failed to search queries: %w", err)
		}
	}

	if len(semanticResults) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("No relevant queries found for: '%s'\n\n💡 Try:\n• Using different search terms\n• Being more specific about what you want to analyze\n• Running 'get_query_index_stats' to see available categories", args.Query))), nil
	}

	// Step 2: Map semantic results to executable queries
	mappings := MapSemanticToExecutable(semanticResults)

	if len(mappings) == 0 {
		// No direct mappings found, show semantic results with explanation
		response := fmt.Sprintf("Found %d relevant queries for '%s', but none map to currently executable queries.\n\n", len(semanticResults), args.Query)
		response += "**Related queries found:**\n"

		displayLimit := 5
		if len(semanticResults) < displayLimit {
			displayLimit = len(semanticResults)
		}

		for i := 0; i < displayLimit; i++ {
			result := semanticResults[i]
			response += fmt.Sprintf("• **%s** (%.1f%% match)\n", result.Path, result.SimilarityScore*100)
			response += fmt.Sprintf("  Intent: %s\n", result.Intent)
			if result.QueryID != "" {
				response += fmt.Sprintf("  QueryID: %s (may not be executable)\n", result.QueryID)
			}
			response += "\n"
		}

		response += "💡 **Currently available executable queries:**\n"
		execQueries := GetExecutableQueries()
		for _, eq := range execQueries {
			response += fmt.Sprintf("• **%s** - %s\n", eq.Name, eq.Description)
		}

		return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
	}

	// Step 3: Build response with executable recommendations
	searchType := "AI-semantic"
	if len(semanticResults) > 0 && semanticResults[0].MatchType == "keyword" {
		searchType = "Keyword-based"
	}

	response := ""

	// Include auto-initialization message if it occurred
	if autoInitResponse != "" {
		response += autoInitResponse
	}

	response += fmt.Sprintf("%s search found %d executable queries for: '%s'\n\n", searchType, len(mappings), args.Query)

	// Apply user-specified limit
	displayLimit := len(mappings)
	if args.Limit > 0 && args.Limit < displayLimit {
		displayLimit = args.Limit
	}

	for i := 0; i < displayLimit; i++ {
		mapping := mappings[i]
		eq := mapping.ExecutableQuery

		response += fmt.Sprintf("**%d. %s** (%.1f%% confidence)\n", i+1, eq.Name, mapping.MappingConfidence*100)
		response += fmt.Sprintf("   **Query ID:** `%s`\n", eq.QueryID)
		response += fmt.Sprintf("   **Purpose:** %s\n", eq.Description)
		response += fmt.Sprintf("   **When to use:** %s\n", eq.WhenToUse)
		response += fmt.Sprintf("   **Mapping reason:** %s\n", mapping.MappingReason)

		if args.IncludeRelated && len(mapping.SemanticMatches) > 0 {
			response += fmt.Sprintf("   **Related queries found:** %d\n", len(mapping.SemanticMatches))
			for j, match := range mapping.SemanticMatches {
				if j >= 3 { // Show max 3 related queries
					response += fmt.Sprintf("     ... and %d more\n", len(mapping.SemanticMatches)-3)
					break
				}
				response += fmt.Sprintf("     • %s (%.1f%% similarity)\n", match.Path, match.SimilarityScore*100)
			}
		}
		response += "\n"
	}

	if displayLimit < len(mappings) {
		response += fmt.Sprintf("... and %d more executable queries. Use `limit: %d` to see more.\n\n", len(mappings)-displayLimit, len(mappings))
	}

	response += "**Next Steps:**\n"
	response += "• Copy a Query ID and use `run_nqe_query_by_id` to execute\n"
	response += "• Use the dedicated tools (e.g., `get_device_basic_info`) for easier execution\n"
	response += "• Add `include_related: true` to see the semantic matches that led to these recommendations\n"

	if searchType == "🔍 Keyword-based" {
		response += "• Generate embeddings with `initialize_query_index` for better AI semantic matching\n"
	}

	return mcp.NewToolResponse(mcp.NewTextContent(response)), nil
}

func (s *ForwardMCPService) smartQueryWorkflow(args SmartQueryWorkflowArgs) (*mcp.PromptResponse, error) {
	s.logToolCall("smart_query_workflow", args, nil)

	workflowGuide := `# Smart Query Discovery Workflow Guide

This guide helps you discover and execute Forward Networks queries effectively.

## 1️⃣ Find Relevant Queries

Use one of these search tools:

**Option A: Quick Discovery (Recommended)**
` + "```json" + `
{
  "tool": "find_executable_query",
  "arguments": {
    "query": "describe what you want to analyze",
    "limit": 5
  }
}
` + "```" + `

**Option B: Detailed Search**
` + "```json" + `
{
  "tool": "search_nqe_queries",
  "arguments": {
    "query": "your search terms",
    "limit": 10,
    "include_code": true
  }
}
` + "```" + `

## 2️⃣ Execute Queries

Once you find a query you like:

` + "```json" + `
{
  "tool": "run_nqe_query_by_id",
  "arguments": {
    "query_id": "FQ_...",
    "options": {
      "limit": 100
    }
  }
}
` + "```" + `

## 💡 Tips

- Be specific in your search terms
- Try different phrasings if needed
- Use category filters for focused results
- Check query code before running
- Start with small result limits

## 🔍 Example Workflow

1. Search: "Find BGP routing problems"
2. Review suggested queries
3. Execute most relevant query
4. Analyze results
5. Refine search if needed

Need help? Just ask for guidance at any step!`

	return mcp.NewPromptResponse("Smart Query Workflow", mcp.NewPromptMessage(mcp.NewTextContent(workflowGuide), mcp.RoleAssistant)), nil
}

// TestSemanticCacheArgs defines arguments for the test_semantic_cache tool
// (add this near other tool argument structs)
type TestSemanticCacheArgs struct {
	Query      string `json:"query"`
	NetworkID  string `json:"network_id"`
	SnapshotID string `json:"snapshot_id"`
}

// testSemanticCache demonstrates semantic cache usage
func (s *ForwardMCPService) testSemanticCache(args TestSemanticCacheArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("test_semantic_cache", args, nil)

	// Try to get from cache
	cached, found := s.semanticCache.Get(args.Query, args.NetworkID, args.SnapshotID)
	if found {
		s.logger.Info("[CACHE HIT] query='%s' network_id='%s' snapshot_id='%s'", args.Query, args.NetworkID, args.SnapshotID)
		return mcp.NewToolResponse(mcp.NewTextContent(
			"[CACHE HIT] Returning cached result for query: " + args.Query + "\n" +
				fmt.Sprintf("Result: %+v", cached),
		)), nil
	}

	// Simulate generating a result
	result := &forward.NQERunResult{
		SnapshotID: args.SnapshotID,
		Items: []map[string]interface{}{
			{"message": "Simulated result for query: " + args.Query},
		},
	}

	// Store in cache
	err := s.semanticCache.Put(args.Query, args.NetworkID, args.SnapshotID, result)
	if err != nil {
		return nil, fmt.Errorf("Failed to store result in cache: %w", err)
	}

	s.logger.Info("[CACHE MISS] query='%s' network_id='%s' snapshot_id='%s' (cached new result)", args.Query, args.NetworkID, args.SnapshotID)

	return mcp.NewToolResponse(mcp.NewTextContent(
		"[CACHE MISS] Generated and cached result for query: " + args.Query + "\n" +
			fmt.Sprintf("Result: %+v", result),
	)), nil
}

// RunSemanticNQEQueryArgs defines arguments for the run_semantic_nqe_query tool
// (add this near other tool argument structs)
type RunSemanticNQEQueryArgs struct {
	Query      string           `json:"query"`
	NetworkID  string           `json:"network_id"`
	SnapshotID string           `json:"snapshot_id"`
	Options    *NQEQueryOptions `json:"options"`
}

// runSemanticNQEQuery implements the handler for the run_semantic_nqe_query tool
func (s *ForwardMCPService) runSemanticNQEQuery(args RunSemanticNQEQueryArgs) (*mcp.ToolResponse, error) {
	s.logToolCall("run_semantic_nqe_query", args, nil)

	if args.Query == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("Please provide a natural language query describing what you want to analyze.")), nil
	}

	// Use semantic search to find the best matching query
	results, err := s.queryIndex.SearchQueries(args.Query, 1)
	if err != nil || len(results) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent("No relevant NQE query found for your description.")), nil
	}
	bestQuery := results[0]

	// Run the best matching query by ID
	runArgs := RunNQEQueryByIDArgs{
		NetworkID:  args.NetworkID,
		SnapshotID: args.SnapshotID,
		QueryID:    bestQuery.QueryID,
		Options:    args.Options,
	}
	return s.runNQEQueryByID(runArgs)
}

// promptForParameter prompts the user for a required parameter in the workflow
func (s *ForwardMCPService) promptForParameter(sessionID, paramName string) (*mcp.ToolResponse, error) {
	var promptText string
	switch paramName {
	case "network_id":
		promptText = "Please provide a network ID (use list_networks to see available networks):"
	case "snapshot_id":
		promptText = "Please provide a snapshot ID (or type 'latest' for the most recent):"
	default:
		promptText = "Please provide a value for " + paramName + ":"
	}
	return mcp.NewToolResponse(mcp.NewTextContent(promptText)), nil
}

// Helper function to check if query index is ready and provide helpful feedback
func (s *ForwardMCPService) checkQueryIndexReady(toolName string) error {
	if !s.queryIndex.IsReady() {
		if s.queryIndex.IsLoading() {
			return fmt.Errorf("🚀 Query index is currently loading in the background. Please wait a moment and try again. This usually takes 10-30 seconds for initial startup")
		}
		return fmt.Errorf("❌ Query index is not initialized. This may indicate a startup issue. Try running 'initialize_query_index' tool to manually initialize")
	}
	return nil
}

// hydrateDatabase hydrates the database by loading queries from the Forward Networks API
func (s *ForwardMCPService) hydrateDatabase(args HydrateDatabaseArgs) (*mcp.ToolResponse, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database is not available")
	}

	// Set defaults
	if args.MaxRetries == 0 {
		args.MaxRetries = 3
	}

	s.logger.Info("🔄 Starting database hydration...")

	// Check if we need to force refresh or if database is empty
	existingQueries, err := s.database.LoadQueries()
	if err != nil {
		s.logger.Warn("🔄 Failed to load existing queries: %v", err)
		existingQueries = []forward.NQEQueryDetail{}
	}

	if len(existingQueries) > 0 && !args.ForceRefresh {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Database already contains %d queries. Use force_refresh=true to refresh anyway.", len(existingQueries)))), nil
	}

	// Create context for the operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Load queries from API
	var queries []forward.NQEQueryDetail
	if args.EnhancedMode {
		// Use enhanced mode with metadata
		existingCommitIDs := make(map[string]string)
		for _, query := range existingQueries {
			if query.LastCommit.ID != "" {
				existingCommitIDs[query.Path] = query.LastCommit.ID
			}
		}

		queries, err = s.forwardClient.GetNQEAllQueriesEnhancedWithCacheContext(ctx, existingCommitIDs)
		if err != nil {
			s.logger.Warn("🔄 Enhanced API failed, falling back to basic API: %v", err)
			queries, err = s.database.loadFromBasicAPI(s.forwardClient, s.logger)
		}
	} else {
		queries, err = s.database.loadFromBasicAPI(s.forwardClient, s.logger)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load queries from API: %w", err)
	}

	// Merge with existing queries if not force refresh
	if !args.ForceRefresh && len(existingQueries) > 0 {
		queries = s.database.mergeQueries(existingQueries, queries)
	}

	// Save to database
	if err := s.database.SaveQueries(queries); err != nil {
		return nil, fmt.Errorf("failed to save queries to database: %w", err)
	}

	// Update last sync time
	if err := s.database.SetMetadata("last_sync", time.Now().Format(time.RFC3339)); err != nil {
		s.logger.Warn("🔄 Failed to update sync time: %v", err)
	}

	s.logger.Info("🔄 Database hydration completed with %d queries", len(queries))

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Database hydration completed successfully. Loaded %d queries from API.", len(queries)))), nil
}

// refreshQueryIndex refreshes the query index from the current database content
func (s *ForwardMCPService) refreshQueryIndex(args RefreshQueryIndexArgs) (*mcp.ToolResponse, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database is not available")
	}

	if s.queryIndex == nil {
		return nil, fmt.Errorf("query index is not available")
	}

	s.logger.Info("🔄 Refreshing query index from database...")

	// Load queries from database
	queries, err := s.database.LoadQueries()
	if err != nil {
		return nil, fmt.Errorf("failed to load queries from database: %w", err)
	}

	if len(queries) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent("No queries found in database. Use hydrate_database to load queries first.")), nil
	}

	// Load queries into index
	if err := s.queryIndex.LoadFromQueries(queries); err != nil {
		return nil, fmt.Errorf("failed to load queries into index: %w", err)
	}

	s.logger.Info("🔄 Query index refreshed with %d queries", len(queries))

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Query index refreshed successfully with %d queries.", len(queries)))), nil
}

// getDatabaseStatus returns the current status of the database and query index
func (s *ForwardMCPService) getDatabaseStatus(args GetDatabaseStatusArgs) (*mcp.ToolResponse, error) {
	status := map[string]interface{}{
		"database_available":    s.database != nil,
		"query_index_available": s.queryIndex != nil,
		"timestamp":             time.Now().Format(time.RFC3339),
	}

	if s.database != nil {
		// Get database stats
		queries, err := s.database.LoadQueries()
		if err != nil {
			status["database_error"] = err.Error()
			status["query_count"] = 0
		} else {
			status["query_count"] = len(queries)
		}

		// Get last sync time
		if lastSync, err := s.database.GetMetadata("last_sync"); err == nil {
			status["last_sync"] = lastSync
		}

		// Get database path
		status["database_path"] = s.database.dbPath
	}

	if s.queryIndex != nil {
		// Get query index stats
		status["query_index_empty"] = !s.queryIndex.IsReady()
		status["query_index_loading"] = s.queryIndex.IsLoading()

		// Get index stats if available
		indexStats := s.queryIndex.GetStatistics()
		if indexStats != nil {
			status["index_stats"] = indexStats
		}
	}

	// Marshal to JSON for pretty output
	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal status: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(statusJSON))), nil
}

// Memory Management Tool Implementations

// createEntity creates a new entity in the knowledge graph
func (s *ForwardMCPService) createEntity(args CreateEntityArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	entity, err := s.memorySystem.CreateEntity(args.Name, args.Type, args.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	entityJSON, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entity: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Entity created successfully:\n%s", string(entityJSON)))), nil
}

// createRelation creates a relation between two entities
func (s *ForwardMCPService) createRelation(args CreateRelationArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	relation, err := s.memorySystem.CreateRelation(args.FromID, args.ToID, args.Type, args.Properties)
	if err != nil {
		return nil, fmt.Errorf("failed to create relation: %w", err)
	}

	relationJSON, err := json.MarshalIndent(relation, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal relation: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Relation created successfully:\n%s", string(relationJSON)))), nil
}

// addObservation adds an observation to an entity
func (s *ForwardMCPService) addObservation(args AddObservationArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	observation, err := s.memorySystem.AddObservation(args.EntityID, args.Content, args.Type, args.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to add observation: %w", err)
	}

	observationJSON, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal observation: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Observation added successfully:\n%s", string(observationJSON)))), nil
}

// searchEntities searches for entities in the knowledge graph
func (s *ForwardMCPService) searchEntities(args SearchEntitiesArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	entities, err := s.memorySystem.SearchEntities(args.Query, args.EntityType, args.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}

	if len(entities) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent("No entities found matching the search criteria.")), nil
	}

	entitiesJSON, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entities: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d entities:\n%s", len(entities), string(entitiesJSON)))), nil
}

// getEntity retrieves a specific entity by ID or name
func (s *ForwardMCPService) getEntity(args GetEntityArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	entity, err := s.memorySystem.GetEntity(args.Identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	entityJSON, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entity: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Entity found:\n%s", string(entityJSON)))), nil
}

// getRelations retrieves relations for an entity
func (s *ForwardMCPService) getRelations(args GetRelationsArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	relations, err := s.memorySystem.GetRelations(args.EntityID, args.RelationType)
	if err != nil {
		return nil, fmt.Errorf("failed to get relations: %w", err)
	}

	if len(relations) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent("No relations found for this entity.")), nil
	}

	relationsJSON, err := json.MarshalIndent(relations, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal relations: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d relations:\n%s", len(relations), string(relationsJSON)))), nil
}

// getObservations retrieves observations for an entity
func (s *ForwardMCPService) getObservations(args GetObservationsArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	observations, err := s.memorySystem.GetObservations(args.EntityID, args.ObservationType)
	if err != nil {
		return nil, fmt.Errorf("failed to get observations: %w", err)
	}

	if len(observations) == 0 {
		return mcp.NewToolResponse(mcp.NewTextContent("No observations found for this entity.")), nil
	}

	observationsJSON, err := json.MarshalIndent(observations, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal observations: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Found %d observations:\n%s", len(observations), string(observationsJSON)))), nil
}

// deleteEntity deletes an entity and all its relations and observations
func (s *ForwardMCPService) deleteEntity(args DeleteEntityArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	// Get entity details before deletion for confirmation
	entity, err := s.memorySystem.GetEntity(args.EntityID)
	if err != nil {
		return nil, fmt.Errorf("entity not found: %w", err)
	}

	err = s.memorySystem.DeleteEntity(args.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete entity: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Entity '%s' (%s) deleted successfully, including all its relations and observations.", entity.Name, entity.Type))), nil
}

// deleteRelation deletes a specific relation
func (s *ForwardMCPService) deleteRelation(args DeleteRelationArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	err := s.memorySystem.DeleteRelation(args.RelationID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete relation: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Relation '%s' deleted successfully.", args.RelationID))), nil
}

// deleteObservation deletes a specific observation
func (s *ForwardMCPService) deleteObservation(args DeleteObservationArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	err := s.memorySystem.DeleteObservation(args.ObservationID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete observation: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Observation '%s' deleted successfully.", args.ObservationID))), nil
}

// getMemoryStats returns statistics about the memory system
func (s *ForwardMCPService) getMemoryStats(args GetMemoryStatsArgs) (*mcp.ToolResponse, error) {
	if s.memorySystem == nil {
		return nil, fmt.Errorf("memory system is not available")
	}

	stats, err := s.memorySystem.GetMemoryStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Memory system statistics:\n%s", string(statsJSON)))), nil
}

// getQueryAnalytics gets analytics about query patterns for a network
func (s *ForwardMCPService) getQueryAnalytics(args GetQueryAnalyticsArgs) (*mcp.ToolResponse, error) {
	if s.apiTracker == nil {
		return nil, fmt.Errorf("API memory tracker is not available")
	}

	analytics, err := s.apiTracker.GetQueryAnalytics(args.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get query analytics: %w", err)
	}

	analyticsJSON, err := json.MarshalIndent(analytics, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analytics: %w", err)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Query analytics for network %s:\n%s", args.NetworkID, string(analyticsJSON)))), nil
}
