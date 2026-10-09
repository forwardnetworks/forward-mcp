package domain

// Config holds all configuration for the application
type Config struct {
	Server  ServerConfig
	Forward ForwardConfig
	MCP     MCPConfig
	HTTP    HTTPConfig
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

	// Instance Configuration
	InstanceID string `json:"instanceId" env:"FORWARD_INSTANCE_ID"`

	// TLS Configuration
	// SECURITY: InsecureSkipVerify has been removed as it allows MITM attacks
	// Use CACertPath for custom CA certificates if needed
	CACertPath     string `json:"caCertPath" env:"FORWARD_CA_CERT_PATH"`
	ClientCertPath string `json:"clientCertPath" env:"FORWARD_CLIENT_CERT_PATH"`
	ClientKeyPath  string `json:"clientKeyPath" env:"FORWARD_CLIENT_KEY_PATH"`
	Timeout        int    `json:"timeout" env:"FORWARD_TIMEOUT"`

	// Semantic Cache Configuration
	SemanticCache SemanticCacheConfig `json:"semanticCache"`
}

// CacheEvictionPolicy defines the eviction strategy
type CacheEvictionPolicy string

const (
	EvictionPolicyLRU    CacheEvictionPolicy = "lru"    // Least Recently Used
	EvictionPolicyLFU    CacheEvictionPolicy = "lfu"    // Least Frequently Used
	EvictionPolicyTTL    CacheEvictionPolicy = "ttl"    // Time To Live based
	EvictionPolicySize   CacheEvictionPolicy = "size"   // Size-based eviction
	EvictionPolicyOldest CacheEvictionPolicy = "oldest" // Oldest first (default)
	EvictionPolicyRandom CacheEvictionPolicy = "random" // Random eviction
)

// SemanticCacheConfig holds semantic cache configuration
type SemanticCacheConfig struct {
	Enabled             bool    `json:"enabled" env:"FORWARD_SEMANTIC_CACHE_ENABLED"`
	MaxEntries          int     `json:"maxEntries" env:"FORWARD_SEMANTIC_CACHE_MAX_ENTRIES"`
	TTLHours            int     `json:"ttlHours" env:"FORWARD_SEMANTIC_CACHE_TTL_HOURS"`
	SimilarityThreshold float64 `json:"similarityThreshold" env:"FORWARD_SEMANTIC_CACHE_SIMILARITY_THRESHOLD"`
	EmbeddingProvider   string  `json:"embeddingProvider" env:"FORWARD_EMBEDDING_PROVIDER"`

	// Enhanced cache configuration for large API results
	MaxMemoryMB      int                 `json:"maxMemoryMB" env:"FORWARD_SEMANTIC_CACHE_MAX_MEMORY_MB"`
	EvictionPolicy   CacheEvictionPolicy `json:"evictionPolicy" env:"FORWARD_SEMANTIC_CACHE_EVICTION_POLICY"`
	CompressResults  bool                `json:"compressResults" env:"FORWARD_SEMANTIC_CACHE_COMPRESS_RESULTS"`
	CompressionLevel int                 `json:"compressionLevel" env:"FORWARD_SEMANTIC_CACHE_COMPRESSION_LEVEL"`
	PersistToDisk    bool                `json:"persistToDisk" env:"FORWARD_SEMANTIC_CACHE_PERSIST_TO_DISK"`
	DiskCachePath    string              `json:"diskCachePath" env:"FORWARD_SEMANTIC_CACHE_DISK_PATH"`
	MetricsEnabled   bool                `json:"metricsEnabled" env:"FORWARD_SEMANTIC_CACHE_METRICS_ENABLED"`

	// Eviction thresholds
	MemoryEvictionThreshold float64 `json:"memoryEvictionThreshold" env:"FORWARD_SEMANTIC_CACHE_MEMORY_THRESHOLD"`
	CleanupIntervalMinutes  int     `json:"cleanupIntervalMinutes" env:"FORWARD_SEMANTIC_CACHE_CLEANUP_INTERVAL"`
}

// MCPConfig holds MCP-specific configuration
type MCPConfig struct {
	Version    string
	MaxRetries int
}

// HTTPConfig holds HTTP/SSE server configuration
type HTTPConfig struct {
	Enabled         bool              `json:"enabled" env:"FORWARD_HTTP_ENABLED"`
	Port            int               `json:"port" env:"FORWARD_HTTP_PORT"`
	Host            string            `json:"host" env:"FORWARD_HTTP_HOST"`
	TLSCertFile     string            `json:"tlsCertFile" env:"FORWARD_HTTP_TLS_CERT"`
	TLSKeyFile      string            `json:"tlsKeyFile" env:"FORWARD_HTTP_TLS_KEY"`
	AuthMode        string            `json:"authMode" env:"FORWARD_HTTP_AUTH_MODE"` // "jwt", "api-key", "none"
	JWTIssuer       string            `json:"jwtIssuer" env:"FORWARD_HTTP_JWT_ISSUER"`
	JWTAudience     string            `json:"jwtAudience" env:"FORWARD_HTTP_JWT_AUDIENCE"`
	JWTPublicKeyURL string            `json:"jwtPublicKeyUrl" env:"FORWARD_HTTP_JWT_PUBLIC_KEY_URL"`
	APIKeys         map[string]string `json:"apiKeys"` // key -> username (loaded from env)
	CORSOrigins     []string          `json:"corsOrigins"`
	RateLimit       int               `json:"rateLimit" env:"FORWARD_HTTP_RATE_LIMIT"`           // requests per minute per user
	MaxConnections  int               `json:"maxConnections" env:"FORWARD_HTTP_MAX_CONNECTIONS"` // max concurrent SSE connections
	ReadTimeout     int               `json:"readTimeout" env:"FORWARD_HTTP_READ_TIMEOUT"`
	WriteTimeout    int               `json:"writeTimeout" env:"FORWARD_HTTP_WRITE_TIMEOUT"`
}
