package forwardapi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPresentedChainReadsUntrustedCertWithVerificationOn(t *testing.T) {
	url, certFile := selfSignedForward(t)

	chain, trusted, err := PresentedChain(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("PresentedChain: %v", err)
	}
	if trusted || len(chain) == 0 {
		t.Fatalf("self-signed server: trusted=%v, %d certificates", trusted, len(chain))
	}

	roots, err := LoadRootCAs(certFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, trusted, err := PresentedChain(context.Background(), url, roots); err != nil || !trusted {
		t.Errorf("with the certificate trusted: trusted=%v err=%v", trusted, err)
	}
}

func TestPresentedChainRejectsPlainHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(ts.Close)
	if _, _, err := PresentedChain(context.Background(), ts.URL, nil); err == nil || !strings.Contains(err.Error(), "https") {
		t.Errorf("http URL: want an https error, got %v", err)
	}
}

func TestTrustAnchorChecksHostName(t *testing.T) {
	url, _ := selfSignedForward(t)
	chain, _, err := PresentedChain(context.Background(), url, nil)
	if err != nil {
		t.Fatal(err)
	}
	// httptest certificates are issued for 127.0.0.1 and example.com.
	anchor, err := TrustAnchor(chain, "127.0.0.1")
	if err != nil {
		t.Fatalf("matching host: %v", err)
	}
	if anchor != chain[0] {
		t.Error("a self-signed server's anchor is not its own certificate")
	}
	if _, err := TrustAnchor(chain, "forward.wrong.test"); err == nil || !strings.Contains(err.Error(), "Subject Alternative Names") {
		t.Errorf("wrong host: want a host name error, got %v", err)
	}
	if _, err := TrustAnchor(nil, "127.0.0.1"); err == nil {
		t.Error("empty chain: want an error")
	}
}

// A real Forward server sent its certificate, the issuing intermediate CA,
// and an unrelated self-signed CA last. The anchor is the issuer, not the
// last certificate.
func TestTrustAnchorFollowsIssuersNotPosition(t *testing.T) {
	ca, caKey := newCert(t, "Forward Networks Intermediate CA", nil, nil, true)
	leaf, _ := newCert(t, "forward.example.test", ca, caKey, false)
	unrelated, _ := newCert(t, "skyforge.local.test", nil, nil, true)

	anchor, err := TrustAnchor([]*x509.Certificate{leaf, ca, unrelated}, "forward.example.test")
	if err != nil {
		t.Fatalf("TrustAnchor: %v", err)
	}
	if anchor != ca {
		t.Errorf("anchor = %q, want the issuing CA", anchor.Subject.CommonName)
	}
	// Order does not matter either.
	if anchor, err = TrustAnchor([]*x509.Certificate{leaf, unrelated, ca}, "forward.example.test"); err != nil || anchor != ca {
		t.Errorf("reordered chain: anchor %v, err %v", anchor.Subject.CommonName, err)
	}
	// Without its issuer the server certificate is the only thing to trust.
	if anchor, err = TrustAnchor([]*x509.Certificate{leaf, unrelated}, "forward.example.test"); err != nil || anchor != leaf {
		t.Errorf("no issuer sent: anchor %v, err %v", anchor.Subject.CommonName, err)
	}
}

func TestSaveTrustedCertAppendsOnceAndKeepsOthers(t *testing.T) {
	urlA, _ := selfSignedForward(t)
	chainA, _, _ := PresentedChain(context.Background(), urlA, nil)
	other := otherCert(t)
	path := filepath.Join(t.TempDir(), "sub", "ca.pem")

	for i, c := range []*x509.Certificate{other, chainA[0]} {
		added, err := SaveTrustedCert(path, c)
		if err != nil || !added {
			t.Fatalf("save %d: added=%v err=%v", i, added, err)
		}
	}
	if added, err := SaveTrustedCert(path, chainA[0]); err != nil || added {
		t.Errorf("saving the same certificate again: added=%v err=%v", added, err)
	}

	// Both certificates are in the file, and it now trusts server A.
	data, _ := os.ReadFile(path)
	if n := strings.Count(string(data), "BEGIN CERTIFICATE"); n != 2 {
		t.Errorf("file holds %d certificates, want 2", n)
	}
	roots, err := LoadRootCAs(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, trusted, err := PresentedChain(context.Background(), urlA, roots); err != nil || !trusted {
		t.Errorf("saved file does not trust the server: trusted=%v err=%v", trusted, err)
	}
	if networks, err := clientFor(urlA, path).GetNetworks(context.Background()); err != nil || len(networks) != 1 {
		t.Errorf("API client with the saved file: %v %v", networks, err)
	}
}

func TestFingerprint(t *testing.T) {
	c := otherCert(t)
	fp := Fingerprint(c)
	if len(fp) != 32*3-1 || strings.ToUpper(fp) != fp {
		t.Errorf("fingerprint %q is not 32 colon-separated upper-case bytes", fp)
	}
	if !SameFingerprint(fp, strings.ToLower(strings.ReplaceAll(fp, ":", ""))) {
		t.Error("the same fingerprint without colons, in lower case, did not match")
	}
	if SameFingerprint(fp, "") || SameFingerprint(fp, Fingerprint(otherCert(t))) {
		t.Error("a different fingerprint matched")
	}
}

// otherCert returns a freshly generated self-signed certificate, different
// from the fixed one every httptest server uses.
func otherCert(t *testing.T) *x509.Certificate {
	c, _ := newCert(t, "other.forward.test", nil, nil, false)
	return c
}

// newCert makes a certificate for name, signed by parent with parentKey, or
// self-signed when parent is nil.
func newCert(t *testing.T, name string, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, isCA bool) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		DNSNames:              []string{name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	}
	if parent == nil {
		parent, parentKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c, key
}
