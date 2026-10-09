package forwardapi

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forward-mcp/internal/ports"
)

// selfSignedForward stands in for an on-prem Forward server whose
// certificate is self-signed, and writes that certificate to a PEM file.
func selfSignedForward(t *testing.T) (url, certFile string) {
	t.Helper()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"101","name":"lab"}]`))
	}))
	t.Cleanup(ts.Close)
	certFile = filepath.Join(t.TempDir(), "forward.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	if err := os.WriteFile(certFile, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return ts.URL, certFile
}

func clientFor(url, caPath string) *Client {
	return NewClient(&ports.ForwardConfig{
		APIKey: "k", APISecret: "s", APIBaseURL: url, Timeout: 5, CACertPath: caPath,
	}, nopLogger{})
}

func TestSelfSignedCertRejectedWithActionableError(t *testing.T) {
	url, _ := selfSignedForward(t)
	_, err := clientFor(url, "").GetNetworks(context.Background())
	if err == nil {
		t.Fatal("a self-signed certificate was accepted without being trusted")
	}
	if !strings.Contains(err.Error(), "FORWARD_CA_CERT_PATH") {
		t.Errorf("error does not tell the user how to fix it: %v", err)
	}
}

func TestSelfSignedCertTrustedViaCACertPath(t *testing.T) {
	url, certFile := selfSignedForward(t)
	networks, err := clientFor(url, certFile).GetNetworks(context.Background())
	if err != nil {
		t.Fatalf("trusted self-signed certificate was rejected: %v", err)
	}
	if len(networks) != 1 || networks[0].ID != "101" {
		t.Fatalf("got %+v", networks)
	}
}

func TestLoadRootCAs(t *testing.T) {
	_, certFile := selfSignedForward(t)
	if _, err := LoadRootCAs(certFile); err != nil {
		t.Errorf("valid PEM: %v", err)
	}
	if _, err := LoadRootCAs(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Error("missing file: want error")
	}
	junk := filepath.Join(t.TempDir(), "junk.pem")
	_ = os.WriteFile(junk, []byte("not a certificate"), 0o600)
	if _, err := LoadRootCAs(junk); err == nil {
		t.Error("file without PEM certificates: want error")
	}
}

func TestTLS12OnlyServerGetsClearError(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	ts.TLS = &tls.Config{MaxVersion: tls.VersionTLS12}
	ts.StartTLS()
	t.Cleanup(ts.Close)
	_, err := clientFor(ts.URL, "").GetNetworks(context.Background())
	if err == nil || !strings.Contains(err.Error(), "TLS 1.3") {
		t.Fatalf("error = %v, want one that names TLS 1.3", err)
	}
}
