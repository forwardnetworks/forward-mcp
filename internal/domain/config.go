package domain

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

// RedisConfig holds Redis-specific configuration for distributed caching and storage
type RedisConfig struct {
	Enabled  bool   `json:"enabled" env:"REDIS_ENABLED"`
	Address  string `json:"address" env:"REDIS_ADDRESS"`
	Port     int    `json:"port" env:"REDIS_PORT"`
	Password string `json:"password" env:"REDIS_PASSWORD"`
	Database int    `json:"database" env:"REDIS_DATABASE"`
	PoolSize int    `json:"poolSize" env:"REDIS_POOL_SIZE"`

	// TLS Configuration
	TLSEnabled bool   `json:"tlsEnabled" env:"REDIS_TLS_ENABLED"`
	CACertPath string `json:"caCertPath" env:"REDIS_CA_CERT_PATH"`

	// Connection settings
	MaxRetries      int `json:"maxRetries" env:"REDIS_MAX_RETRIES"`
	MinIdleConns    int `json:"minIdleConns" env:"REDIS_MIN_IDLE_CONNS"`
	ConnMaxIdleTime int `json:"connMaxIdleTime" env:"REDIS_CONN_MAX_IDLE_TIME"` // seconds
	ConnMaxLifetime int `json:"connMaxLifetime" env:"REDIS_CONN_MAX_LIFETIME"`  // seconds
}
