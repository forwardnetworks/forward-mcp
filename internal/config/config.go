package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/forward-mcp/internal/logger"
	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server  ServerConfig
	Forward ForwardConfig
	MCP     MCPConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port int
	Host string
}

// ForwardConfig holds Forward Networks API configuration
type ForwardConfig struct {
	APIKey            string `json:"apiKey" env:"FORWARD_API_KEY"`
	APISecret         string `json:"apiSecret" env:"FORWARD_API_SECRET"`
	APIBaseURL        string `json:"apiBaseUrl" env:"FORWARD_API_BASE_URL"`
	DefaultNetworkID  string `json:"defaultNetworkId" env:"FORWARD_DEFAULT_NETWORK_ID"`
	DefaultSnapshotID string `json:"defaultSnapshotId" env:"FORWARD_DEFAULT_SNAPSHOT_ID"`
	DefaultQueryLimit int    `json:"defaultQueryLimit" env:"FORWARD_DEFAULT_QUERY_LIMIT"`

	// TLS Configuration
	InsecureSkipVerify bool   `json:"insecureSkipVerify" env:"FORWARD_INSECURE_SKIP_VERIFY"`
	CACertPath         string `json:"caCertPath" env:"FORWARD_CA_CERT_PATH"`
	ClientCertPath     string `json:"clientCertPath" env:"FORWARD_CLIENT_CERT_PATH"`
	ClientKeyPath      string `json:"clientKeyPath" env:"FORWARD_CLIENT_KEY_PATH"`
	Timeout            int    `json:"timeout" env:"FORWARD_TIMEOUT"`

	// Semantic Cache Configuration
	SemanticCache SemanticCacheConfig `json:"semanticCache"`
}

// CacheEvictionPolicy defines the eviction strategy
type CacheEvictionPolicy string

const (
	EvictionPolicyLRU        CacheEvictionPolicy = "lru"        // Least Recently Used
	EvictionPolicyLFU        CacheEvictionPolicy = "lfu"        // Least Frequently Used  
	EvictionPolicyTTL        CacheEvictionPolicy = "ttl"        // Time To Live based
	EvictionPolicySize       CacheEvictionPolicy = "size"       // Size-based eviction
	EvictionPolicyOldest     CacheEvictionPolicy = "oldest"     // Oldest first (default)
	EvictionPolicyRandom     CacheEvictionPolicy = "random"     // Random eviction
)

// SemanticCacheConfig holds semantic cache configuration
type SemanticCacheConfig struct {
	Enabled             bool                 `json:"enabled" env:"FORWARD_SEMANTIC_CACHE_ENABLED"`
	MaxEntries          int                  `json:"maxEntries" env:"FORWARD_SEMANTIC_CACHE_MAX_ENTRIES"`
	TTLHours            int                  `json:"ttlHours" env:"FORWARD_SEMANTIC_CACHE_TTL_HOURS"`
	SimilarityThreshold float64              `json:"similarityThreshold" env:"FORWARD_SEMANTIC_CACHE_SIMILARITY_THRESHOLD"`
	EmbeddingProvider   string               `json:"embeddingProvider" env:"FORWARD_EMBEDDING_PROVIDER"`
	
	// Enhanced cache configuration for large API results
	MaxMemoryMB         int                  `json:"maxMemoryMB" env:"FORWARD_SEMANTIC_CACHE_MAX_MEMORY_MB"`
	EvictionPolicy      CacheEvictionPolicy  `json:"evictionPolicy" env:"FORWARD_SEMANTIC_CACHE_EVICTION_POLICY"`
	CompressResults     bool                 `json:"compressResults" env:"FORWARD_SEMANTIC_CACHE_COMPRESS_RESULTS"`
	CompressionLevel    int                  `json:"compressionLevel" env:"FORWARD_SEMANTIC_CACHE_COMPRESSION_LEVEL"`
	PersistToDisk       bool                 `json:"persistToDisk" env:"FORWARD_SEMANTIC_CACHE_PERSIST_TO_DISK"`
	DiskCachePath       string               `json:"diskCachePath" env:"FORWARD_SEMANTIC_CACHE_DISK_PATH"`
	MetricsEnabled      bool                 `json:"metricsEnabled" env:"FORWARD_SEMANTIC_CACHE_METRICS_ENABLED"`
	
	// Eviction thresholds
	MemoryEvictionThreshold float64           `json:"memoryEvictionThreshold" env:"FORWARD_SEMANTIC_CACHE_MEMORY_THRESHOLD"`
	CleanupIntervalMinutes  int              `json:"cleanupIntervalMinutes" env:"FORWARD_SEMANTIC_CACHE_CLEANUP_INTERVAL"`
}

