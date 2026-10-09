package httpserver

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forward-mcp/internal/ports"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type idp struct {
	key  *rsa.PrivateKey
	jwk  jwk.Key
	url  string
	cert []byte // the JWKS server's self-signed certificate, PEM
}

// newIdP serves a JWKS over TLS and returns a signer for its key.
func newIdP(t *testing.T) *idp {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signKey, err := jwk.Import(priv)
	if err != nil {
		t.Fatal(err)
	}
	_ = signKey.Set(jwk.KeyIDKey, "k1")
	_ = signKey.Set(jwk.AlgorithmKey, jwa.RS256())
	pub, err := signKey.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	set := jwk.NewSet()
	_ = set.AddKey(pub)

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(ts.Close)
	jwksHTTPClient = ts.Client()
	t.Cleanup(func() { jwksHTTPClient = nil })
	return &idp{key: priv, jwk: signKey, url: ts.URL + "/.well-known/jwks.json", cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})}
}

type claims struct {
	iss, aud, sub string
	exp           time.Time
}

func (p *idp) sign(t *testing.T, c claims, key jwk.Key) string {
	t.Helper()
	b := jwt.NewBuilder().Issuer(c.iss).Audience([]string{c.aud}).Expiration(c.exp).IssuedAt(time.Now())
	if c.sub != "" {
		b = b.Subject(c.sub)
	}
	tok, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key))
	if err != nil {
		t.Fatal(err)
	}
	return string(signed)
}

func jwtConfig(p *idp) *ports.HTTPConfig {
	return &ports.HTTPConfig{
		Enabled: true, AuthMode: "jwt",
		JWTIssuer: "https://idp.example", JWTAudience: "forward-mcp",
		JWTPublicKeyURL: p.url,
	}
}

func status(t *testing.T, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestJWTRejectsBadTokens(t *testing.T) {
	p := newIdP(t)
	ts := newTestServer(t, jwtConfig(p), &captureLog{})

	otherPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	otherKey, _ := jwk.Import(otherPriv)
	_ = otherKey.Set(jwk.KeyIDKey, "k1") // same kid, wrong key

	good := claims{iss: "https://idp.example", aud: "forward-mcp", sub: "alice", exp: time.Now().Add(time.Hour)}
	cases := map[string]string{
		"wrong issuer":   p.sign(t, claims{"https://evil.example", good.aud, good.sub, good.exp}, p.jwk),
		"wrong audience": p.sign(t, claims{good.iss, "someone-else", good.sub, good.exp}, p.jwk),
		"expired":        p.sign(t, claims{good.iss, good.aud, good.sub, time.Now().Add(-time.Minute)}, p.jwk),
		"no subject":     p.sign(t, claims{good.iss, good.aud, "", good.exp}, p.jwk),
		"wrong key":      p.sign(t, good, otherKey),
		"not a jwt":      "not-a-jwt",
	}
	for name, token := range cases {
		if got := status(t, ts.URL+MCPPath, token); got != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, got)
		}
	}
}

func TestJWTValidTokenCallsTool(t *testing.T) {
	p := newIdP(t)
	log := &captureLog{}
	ts := newTestServer(t, jwtConfig(p), log)
	token := p.sign(t, claims{iss: "https://idp.example", aud: "forward-mcp", sub: "alice", exp: time.Now().Add(time.Hour)}, p.jwk)

	callPing(t, &mcp.StreamableClientTransport{
		Endpoint:   ts.URL + MCPPath,
		HTTPClient: &http.Client{Transport: bearer{token}},
	})
	if !log.contains("user=alice") {
		t.Error("request log does not name the JWT subject")
	}
}

// The IdP has a self-signed certificate; trusting it via JWKSCACertPath
// (FORWARD_HTTP_JWKS_CA_CERT) is enough, without the test client override.
func TestJWTWithSelfSignedIdPCertificate(t *testing.T) {
	p := newIdP(t)
	jwksHTTPClient = nil // use the configured CA, as in production
	idpCert := writePEM(t, p.cert)

	cfg := jwtConfig(p)
	cfg.JWKSCACertPath = idpCert
	ts := newTestServer(t, cfg, &captureLog{})
	token := p.sign(t, claims{iss: "https://idp.example", aud: "forward-mcp", sub: "alice", exp: time.Now().Add(time.Hour)}, p.jwk)

	callPing(t, &mcp.StreamableClientTransport{
		Endpoint:   ts.URL + MCPPath,
		HTTPClient: &http.Client{Transport: bearer{token}},
	})

	cfg2 := jwtConfig(p) // same IdP, CA not configured: token cannot be checked
	ts2 := newTestServer(t, cfg2, &captureLog{})
	if got := status(t, ts2.URL+MCPPath, token); got != http.StatusUnauthorized {
		t.Fatalf("untrusted IdP certificate: status %d, want 401", got)
	}
}

// writePEM writes a PEM certificate to a temporary file and returns its path.
func writePEM(t *testing.T, pemBytes []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "idp.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
