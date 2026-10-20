package envconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// A certificate saved by `forward-mcp-server trust` is trusted with no
// configuration, and FORWARD_CA_CERT_PATH still wins when it is set.
func TestDefaultTrustFileUsedWhenNoCAPathSet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FORWARD_CA_CERT_PATH", "")

	cfg, err := Load(nopLog{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Forward.CACertPath != "" {
		t.Fatalf("no trust file yet, but CACertPath = %q", cfg.Forward.CACertPath)
	}

	saved := filepath.Join(home, ".forward-mcp", "ca.pem")
	if err := os.MkdirAll(filepath.Dir(saved), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(selfSignedPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(saved, data, 0644); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(nopLog{}); err != nil {
		t.Fatal(err)
	}
	if cfg.Forward.CACertPath != saved {
		t.Errorf("CACertPath = %q, want the saved trust file %q", cfg.Forward.CACertPath, saved)
	}

	explicit := selfSignedPEM(t)
	t.Setenv("FORWARD_CA_CERT_PATH", explicit)
	if cfg, err = Load(nopLog{}); err != nil {
		t.Fatal(err)
	}
	if cfg.Forward.CACertPath != explicit {
		t.Errorf("FORWARD_CA_CERT_PATH did not take precedence: %q", cfg.Forward.CACertPath)
	}
}