// MCPConfig holds MCP-specific configuration
type MCPConfig struct {
	Version    string
	MaxRetries int
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() *Config {
	// Try to load .env file (fail silently if not found)
	loadEnvFile()

	config := &Config{
		Server: ServerConfig{
			Port: getEnvAsInt("SERVER_PORT", 8080),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		Forward: ForwardConfig{
			APIKey:             getEnv("FORWARD_API_KEY", ""),
			APISecret:          getEnv("FORWARD_API_SECRET", ""),
			APIBaseURL:         getEnv("FORWARD_API_BASE_URL", ""),
			Timeout:            getEnvAsInt("FORWARD_TIMEOUT", 30),
			InsecureSkipVerify: getEnvAsBool("FORWARD_INSECURE_SKIP_VERIFY", false),
			CACertPath:         getEnv("FORWARD_CA_CERT_PATH", ""),
			ClientCertPath:     getEnv("FORWARD_CLIENT_CERT_PATH", ""),
			ClientKeyPath:      getEnv("FORWARD_CLIENT_KEY_PATH", ""),
			DefaultNetworkID:   getEnv("FORWARD_DEFAULT_NETWORK_ID", ""),
			DefaultSnapshotID:  getEnv("FORWARD_DEFAULT_SNAPSHOT_ID", ""),
			DefaultQueryLimit:  getEnvAsInt("FORWARD_DEFAULT_QUERY_LIMIT", 10000),
			SemanticCache: SemanticCacheConfig{
				Enabled:             getEnvAsBool("FORWARD_SEMANTIC_CACHE_ENABLED", true),
				MaxEntries:          getEnvAsInt("FORWARD_SEMANTIC_CACHE_MAX_ENTRIES", 1000),
				TTLHours:            getEnvAsInt("FORWARD_SEMANTIC_CACHE_TTL_HOURS", 24),
				SimilarityThreshold: getEnvAsFloat("FORWARD_SEMANTIC_CACHE_SIMILARITY_THRESHOLD", 0.85),
				EmbeddingProvider:   getEnv("FORWARD_EMBEDDING_PROVIDER", "openai"),
				
				// Enhanced cache configuration defaults
				MaxMemoryMB:             getEnvAsInt("FORWARD_SEMANTIC_CACHE_MAX_MEMORY_MB", 512),  // 512MB default
				EvictionPolicy:          CacheEvictionPolicy(getEnv("FORWARD_SEMANTIC_CACHE_EVICTION_POLICY", "lru")),
				CompressResults:         getEnvAsBool("FORWARD_SEMANTIC_CACHE_COMPRESS_RESULTS", true),
				CompressionLevel:        getEnvAsInt("FORWARD_SEMANTIC_CACHE_COMPRESSION_LEVEL", 6),  // Gzip level 6 (balanced)
				PersistToDisk:           getEnvAsBool("FORWARD_SEMANTIC_CACHE_PERSIST_TO_DISK", false),
				DiskCachePath:           getEnv("FORWARD_SEMANTIC_CACHE_DISK_PATH", "/tmp/forward-cache"),
				MetricsEnabled:          getEnvAsBool("FORWARD_SEMANTIC_CACHE_METRICS_ENABLED", true),
				MemoryEvictionThreshold: getEnvAsFloat("FORWARD_SEMANTIC_CACHE_MEMORY_THRESHOLD", 0.8),  // 80%
				CleanupIntervalMinutes:  getEnvAsInt("FORWARD_SEMANTIC_CACHE_CLEANUP_INTERVAL", 30),
			},
		},
		MCP: MCPConfig{
			Version:    getEnv("MCP_VERSION", "v1"),
			MaxRetries: getEnvAsInt("MCP_MAX_RETRIES", 3),
		},
	}

	// Try to load JSON config file
	if err := loadJSONConfig(config); err != nil {
		debugLogger := logger.New()
		debugLogger.Debug("Could not load JSON config file: %v", err)
	}

	return config
}

// loadEnvFile loads environment variables from .env file
func loadEnvFile() {
	if err := godotenv.Load(); err != nil {
		debugLogger := logger.New()
		debugLogger.Debug("Could not load .env file: %v", err)
	}
}

// loadJSONConfig loads configuration from a JSON file
func loadJSONConfig(config *Config) error {
	// Try to find config file in common locations
	configPaths := []string{
		"config.json",
		"examples/config.json",
		"/etc/forward-mcp/config.json",
	}

	var configFile []byte
	var err error
	for _, path := range configPaths {
		configFile, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("could not find config file in any location: %w", err)
	}

	// Parse JSON config
	var jsonConfig struct {
		Forward ForwardConfig `json:"forward"`
	}
	if err := json.Unmarshal(configFile, &jsonConfig); err != nil {
		return fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// Update config with JSON values if they are not empty
	if jsonConfig.Forward.APIKey != "" {
		config.Forward.APIKey = jsonConfig.Forward.APIKey
	}
	if jsonConfig.Forward.APISecret != "" {
		config.Forward.APISecret = jsonConfig.Forward.APISecret
	}
	if jsonConfig.Forward.APIBaseURL != "" {
		config.Forward.APIBaseURL = jsonConfig.Forward.APIBaseURL
	}
	if jsonConfig.Forward.DefaultNetworkID != "" {
		config.Forward.DefaultNetworkID = jsonConfig.Forward.DefaultNetworkID
	}
	if jsonConfig.Forward.DefaultSnapshotID != "" {
		config.Forward.DefaultSnapshotID = jsonConfig.Forward.DefaultSnapshotID
	}
	if jsonConfig.Forward.DefaultQueryLimit > 0 {
		config.Forward.DefaultQueryLimit = jsonConfig.Forward.DefaultQueryLimit
	}

	return nil
}

// Helper function to get environment variable with default
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Helper function to get environment variable as int with default
func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Helper function to get environment variable as bool with default
func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		lowerValue := strings.ToLower(strings.TrimSpace(value))
		return lowerValue == "true" || lowerValue == "1" || lowerValue == "yes" || lowerValue == "on"
	}
	return defaultValue
}

// Helper function to get environment variable as float with default
func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
