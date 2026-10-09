package ports

import "github.com/forward-mcp/internal/domain"

// Configuration values the adapters are built from.
type (
	Config              = domain.Config
	ServerConfig        = domain.ServerConfig
	ForwardConfig       = domain.ForwardConfig
	SemanticCacheConfig = domain.SemanticCacheConfig
	CacheEvictionPolicy = domain.CacheEvictionPolicy
	MCPConfig           = domain.MCPConfig
	HTTPConfig          = domain.HTTPConfig
)

// Cache eviction policies.
const (
	EvictionPolicyLRU    = domain.EvictionPolicyLRU
	EvictionPolicyLFU    = domain.EvictionPolicyLFU
	EvictionPolicyTTL    = domain.EvictionPolicyTTL
	EvictionPolicySize   = domain.EvictionPolicySize
	EvictionPolicyOldest = domain.EvictionPolicyOldest
	EvictionPolicyRandom = domain.EvictionPolicyRandom
)

// Version is the forward-mcp release.
const Version = domain.Version
