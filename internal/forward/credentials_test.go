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

	// Load configuration using the proper config loader
	cfg := config.LoadConfig()

	// Mask sensitive credentials in logs
	maskedKey := ""
	if cfg.Forward.APIKey != "" {
		if len(cfg.Forward.APIKey) > 8 {
			maskedKey = cfg.Forward.APIKey[:4] + "****" + cfg.Forward.APIKey[len(cfg.Forward.APIKey)-4:]
		} else {
			maskedKey = "****"
		}
	}
	maskedSecret := ""
	if cfg.Forward.APISecret != "" {
		maskedSecret = "****"
	}
	t.Logf("API_KEY: %q, API_SECRET: %q, API_BASE_URL: %q, INSECURE_SKIP_VERIFY: %v",
		maskedKey, maskedSecret, cfg.Forward.APIBaseURL, cfg.Forward.InsecureSkipVerify)

	if cfg.Forward.APIKey == "" || cfg.Forward.APISecret == "" || cfg.Forward.APIBaseURL == "" {
		t.Skip("FORWARD_API_KEY, FORWARD_API_SECRET, and FORWARD_API_BASE_URL must be set to run this test")
	}

	client := NewClient(&cfg.Forward)

	// Test credentials by calling a real Forward Networks API endpoint
	networks, err := client.GetNetworks()
	if err != nil {
		t.Fatalf("API credentials test failed: %v", err)
	}

	t.Logf("Successfully authenticated - found %d networks", len(networks))
}
