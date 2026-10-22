package forwardapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PresentedChain connects to the server at baseURL and returns the
// certificates it presents, and whether they verify against roots (the system
// store when roots is nil). Verification stays on: an untrusted chain is read
// from the verification error, never by turning verification off.
func PresentedChain(ctx context.Context, baseURL string, roots *x509.CertPool) ([]*x509.Certificate, bool, error) {
	host, addr, err := serverAddress(baseURL)
	if err != nil {
		return nil, false, err
	}
	dialer := &tls.Dialer{Config: &tls.Config{
		MinVersion: tls.VersionTLS13, // the same floor the API client enforces
		ServerName: host,
		RootCAs:    roots,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err == nil {
		defer conn.Close()
		return conn.(*tls.Conn).ConnectionState().PeerCertificates, true, nil
	}
	var verr *tls.CertificateVerificationError
	if errors.As(err, &verr) && len(verr.UnverifiedCertificates) > 0 {
		return verr.UnverifiedCertificates, false, nil
	}
	if msg := describeTLSError(err, baseURL); msg != "" {
		return nil, false, errors.New(msg)
	}
	return nil, false, fmt.Errorf("cannot connect to %s: %w", addr, err)
}

// TrustAnchor picks the certificate to trust from a chain the server
// presented: the top of the issuer path from the server certificate, which is
// the issuing CA when the server sends one, so trust survives renewal of the
// server certificate. For a self-signed server it is the server certificate
// itself. The path is followed by signature, not by position, because servers
// send certificates out of order and add unrelated ones. It fails when the
// chain would still not verify for host with that certificate trusted, because
// trusting it would not help.
func TrustAnchor(chain []*x509.Certificate, host string) (*x509.Certificate, error) {
	if len(chain) == 0 {
		return nil, errors.New("the server presented no certificate")
	}
	anchor := chain[0]
	onPath := map[*x509.Certificate]bool{anchor: true}
	for {
		var issuer *x509.Certificate
		for _, c := range chain[1:] {
			if !onPath[c] && anchor.CheckSignatureFrom(c) == nil {
				issuer = c
				break
			}
		}
		if issuer == nil {
			break
		}
		onPath[issuer] = true
		anchor = issuer
	}
	roots := x509.NewCertPool()
	roots.AddCert(anchor)
	intermediates := x509.NewCertPool()
	for _, c := range chain[1:] {
		intermediates.AddCert(c)
	}
	_, err := chain[0].Verify(x509.VerifyOptions{
		DNSName:       host,
		Roots:         roots,
		Intermediates: intermediates,
	})
	var hostname x509.HostnameError
	switch {
	case err == nil:
		return anchor, nil
	case errors.As(err, &hostname):
		return nil, fmt.Errorf("the certificate does not match the host name %s (%v). Trusting it would not help: "+
			"use the host name the certificate was issued for in FORWARD_API_BASE_URL, or reissue the certificate with %s "+
			"in its Subject Alternative Names", host, hostname, host)
	default:
		return nil, fmt.Errorf("the certificate chain does not verify even with %q trusted (%v). "+
			"Check the certificate's validity dates, or ask for the CA certificate that issued it", anchor.Subject.CommonName, err)
	}
}

// Fingerprint returns a certificate's SHA-256 fingerprint as colon-separated
// upper-case hex, the form browsers and openssl print.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// SameFingerprint reports whether two fingerprints are equal, ignoring case
// and the separators people copy along with them.
func SameFingerprint(a, b string) bool {
	norm := func(s string) string {
		return strings.ToUpper(strings.NewReplacer(":", "", " ", "", "-", "").Replace(strings.TrimSpace(s)))
	}
	return a != "" && b != "" && norm(a) == norm(b)
}

// SaveTrustedCert adds cert to the PEM file at path, keeping the certificates
// already there so several servers can be trusted at once. It reports false
// when the file already holds the certificate. The file is replaced in one
// rename, so a reader never sees it half written.
func SaveTrustedCert(path string, cert *x509.Certificate) (bool, error) {
	existing, err := os.ReadFile(path) // #nosec G304 -- path is the operator's trust --out flag
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("cannot read %s: %w", path, err)
	}
	for rest := existing; ; {
		var block *pem.Block
		if block, rest = pem.Decode(rest); block == nil {
			break
		}
		if block.Type == "CERTIFICATE" && bytes.Equal(block.Bytes, cert.Raw) {
			return false, nil
		}
	}

	data := append([]byte{}, existing...)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})...)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("cannot create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".ca-*.pem")
	if err != nil {
		return false, fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("cannot write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	// A certificate is public, so the file is readable like any CA bundle.
	if err := os.Chmod(tmp.Name(), 0644); err != nil { // #nosec G302 -- certificates are public, readable like any CA bundle
		return false, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return false, fmt.Errorf("cannot save %s: %w", path, err)
	}
	return true, nil
}

// serverAddress returns the host name and host:port to dial for baseURL.
func serverAddress(baseURL string) (string, string, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return "", "", fmt.Errorf("FORWARD_API_BASE_URL %q is not a URL such as https://forward.example.com", baseURL)
	}
	if u.Scheme != "https" {
		return "", "", fmt.Errorf("FORWARD_API_BASE_URL %q does not use https, so there is no certificate to trust", baseURL)
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "443"
	}
	return host, net.JoinHostPort(host, port), nil
}
