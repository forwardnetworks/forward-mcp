package config

import (
	"log"
	"os"
	"strconv"
	"strings"

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

// ForwardConfig holds the Forward API configuration
type ForwardConfig struct {
	APIKey             string `env:"FORWARD_API_KEY,required"`
	APISecret          string `env:"FORWARD_API_SECRET,required"`
	APIBaseURL         string `env:"FORWARD_API_BASE_URL,required"`
	Timeout            int    `env:"FORWARD_TIMEOUT,default=30"`
	InsecureSkipVerify bool   `env:"FORWARD_INSECURE_SKIP_VERIFY,default=false"`
	CACertPath         string `env:"FORWARD_CA_CERT_PATH"`
	ClientCertPath     string `env:"FORWARD_CLIENT_CERT_PATH"`
	ClientKeyPath      string `env:"FORWARD_CLIENT_KEY_PATH"`
}

// MCPConfig holds MCP-specific configuration
type MCPConfig struct {
	Version    string
	MaxRetries int
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() *Config {
	// Try to load .env file (fail silently if not found)
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	return &Config{
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
		},
		MCP: MCPConfig{
			Version:    getEnv("MCP_VERSION", "v1"),
			MaxRetries: getEnvAsInt("MCP_MAX_RETRIES", 3),
		},
	}
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
