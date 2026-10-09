package ports

import "time"

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
