// Package envconfig loads configuration from the environment, a .env file and
// an optional config.json.
package envconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/forward-mcp/internal/ports"
	"github.com/joho/godotenv"
)

// The configuration types this adapter fills, named through the port.
type (
	Config              = ports.Config
	ForwardConfig       = ports.ForwardConfig
	SemanticCacheConfig = ports.SemanticCacheConfig
	CacheEvictionPolicy = ports.CacheEvictionPolicy
	MCPConfig           = ports.MCPConfig
	ServerConfig        = ports.ServerConfig
	HTTPConfig          = ports.HTTPConfig
)

// Load reads configuration from environment variables, a .env file and an
// optional config.json. It refuses settings that would weaken TLS.
func Load(log ports.Logger) (*Config, error) {
	// Try to load .env file (fail silently if not found)
	loadEnvFile(log)

	config := &Config{
		Server: ServerConfig{
			Port: getEnvAsInt("SERVER_PORT", 8080),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		Forward: ForwardConfig{
			APIKey:            getEnv("FORWARD_API_KEY", ""),
			APISecret:         getEnv("FORWARD_API_SECRET", ""),
			APIBaseURL:        getEnv("FORWARD_API_BASE_URL", ""),
			Timeout:           getEnvAsInt("FORWARD_TIMEOUT", 600), // 10 minutes for enhanced API operations
			CACertPath:        getEnv("FORWARD_CA_CERT_PATH", ""),
			ClientCertPath:    getEnv("FORWARD_CLIENT_CERT_PATH", ""),
			ClientKeyPath:     getEnv("FORWARD_CLIENT_KEY_PATH", ""),
			DefaultNetworkID:  getEnv("FORWARD_DEFAULT_NETWORK_ID", ""),
			DefaultSnapshotID: getEnv("FORWARD_DEFAULT_SNAPSHOT_ID", ""),
			DefaultQueryLimit: getEnvAsInt("FORWARD_DEFAULT_QUERY_LIMIT", 10000),
			SemanticCache: SemanticCacheConfig{
				Enabled:             getEnvAsBool("FORWARD_SEMANTIC_CACHE_ENABLED", true),
				MaxEntries:          getEnvAsInt("FORWARD_SEMANTIC_CACHE_MAX_ENTRIES", 1000),
				TTLHours:            getEnvAsInt("FORWARD_SEMANTIC_CACHE_TTL_HOURS", 24),
				SimilarityThreshold: getEnvAsFloat("FORWARD_SEMANTIC_CACHE_SIMILARITY_THRESHOLD", 0.85),
				EmbeddingProvider:   getEnv("FORWARD_EMBEDDING_PROVIDER", "openai"),

				// Enhanced cache configuration defaults
				MaxMemoryMB:             getEnvAsInt("FORWARD_SEMANTIC_CACHE_MAX_MEMORY_MB", 512), // 512MB default
				EvictionPolicy:          CacheEvictionPolicy(getEnv("FORWARD_SEMANTIC_CACHE_EVICTION_POLICY", "lru")),
				CompressResults:         getEnvAsBool("FORWARD_SEMANTIC_CACHE_COMPRESS_RESULTS", true),
				CompressionLevel:        getEnvAsInt("FORWARD_SEMANTIC_CACHE_COMPRESSION_LEVEL", 6), // Gzip level 6 (balanced)
				PersistToDisk:           getEnvAsBool("FORWARD_SEMANTIC_CACHE_PERSIST_TO_DISK", false),
				DiskCachePath:           getEnv("FORWARD_SEMANTIC_CACHE_DISK_PATH", "/tmp/forward-cache"),
				MetricsEnabled:          getEnvAsBool("FORWARD_SEMANTIC_CACHE_METRICS_ENABLED", true),
				MemoryEvictionThreshold: getEnvAsFloat("FORWARD_SEMANTIC_CACHE_MEMORY_THRESHOLD", 0.8), // 80%
				CleanupIntervalMinutes:  getEnvAsInt("FORWARD_SEMANTIC_CACHE_CLEANUP_INTERVAL", 30),
			},
		},
		MCP: MCPConfig{
			Version:    getEnv("MCP_VERSION", "v1"),
			MaxRetries: getEnvAsInt("MCP_MAX_RETRIES", 3),
		},
		HTTP: HTTPConfig{
			Enabled:         getEnvAsBool("FORWARD_HTTP_ENABLED", false),
			Port:            getEnvAsInt("FORWARD_HTTP_PORT", 8080),
			Host:            getEnv("FORWARD_HTTP_HOST", "0.0.0.0"),
			TLSCertFile:     getEnv("FORWARD_HTTP_TLS_CERT", ""),
			TLSKeyFile:      getEnv("FORWARD_HTTP_TLS_KEY", ""),
			AllowInsecure:   getEnvAsBool("FORWARD_HTTP_ALLOW_INSECURE", false),
			AuthMode:        getEnv("FORWARD_HTTP_AUTH_MODE", "api-key"),
			JWTIssuer:       getEnv("FORWARD_HTTP_JWT_ISSUER", ""),
			JWTAudience:     getEnv("FORWARD_HTTP_JWT_AUDIENCE", "forward-mcp"),
			JWTPublicKeyURL: getEnv("FORWARD_HTTP_JWT_PUBLIC_KEY_URL", ""),
			JWKSCACertPath:  getEnv("FORWARD_HTTP_JWKS_CA_CERT", ""),
			APIKeys:         parseAPIKeys(getEnv("FORWARD_HTTP_API_KEYS", "")),
			CORSOrigins:     parseStringList(getEnv("FORWARD_HTTP_CORS_ORIGINS", "")),
			RateLimit:       getEnvAsInt("FORWARD_HTTP_RATE_LIMIT", 100),
			MaxConnections:  getEnvAsInt("FORWARD_HTTP_MAX_CONNECTIONS", 100),
			ReadTimeout:     getEnvAsInt("FORWARD_HTTP_READ_TIMEOUT", 30),
			WriteTimeout:    getEnvAsInt("FORWARD_HTTP_WRITE_TIMEOUT", 30),
		},
	}

	// SECURITY: Check for deprecated InsecureSkipVerify setting
	if getEnvAsBool("FORWARD_INSECURE_SKIP_VERIFY", false) {
		return nil, fmt.Errorf("SECURITY ERROR: FORWARD_INSECURE_SKIP_VERIFY is no longer supported. " +
			"This setting bypasses TLS certificate validation and enables man-in-the-middle attacks. " +
			"Remove this environment variable. For custom CA certificates, use FORWARD_CA_CERT_PATH instead")
	}

	// Try to load JSON config file
	if err := loadJSONConfig(config); err != nil {
		log.Debug("Could not load JSON config file: %v", err)
	}

	if err := validate(config, log); err != nil {
		return nil, err
	}
	return config, nil
}

// loadEnvFile loads environment variables from .env file
func loadEnvFile(log ports.Logger) {
	if err := godotenv.Load(); err != nil {
		log.Debug("Could not load .env file: %v", err)
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

// parseAPIKeys parses API keys from "key1:user1,key2:user2" format
func parseAPIKeys(s string) map[string]string {
	if s == "" {
		return make(map[string]string)
	}

	result := make(map[string]string)
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			user := strings.TrimSpace(parts[1])
			if key != "" && user != "" {
				result[key] = user
			}
		}
	}
	return result
}

// parseStringList parses comma-separated string list
func parseStringList(s string) []string {
	if s == "" {
		return nil
	}

	result := make([]string, 0)
	items := strings.Split(s, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
