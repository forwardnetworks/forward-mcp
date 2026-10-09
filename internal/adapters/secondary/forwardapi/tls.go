package forwardapi

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadRootCAs returns the system trust store plus every certificate in the PEM
// file at path. A self-signed server certificate can be given directly: it is
// its own CA. It fails if the file is missing or holds no certificate, so a
// typo in the path is reported instead of silently ignored.
func LoadRootCAs(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read CA certificate file %s: %w", path, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("CA certificate file %s contains no PEM certificates", path)
	}
	return pool, nil
}

// describeTLSError turns a certificate verification failure into a message
// that says what to do. It returns "" for errors that are not about TLS trust.
func describeTLSError(err error, baseURL string) string {
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verify *tls.CertificateVerificationError
	switch {
	case strings.Contains(err.Error(), "protocol version"):
		return fmt.Sprintf("%s does not support TLS 1.3, which this server requires for API connections. "+
			"Enable TLS 1.3 on the Forward server or the proxy in front of it", baseURL)
	case errors.As(err, &hostname):
		return fmt.Sprintf("the TLS certificate of %s does not match its host name (%v). "+
			"Use the host name the certificate was issued for in FORWARD_API_BASE_URL, or reissue the certificate "+
			"with that name in its Subject Alternative Names; certificates that set only the Common Name are rejected", baseURL, hostname)
	case errors.As(err, &invalid):
		return fmt.Sprintf("the TLS certificate of %s is not valid (%v). Check its validity dates and that it has a Subject Alternative Name", baseURL, invalid)
	case errors.As(err, &unknown), errors.As(err, &verify):
		return fmt.Sprintf("the TLS certificate of %s is not trusted, which is expected for a self-signed or internal-CA certificate. "+
			"Save the server certificate (or the CA that issued it) as a PEM file and set FORWARD_CA_CERT_PATH to that file", baseURL)
	}
	return ""
}
