package logger

import (
	"log"
	"os"
	"strings"
)

// Logger wraps the standard logger with level control
type Logger struct {
	infoLogger  *log.Logger
	debugLogger *log.Logger
	debugMode   bool
}

// New creates a new logger instance
func New() *Logger {
	// Check for debug mode from environment
	debugMode := isDebugEnabled()

	// Create loggers with appropriate prefixes
	infoLogger := log.New(os.Stderr, "[INFO] ", log.LstdFlags)
	debugLogger := log.New(os.Stderr, "[DEBUG] ", log.LstdFlags|log.Lshortfile)

	return &Logger{
		infoLogger:  infoLogger,
		debugLogger: debugLogger,
		debugMode:   debugMode,
	}
}

// isDebugEnabled checks environment variables for debug mode
func isDebugEnabled() bool {
	debug := os.Getenv("DEBUG")
	if debug == "" {
		debug = os.Getenv("FORWARD_MCP_DEBUG")
	}

	// Accept various truthy values
	switch strings.ToLower(debug) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Info logs informational messages (always shown)
func (l *Logger) Info(format string, args ...interface{}) {
	l.infoLogger.Printf(format, args...)
}

// Debug logs debug messages (only shown if debug mode is enabled)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.debugMode {
		l.debugLogger.Printf(format, args...)
	}
}

// Error logs error messages (always shown)
func (l *Logger) Error(format string, args ...interface{}) {
	l.infoLogger.Printf("[ERROR] "+format, args...)
}

// Fatalf logs an error message and exits the program
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.infoLogger.Printf("[FATAL] "+format, args...)
	os.Exit(1)
}

// Warn logs warning messages (always shown)
func (l *Logger) Warn(format string, args ...interface{}) {
	l.infoLogger.Printf("[WARN] "+format, args...)
}

// IsDebugEnabled returns whether debug mode is active
func (l *Logger) IsDebugEnabled() bool {
	return l.debugMode
}

// SetDebugMode allows runtime control of debug mode
func (l *Logger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
}
