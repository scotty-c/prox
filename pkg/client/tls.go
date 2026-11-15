package client

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scotty-c/prox/pkg/apierr"
	"github.com/scotty-c/prox/pkg/config"
)

// TLSOptions controls certificate verification for the Proxmox API
type TLSOptions = config.TLSOptions

// Overrides set from command-line flags (highest precedence)
var (
	insecureFlag    bool
	insecureFlagSet bool
	caCertFlag      string
	fingerprintFlag string
)

// SetTLSFlagOverrides records --insecure and --ca-cert from the command line.
// insecureSet reports whether --insecure was given at all, so that
// --insecure=false can override a profile or environment setting.
func SetTLSFlagOverrides(insecure, insecureSet bool, caCert, fingerprint string) {
	insecureFlag = insecure
	insecureFlagSet = insecureSet
	caCertFlag = caCert
	fingerprintFlag = fingerprint
}

// ResolveTLSOptions combines TLS settings from, in increasing precedence:
// the current profile, PROX_INSECURE / PROX_CA_CERT / PROX_FINGERPRINT, and
// command-line flags. Certificates are verified unless one of these says
// otherwise. A CA and a fingerprint are two ways to say who to trust, so
// whichever a layer sets replaces the other one from the layers below it.
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
		opts.CACert, opts.Fingerprint = v, ""
	}
	if v := strings.TrimSpace(os.Getenv("PROX_FINGERPRINT")); v != "" {
		fp, err := config.NormalizeFingerprint(v)
		if err != nil {
			return opts, fmt.Errorf("invalid PROX_FINGERPRINT: %w", err)
		}
		opts.Fingerprint, opts.CACert = fp, ""
	}

	if insecureFlagSet {
		opts.Insecure = insecureFlag
	}
	if caCertFlag != "" {
		opts.CACert, opts.Fingerprint = caCertFlag, ""
	}
	if fingerprintFlag != "" {
		fp, err := config.NormalizeFingerprint(fingerprintFlag)
		if err != nil {
			return opts, fmt.Errorf("invalid --fingerprint: %w", err)
		}
		opts.Fingerprint, opts.CACert = fp, ""
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
	if opts.Fingerprint != "" {
		// A pinned certificate is trusted for what it is, not for who signed
		// it or what name it carries (the Proxmox default certificate is
		// self-signed and often reached by IP address). Chain checking is off
		// and this check takes its place.
		want := opts.Fingerprint
		cfg.InsecureSkipVerify = true //nolint:gosec // replaced by the fingerprint check below
		cfg.VerifyPeerCertificate = func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return &FingerprintMismatchError{Want: want}
			}
			if got := fingerprintOf(raw[0]); got != want {
				return &FingerprintMismatchError{Want: want, Got: got}
			}
			return nil
		}
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

// FingerprintMismatchError means the server presented a certificate other than
// the one the profile pins
type FingerprintMismatchError struct {
	Want, Got string
}

func (e *FingerprintMismatchError) Error() string {
	if e.Got == "" {
		return "the server presented no certificate"
	}
	return fmt.Sprintf("the server's certificate fingerprint %s does not match the pinned %s", e.Got, e.Want)
}

// fingerprintOf is the SHA-256 fingerprint of a DER certificate in the form
// the Proxmox web interface shows
func fingerprintOf(der []byte) string {
	sum := sha256.Sum256(der)
	pairs := make([]string, len(sum))
	for i, b := range sum {
		pairs[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(pairs, ":")
}

// ServerCertificate describes the certificate a server presented
type ServerCertificate struct {
	Fingerprint string
	Subject     string
	Issuer      string
	DNSNames    []string
	NotBefore   time.Time
	NotAfter    time.Time
	SelfSigned  bool
}

// ProbeCertificate connects to the server at baseURL and returns the
// certificate it presents, without trusting it and without sending anything
// but the TLS handshake. The result is for showing to a person who will decide
// whether to trust it; nothing is verified here.
func ProbeCertificate(ctx context.Context, baseURL string) (*ServerCertificate, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid URL %q", baseURL)
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	dialer := tls.Dialer{Config: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         u.Hostname(),
		InsecureSkipVerify: true, //nolint:gosec // the point is to look at an untrusted certificate
	}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("the server presented no certificate")
	}
	leaf := certs[0]
	return &ServerCertificate{
		Fingerprint: fingerprintOf(leaf.Raw),
		Subject:     leaf.Subject.String(),
		Issuer:      leaf.Issuer.String(),
		DNSNames:    leaf.DNSNames,
		NotBefore:   leaf.NotBefore,
		NotAfter:    leaf.NotAfter,
		SelfSigned:  leaf.Subject.String() == leaf.Issuer.String(),
	}, nil
}

// ErrCertificate marks errors caused by certificate verification failing
var ErrCertificate = apierr.ErrCertificate

// wrapTLSError adds instructions to certificate verification errors
func wrapTLSError(err error, baseURL string) error {
	var mismatch *FingerprintMismatchError
	if errors.As(err, &mismatch) {
		return fmt.Errorf(`%w for %s: %w

The fingerprint saved for this server no longer matches its certificate. Either
the certificate was renewed or replaced, or something is intercepting the
connection. If you expect a new certificate, compare the fingerprint above with
the one in the Proxmox web interface (node > System > Certificates) and save it:
  prox config setup --fingerprint %s`, ErrCertificate, baseURL, mismatch, mismatch.Got)
	}
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
  - run 'prox config setup' on a terminal: it shows the server's fingerprint,
    asks you to compare it with the Proxmox web interface, and saves it
  - or trust the Proxmox CA: copy /etc/pve/pve-root-ca.pem from a node and pass
    --ca-cert /path/to/pve-root-ca.pem (or set PROX_CA_CERT)
  - or skip verification: pass --insecure (or set PROX_INSECURE=true)`, ErrCertificate, baseURL, err)
}
