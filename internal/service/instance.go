package service

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/forward-mcp/internal/domain"
	"net/url"
	"strings"
)

// GenerateInstanceID creates a stable, unique identifier for a Forward Networks instance
// based on the API base URL. This ensures database and cache partitioning between instances.
func GenerateInstanceID(apiBaseURL string) string {
	if apiBaseURL == "" {
		return "default"
	}

	// Parse the URL to extract the host
	parsed, err := url.Parse(apiBaseURL)
	if err != nil {
		// If parsing fails, use the raw URL
		return hashString(apiBaseURL)
	}

	// Use the host (domain + port) as the basis for the instance ID
	// This ensures different instances (e.g., customer1.fwd.app vs customer2.fwd.app) get different IDs
	host := strings.ToLower(parsed.Host)
	if host == "" {
		host = strings.ToLower(apiBaseURL)
	}

	return hashString(host)
}

// hashString creates a short, stable hash of a string using SHA-256
func hashString(s string) string {
	hasher := sha256.New()
	hasher.Write([]byte(s))
	hash := hex.EncodeToString(hasher.Sum(nil))
	// Return first 16 characters for better collision resistance while keeping it readable
	return hash[:16]
}

// InstanceID returns the configured instance ID, or one derived from the API
// base URL when none is configured.
func InstanceID(cfg *domain.Config) string {
	if cfg.Forward.InstanceID != "" {
		return cfg.Forward.InstanceID
	}
	return GenerateInstanceID(cfg.Forward.APIBaseURL)
}
