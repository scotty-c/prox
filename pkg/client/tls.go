package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/scotty-c/prox/pkg/config"
)

// TLSOptions controls certificate verification for the Proxmox API
type TLSOptions = config.TLSOptions

// Overrides set from command-line flags (highest precedence)
var (
	insecureFlag    bool
	insecureFlagSet bool
	caCertFlag      string
)

// SetTLSFlagOverrides records --insecure and --ca-cert from the command line.
// insecureSet reports whether --insecure was given at all, so that
// --insecure=false can override a profile or environment setting.
func SetTLSFlagOverrides(insecure, insecureSet bool, caCert string) {
	insecureFlag = insecure
	insecureFlagSet = insecureSet
	caCertFlag = caCert
}

// ResolveTLSOptions combines TLS settings from, in increasing precedence:
// the current profile, PROX_INSECURE / PROX_CA_CERT, and command-line flags.
// Certificates are verified unless one of these says otherwise.
func ResolveTLSOptions() (TLSOptions, error) {
	opts, err := config.ReadTLSOptions(config.GetCurrentProfile())
	if err != nil {
		return opts, err
	}

	if v := strings.TrimSpace(os.Getenv("PROX_INSECURE")); v != "" {
		insecure, err := strconv.ParseBool(v)
		if err != nil {
			return opts, fmt.Errorf("invalid PROX_INSECURE value %q: use true or false", v)
		}
		opts.Insecure = insecure
	}
	if v := strings.TrimSpace(os.Getenv("PROX_CA_CERT")); v != "" {
		opts.CACert = v
	}

	if insecureFlagSet {
		opts.Insecure = insecureFlag
	}
	if caCertFlag != "" {
		opts.CACert = caCertFlag
	}
	return opts, nil
}

// buildTLSConfig returns the TLS configuration for opts
func buildTLSConfig(opts TLSOptions) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if opts.Insecure {
		cfg.InsecureSkipVerify = true
		return cfg, nil
	}
	if opts.CACert != "" {
		pem, err := os.ReadFile(opts.CACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate %s: %w", opts.CACert, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no PEM certificates found in %s", opts.CACert)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// ErrCertificate marks errors caused by certificate verification failing
var ErrCertificate = errors.New("TLS certificate verification failed")

// wrapTLSError adds instructions to certificate verification errors
func wrapTLSError(err error, baseURL string) error {
	var verifyErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidErr x509.CertificateInvalidError
	if !errors.As(err, &verifyErr) && !errors.As(err, &unknownAuthority) &&
		!errors.As(err, &hostnameErr) && !errors.As(err, &invalidErr) {
		return err
	}
	return fmt.Errorf(`%w for %s: %v

Proxmox uses a self-signed certificate unless you have configured one
(for example with ACME). To connect, either:
  - trust the Proxmox CA: copy /etc/pve/pve-root-ca.pem from a node and pass
    --ca-cert /path/to/pve-root-ca.pem (or set PROX_CA_CERT, or save it with
    'prox config update --ca-cert /path/to/pve-root-ca.pem')
  - or skip verification: pass --insecure (or set PROX_INSECURE=true, or save
    it with 'prox config update --insecure')`, ErrCertificate, baseURL, err)
}
