package ports

import "github.com/forward-mcp/internal/domain"

// Logger writes diagnostics. It never writes to stdout, which carries the MCP
// protocol.
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// Configuration values the adapters are built from.
type (
	ForwardConfig       = domain.ForwardConfig
	SemanticCacheConfig = domain.SemanticCacheConfig
)
