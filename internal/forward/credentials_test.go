package forward

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forward-mcp/internal/config"
	"github.com/joho/godotenv"
)

// getProjectRoot returns the absolute path to the project root.
func getProjectRoot() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

func TestForwardAPICredentials(t *testing.T) {
	rootDir := getProjectRoot()
	envPath := filepath.Join(rootDir, ".env")
	t.Logf("Looking for .env at: %s", envPath)

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Skip(".env file not found - skipping API credentials test")
	}
	_ = godotenv.Load(envPath)

	apiKey := os.Getenv("FORWARD_API_KEY")
	apiSecret := os.Getenv("FORWARD_API_SECRET")
	apiBaseURL := os.Getenv("FORWARD_API_BASE_URL")

	// Mask sensitive credentials in logs
	maskedKey := ""
	if apiKey != "" {
		if len(apiKey) > 8 {
			maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
		} else {
			maskedKey = "****"
		}
	}
	maskedSecret := ""
	if apiSecret != "" {
		maskedSecret = "****"
	}
	t.Logf("API_KEY: %q, API_SECRET: %q, API_BASE_URL: %q", maskedKey, maskedSecret, apiBaseURL)

	if apiKey == "" || apiSecret == "" || apiBaseURL == "" {
		t.Skip("FORWARD_API_KEY, FORWARD_API_SECRET, and FORWARD_API_BASE_URL must be set to run this test")
	}

	client := NewClient(&config.ForwardConfig{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		APIBaseURL: apiBaseURL,
		Timeout:    10,
	})

	_, err := client.GetAvailableModels()
	if err != nil {
		t.Fatalf("API credentials test failed: %v", err)
	}
}
