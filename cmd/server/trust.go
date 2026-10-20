package main

import (
	"bufio"
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/forward-mcp/internal/adapters/secondary/envconfig"
	"github.com/forward-mcp/internal/adapters/secondary/forwardapi"
	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
)

const trustUsage = `Usage: forward-mcp-server trust [--fingerprint SHA256] [--out FILE]

Fetches the TLS certificate of FORWARD_API_BASE_URL, shows it, and after you
confirm saves it so the server trusts it. Use it for a Forward server with a
self-signed or internal-CA certificate. Certificate checks stay on.

`

// runTrust implements "forward-mcp-server trust". It returns the exit code.
func runTrust(args []string, in io.Reader, out io.Writer) int {
	fs := flag.NewFlagSet("trust", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		fmt.Fprint(out, trustUsage)
		fs.PrintDefaults()
	}
	fingerprint := fs.String("fingerprint", "", "accept the certificate without a prompt if its SHA-256 fingerprint matches this one")
	outPath := fs.String("out", envconfig.DefaultCACertPath(), "PEM file to add the certificate to")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...interface{}) int {
		fmt.Fprintf(out, "Error: "+format+"\n", a...)
		return 1
	}
	if *outPath == "" {
		return fail("cannot find your home directory. Give the file to save to with --out")
	}

	cfg, err := envconfig.Load(stderrlog.New())
	if err != nil {
		return fail("%v", err)
	}
	baseURL := cfg.Forward.APIBaseURL
	if baseURL == "" {
		return fail("FORWARD_API_BASE_URL is not set. Set it to your Forward server, such as https://forward.example.com")
	}
	var roots *x509.CertPool // nil: the system store
	if cfg.Forward.CACertPath != "" {
		if roots, err = forwardapi.LoadRootCAs(cfg.Forward.CACertPath); err != nil {
			return fail("%v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	chain, trusted, err := forwardapi.PresentedChain(ctx, baseURL, roots)
	if err != nil {
		return fail("%v", err)
	}
	if trusted {
		fmt.Fprintf(out, "The certificate of %s is already trusted. Nothing to do.\n", baseURL)
		return 0
	}

	u, _ := url.Parse(baseURL) // PresentedChain has already parsed it
	anchor, err := forwardapi.TrustAnchor(chain, u.Hostname())
	if err != nil {
		return fail("%v", err)
	}

	fmt.Fprintf(out, "Server:      %s\n", u.Host)
	fmt.Fprintf(out, "Certificate: %s\n", certName(chain[0].Subject))
	for _, c := range chain[1:] {
		fmt.Fprintf(out, "Also sent:   %s\n", certName(c.Subject))
	}
	fmt.Fprintf(out, "\nTo trust:    %s\n", certName(anchor.Subject))
	fmt.Fprintf(out, "Issuer:      %s\n", certName(anchor.Issuer))
	fmt.Fprintf(out, "Valid until: %s\n", anchor.NotAfter.Format("2006-01-02"))
	got := forwardapi.Fingerprint(anchor)
	fmt.Fprintf(out, "SHA-256:     %s\n\n", got)

	if *fingerprint != "" {
		if !forwardapi.SameFingerprint(*fingerprint, got) {
			return fail("the fingerprint does not match --fingerprint. Nothing was saved. If the server's certificate changed, check the new one with your administrator before trusting it")
		}
	} else {
		fmt.Fprint(out, "Check the fingerprint with your Forward administrator. Trust this certificate? [y/N] ")
		answer, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			fmt.Fprintln(out, "Not saved.")
			return 1
		}
	}

	added, err := forwardapi.SaveTrustedCert(*outPath, anchor)
	if err != nil {
		return fail("%v", err)
	}
	if added {
		fmt.Fprintf(out, "Saved to %s.\n", *outPath)
	} else {
		fmt.Fprintf(out, "%s already holds this certificate.\n", *outPath)
	}

	// Name what the server will read, and prove it now connects.
	switch configured := os.Getenv("FORWARD_CA_CERT_PATH"); {
	case configured != "" && configured != *outPath:
		fmt.Fprintf(out, "FORWARD_CA_CERT_PATH is set to %s, which the server reads instead. Unset it, or set it to %s.\n", configured, *outPath)
	case configured == "" && *outPath != envconfig.DefaultCACertPath():
		fmt.Fprintf(out, "Set FORWARD_CA_CERT_PATH=%s so the server reads it.\n", *outPath)
	default:
		fmt.Fprintln(out, "The server reads this file automatically.")
	}
	saved, err := forwardapi.LoadRootCAs(*outPath)
	if err != nil {
		return fail("%v", err)
	}
	if _, ok, err := forwardapi.PresentedChain(ctx, baseURL, saved); err != nil || !ok {
		return fail("the connection is still not trusted with %s (%v)", *outPath, err)
	}
	fmt.Fprintf(out, "Verified: %s is now trusted.\n", baseURL)
	return 0
}

// certName is a name's common name and organization, the parts people read;
// the full distinguished name prints attributes such as e-mail as hex.
func certName(n pkix.Name) string {
	if len(n.Organization) == 0 {
		return n.CommonName
	}
	return fmt.Sprintf("%s (%s)", n.CommonName, strings.Join(n.Organization, ", "))
}
