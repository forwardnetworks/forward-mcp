package service

import (
	"crypto/md5"
	"encoding/hex"
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

// hashString creates a short, stable hash of a string
func hashString(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	hash := hex.EncodeToString(hasher.Sum(nil))
	// Return first 8 characters for readability
	return hash[:8]
}
