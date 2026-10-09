package ports

import (
	"time"

	"github.com/forward-mcp/internal/domain"
)

// Logger writes diagnostics. It never writes to stdout, which carries the MCP
// protocol.
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	// LogToolCall records one tool invocation, its duration and its error.
	LogToolCall(toolName string, args interface{}, duration time.Duration, err error)
}

// Configuration values the adapters are built from.
type (
	Config              = domain.Config
	ServerConfig        = domain.ServerConfig
	ForwardConfig       = domain.ForwardConfig
	SemanticCacheConfig = domain.SemanticCacheConfig
	CacheEvictionPolicy = domain.CacheEvictionPolicy
	MCPConfig           = domain.MCPConfig
)
