package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger wraps the standard logger with level control and file output
type Logger struct {
	infoLogger  *log.Logger
	debugLogger *log.Logger
	fileLogger  *log.Logger
	debugMode   bool
	logFile     *os.File
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Tool      string                 `json:"tool,omitempty"`
	Duration  string                 `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// New creates a new logger instance with optional file output
func New() *Logger {
	// Check for debug mode from environment
	debugMode := isDebugEnabled()

	// Create loggers with appropriate prefixes
	infoLogger := log.New(os.Stderr, "[INFO] ", log.LstdFlags)
	debugLogger := log.New(os.Stderr, "[DEBUG] ", log.LstdFlags|log.Lshortfile)

	logger := &Logger{
		infoLogger:  infoLogger,
		debugLogger: debugLogger,
		debugMode:   debugMode,
	}

	// Setup file logging if requested
	if logPath := getLogFilePath(); logPath != "" {
		if err := logger.setupFileLogging(logPath); err != nil {
			// Log to stderr if file setup fails
			fmt.Fprintf(os.Stderr, "[WARN] Failed to setup file logging: %v\n", err)
		}
	}

	return logger
}

// setupFileLogging configures file output for structured logs
func (l *Logger) setupFileLogging(logPath string) error {
	// Create log directory if it doesn't exist
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file with append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.logFile = file
	l.fileLogger = log.New(file, "", 0) // No prefix for JSON logs

	// Log initialization
	l.logToFile("info", "Logger initialized with file output", "", nil, nil)
	return nil
}

// getLogFilePath returns the log file path from environment or default
func getLogFilePath() string {
	if path := os.Getenv("FORWARD_MCP_LOG_FILE"); path != "" {
		return path
	}
	if path := os.Getenv("MCP_LOG_FILE"); path != "" {
		return path
	}
	// Default path based on documentation recommendation
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".forward-mcp", "server.log")
	}
	return ""
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

// logToFile writes a structured log entry to the file
func (l *Logger) logToFile(level, message, tool string, duration *time.Duration, context map[string]interface{}) {
	if l.fileLogger == nil {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Tool:      tool,
		Context:   context,
	}

	if duration != nil {
		entry.Duration = duration.String()
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		l.fileLogger.Printf(`{"timestamp":"%s","level":"%s","message":"JSON marshal error: %v","original_message":"%s"}`,
			time.Now().Format(time.RFC3339), "error", err, message)
		return
	}

	l.fileLogger.Println(string(jsonData))
}

// Info logs informational messages (always shown)
func (l *Logger) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.infoLogger.Printf("%s", message)
	l.logToFile("info", message, "", nil, nil)
}

// Debug logs debug messages (only shown if debug mode is enabled)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.debugMode {
		message := fmt.Sprintf(format, args...)
		l.debugLogger.Printf("%s", message)
		l.logToFile("debug", message, "", nil, nil)
	}
}

// Error logs error messages (always shown)
func (l *Logger) Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.infoLogger.Printf("%s", "[ERROR] "+message)
	l.logToFile("error", message, "", nil, nil)
}

// Fatalf logs an error message and exits the program
func (l *Logger) Fatalf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.infoLogger.Printf("%s", "[FATAL] "+message)
	l.logToFile("fatal", message, "", nil, nil)
	l.Close() // Ensure file is closed before exit
	os.Exit(1)
}

// Warn logs warning messages (always shown)
func (l *Logger) Warn(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.infoLogger.Printf("%s", "[WARN] "+message)
	l.logToFile("warn", message, "", nil, nil)
}

// IsDebugEnabled returns whether debug mode is active
func (l *Logger) IsDebugEnabled() bool {
	return l.debugMode
}

// SetDebugMode allows runtime control of debug mode
func (l *Logger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
}

// Enhanced logging methods following MCP debugging best practices

// LogToolCall logs a tool call with performance metrics and context
func (l *Logger) LogToolCall(toolName string, args interface{}, duration time.Duration, err error) {
	context := map[string]interface{}{
		"tool_name": toolName,
		"duration":  duration.String(),
	}

	// Add args if debug mode is enabled
	if l.debugMode {
		if argsJSON, jsonErr := json.Marshal(args); jsonErr == nil {
			context["args"] = string(argsJSON)
		}
	}

	if err != nil {
		context["error"] = err.Error()
		message := fmt.Sprintf("Tool %s failed (duration: %v): %v", toolName, duration, err)
		l.Error("%s", message)
		l.logToFile("error", message, toolName, &duration, context)
	} else {
		message := fmt.Sprintf("Tool %s completed successfully (duration: %v)", toolName, duration)
		l.Debug("%s", message)
		if l.debugMode {
			l.logToFile("debug", message, toolName, &duration, context)
		}
	}
}

// LogResourceAccess logs resource access for debugging
func (l *Logger) LogResourceAccess(resourceURI string, operation string, success bool, details interface{}) {
	context := map[string]interface{}{
		"resource_uri": resourceURI,
		"operation":    operation,
		"success":      success,
	}

	if details != nil {
		context["details"] = details
	}

	message := fmt.Sprintf("Resource access: %s [%s] - success: %t", resourceURI, operation, success)

	if success {
		l.Debug("%s", message)
		if l.debugMode {
			l.logToFile("debug", message, "", nil, context)
		}
	} else {
		l.Warn("%s", message)
		l.logToFile("warn", message, "", nil, context)
	}
}

// LogInitialization logs server initialization events
func (l *Logger) LogInitialization(component string, status string, details interface{}) {
	context := map[string]interface{}{
		"component": component,
		"status":    status,
	}

	if details != nil {
		context["details"] = details
	}

	message := fmt.Sprintf("Initialization: %s - %s", component, status)
	l.Info("%s", message)
	l.logToFile("info", message, "", nil, context)
}

// LogPerformanceMetric logs performance metrics
func (l *Logger) LogPerformanceMetric(operation string, duration time.Duration, metadata interface{}) {
	context := map[string]interface{}{
		"operation": operation,
		"duration":  duration.String(),
	}

	if metadata != nil {
		context["metadata"] = metadata
	}

	message := fmt.Sprintf("Performance: %s took %v", operation, duration)

	// Only log to console if significant or in debug mode
	if duration > time.Second || l.debugMode {
		l.Debug("%s", message)
	}

	// Always log to file for performance tracking
	l.logToFile("performance", message, "", &duration, context)
}

// LogCacheMetrics logs cache performance metrics
func (l *Logger) LogCacheMetrics(operation string, hit bool, cacheType string, metadata interface{}) {
	hitMiss := "MISS"
	if hit {
		hitMiss = "HIT"
	}

	context := map[string]interface{}{
		"operation":  operation,
		"cache_hit":  hit,
		"cache_type": cacheType,
	}

	if metadata != nil {
		context["metadata"] = metadata
	}

	message := fmt.Sprintf("Cache %s: %s [%s]", hitMiss, operation, cacheType)
	l.Debug("%s", message)

	if l.debugMode {
		l.logToFile("cache", message, "", nil, context)
	}
}

// Close properly closes the log file
func (l *Logger) Close() error {
	if l.logFile != nil {
		l.logToFile("info", "Logger shutting down", "", nil, nil)
		return l.logFile.Close()
	}
	return nil
}
