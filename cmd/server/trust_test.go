package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trustEnv starts a self-signed server and points the configuration at it,
// with a fresh home directory so the default trust file starts empty.
func trustEnv(t *testing.T) (home, fingerprint string) {
	t.Helper()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(ts.Close)
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FORWARD_API_BASE_URL", ts.URL)
	t.Setenv("FORWARD_CA_CERT_PATH", "")
	t.Setenv("FORWARD_API_KEY", "k")
	t.Setenv("FORWARD_API_SECRET", "s")
	sum := sha256.Sum256(ts.Certificate().Raw)
	return home, fmt.Sprintf("%x", sum[:])
}

func TestTrustSavesAfterConfirmation(t *testing.T) {
	home, _ := trustEnv(t)
	var out bytes.Buffer
	if code := runTrust(nil, strings.NewReader("y\n"), &out); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out.String())
	}
	saved := filepath.Join(home, ".forward-mcp", "ca.pem")
	if _, err := os.Stat(saved); err != nil {
		t.Fatalf("certificate not saved: %v\n%s", err, out.String())
	}
	for _, want := range []string{"SHA-256:", "Saved to", "reads this file automatically", "is now trusted"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	// The default file is now loaded, so a second run has nothing to do.
	out.Reset()
	if code := runTrust(nil, strings.NewReader(""), &out); code != 0 || !strings.Contains(out.String(), "already trusted") {
		t.Errorf("second run: exit %d:\n%s", code, out.String())
	}
}

func TestTrustDeclinedSavesNothing(t *testing.T) {
	home, _ := trustEnv(t)
	var out bytes.Buffer
	if code := runTrust(nil, strings.NewReader("n\n"), &out); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(home, ".forward-mcp", "ca.pem")); !os.IsNotExist(err) {
		t.Errorf("a declined certificate was saved (stat err %v)", err)
	}
}

func TestTrustFingerprintFlag(t *testing.T) {
	home, fp := trustEnv(t)
	var out bytes.Buffer
	if code := runTrust([]string{"--fingerprint", strings.Repeat("00", 32)}, strings.NewReader(""), &out); code != 1 {
		t.Errorf("wrong fingerprint: exit %d, want 1:\n%s", code, out.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".forward-mcp", "ca.pem")); !os.IsNotExist(err) {
		t.Error("a certificate with the wrong fingerprint was saved")
	}

	out.Reset()
	if code := runTrust([]string{"--fingerprint", fp}, strings.NewReader(""), &out); code != 0 {
		t.Errorf("matching fingerprint: exit %d:\n%s", code, out.String())
	}
}
