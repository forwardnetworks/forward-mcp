package envconfig

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/forward-mcp/internal/ports"
)

// validate refuses settings that are unsafe or cannot work, so the server
// fails at startup with a clear message instead of misbehaving later.
func validate(cfg *Config, log ports.Logger) error {
	if err := checkCAFile("FORWARD_CA_CERT_PATH", cfg.Forward.CACertPath); err != nil {
		return err
	}
	if !cfg.HTTP.Enabled {
		return nil
	}
	h := &cfg.HTTP

	if (h.TLSCertFile == "") != (h.TLSKeyFile == "") {
		return fmt.Errorf("set both FORWARD_HTTP_TLS_CERT and FORWARD_HTTP_TLS_KEY, or neither")
	}
	if h.TLSCertFile == "" {
		if !h.AllowInsecure {
			return fmt.Errorf("SECURITY ERROR: the HTTP server needs TLS. Set FORWARD_HTTP_TLS_CERT and FORWARD_HTTP_TLS_KEY " +
				"(a self-signed certificate works), or set FORWARD_HTTP_ALLOW_INSECURE=true behind a TLS-terminating proxy or for local development")
		}
		log.Warn("SECURITY WARNING: HTTP server will run without TLS (FORWARD_HTTP_ALLOW_INSECURE=true)")
	}

	if err := validateCORSOrigins(h.CORSOrigins); err != nil {
		return fmt.Errorf("SECURITY ERROR: invalid FORWARD_HTTP_CORS_ORIGINS: %w", err)
	}

	if h.AuthMode == "" {
		h.AuthMode = "api-key"
	}
	switch h.AuthMode {
	case "api-key":
		if len(h.APIKeys) == 0 {
			return fmt.Errorf("FORWARD_HTTP_AUTH_MODE=api-key needs FORWARD_HTTP_API_KEYS (format key1:user1,key2:user2)")
		}
	case "jwt":
		u, err := url.Parse(h.JWTPublicKeyURL)
		if h.JWTPublicKeyURL == "" || err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("FORWARD_HTTP_AUTH_MODE=jwt needs FORWARD_HTTP_JWT_PUBLIC_KEY_URL set to an https:// JWKS URL")
		}
		if err := checkCAFile("FORWARD_HTTP_JWKS_CA_CERT", h.JWKSCACertPath); err != nil {
			return err
		}
		if h.JWTIssuer == "" {
			log.Warn("FORWARD_HTTP_JWT_ISSUER is not set: tokens from any issuer signed by the JWKS keys are accepted")
		}
	case "none":
		log.Warn("SECURITY WARNING: FORWARD_HTTP_AUTH_MODE=none - anyone who can reach the server can use it")
	default:
		return fmt.Errorf("FORWARD_HTTP_AUTH_MODE must be api-key, jwt or none, not %q", h.AuthMode)
	}
	return nil
}

// checkCAFile verifies that a configured CA file exists and holds at least one
// PEM certificate.
func checkCAFile(setting, path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: cannot read %s: %w", setting, path, err)
	}
	for block, rest := pem.Decode(data); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%s: %s contains no PEM certificate (expected -----BEGIN CERTIFICATE-----)", setting, path)
}

// validateCORSOrigins rejects "*" (credentials are allowed, so a wildcard
// would let any site use a signed-in browser) and anything but exact
// https:// origins, with http://localhost and http://127.0.0.1 allowed for
// development.
func validateCORSOrigins(origins []string) error {
	for _, origin := range origins {
		if origin == "*" {
			return fmt.Errorf("the wildcard origin * is not allowed; list exact origins")
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
			return fmt.Errorf("origin %q must be scheme://host[:port] with no path", origin)
		}
		local := strings.HasPrefix(u.Host, "localhost") || strings.HasPrefix(u.Host, "127.0.0.1")
		if u.Scheme != "https" && !(u.Scheme == "http" && local) {
			return fmt.Errorf("origin %q must use https:// (http:// only for localhost)", origin)
		}
	}
	return nil
}
