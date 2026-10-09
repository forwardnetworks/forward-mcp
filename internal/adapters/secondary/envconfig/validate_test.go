package envconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type nopLog struct{}

func (nopLog) Debug(string, ...interface{})                          {}
func (nopLog) Info(string, ...interface{})                           {}
func (nopLog) Warn(string, ...interface{})                           {}
func (nopLog) Error(string, ...interface{})                          {}
func (nopLog) LogToolCall(string, interface{}, time.Duration, error) {}

// selfSignedPEM writes a self-signed certificate to a file and returns its path.
func selfSignedPEM(t *testing.T) string {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "forward.lab"},
		DNSNames: []string{"forward.lab"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "cert.pem")
	_ = os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	return p
}

// env clears every setting Load reads, then applies the given ones.
func env(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{
		"FORWARD_CA_CERT_PATH", "FORWARD_HTTP_ENABLED", "FORWARD_HTTP_TLS_CERT", "FORWARD_HTTP_TLS_KEY",
		"FORWARD_HTTP_ALLOW_INSECURE", "FORWARD_HTTP_AUTH_MODE", "FORWARD_HTTP_API_KEYS",
		"FORWARD_HTTP_CORS_ORIGINS", "FORWARD_HTTP_JWT_PUBLIC_KEY_URL", "FORWARD_HTTP_JWKS_CA_CERT",
		"FORWARD_HTTP_JWT_ISSUER", "FORWARD_INSECURE_SKIP_VERIFY",
	} {
		t.Setenv(k, "") // restores the old value after the test
		os.Unsetenv(k)
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLoadHTTPSettings(t *testing.T) {
	cert := selfSignedPEM(t)
	junk := filepath.Join(t.TempDir(), "junk.pem")
	_ = os.WriteFile(junk, []byte("hello"), 0o600)
	dev := map[string]string{"FORWARD_HTTP_ENABLED": "true", "FORWARD_HTTP_ALLOW_INSECURE": "true", "FORWARD_HTTP_API_KEYS": "k:alice"}
	with := func(extra map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range dev {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	cases := []struct {
		name    string
		env     map[string]string
		wantErr string // "" = must load
	}{
		{"stdio only", map[string]string{}, ""},
		{"dev mode loads ALLOW_INSECURE", dev, ""},
		{"no TLS and no opt-in", map[string]string{"FORWARD_HTTP_ENABLED": "true", "FORWARD_HTTP_API_KEYS": "k:a"}, "needs TLS"},
		{"cert without key", with(map[string]string{"FORWARD_HTTP_TLS_CERT": cert}), "set both"},
		{"CORS wildcard", with(map[string]string{"FORWARD_HTTP_CORS_ORIGINS": "*"}), "wildcard"},
		{"CORS plain http", with(map[string]string{"FORWARD_HTTP_CORS_ORIGINS": "http://app.example.com"}), "https"},
		{"CORS localhost", with(map[string]string{"FORWARD_HTTP_CORS_ORIGINS": "http://localhost:3000,https://app.example.com"}), ""},
		{"api-key without keys", with(map[string]string{"FORWARD_HTTP_API_KEYS": ""}), "FORWARD_HTTP_API_KEYS"},
		{"jwt needs https JWKS", with(map[string]string{"FORWARD_HTTP_AUTH_MODE": "jwt", "FORWARD_HTTP_JWT_PUBLIC_KEY_URL": "http://idp/jwks"}), "https://"},
		{"jwt with self-signed IdP CA", with(map[string]string{"FORWARD_HTTP_AUTH_MODE": "jwt", "FORWARD_HTTP_JWT_PUBLIC_KEY_URL": "https://idp.lab/jwks", "FORWARD_HTTP_JWKS_CA_CERT": cert}), ""},
		{"jwt CA file is junk", with(map[string]string{"FORWARD_HTTP_AUTH_MODE": "jwt", "FORWARD_HTTP_JWT_PUBLIC_KEY_URL": "https://idp.lab/jwks", "FORWARD_HTTP_JWKS_CA_CERT": junk}), "no PEM certificate"},
		{"unknown auth mode", with(map[string]string{"FORWARD_HTTP_AUTH_MODE": "basic"}), "must be api-key"},
		{"Forward CA self-signed cert", map[string]string{"FORWARD_CA_CERT_PATH": cert}, ""},
		{"Forward CA missing file", map[string]string{"FORWARD_CA_CERT_PATH": "/nonexistent/ca.pem"}, "cannot read"},
		{"Forward CA junk file", map[string]string{"FORWARD_CA_CERT_PATH": junk}, "no PEM certificate"},
		{"skip-verify refused", map[string]string{"FORWARD_INSECURE_SKIP_VERIFY": "true"}, "no longer supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env(t, tc.env)
			cfg, err := Load(nopLog{})
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}
			if tc.name == "dev mode loads ALLOW_INSECURE" && !cfg.HTTP.AllowInsecure {
				t.Fatal("FORWARD_HTTP_ALLOW_INSECURE=true was not loaded")
			}
		})
	}
}
