package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// TLSOptions controls how the Proxmox server's certificate is checked
type TLSOptions struct {
	// Insecure skips certificate verification (for self-signed certificates)
	Insecure bool
	// CACert is the path to a PEM file with extra CA certificates to trust,
	// e.g. a copy of /etc/pve/pve-root-ca.pem from a Proxmox node
	CACert string
	// Fingerprint pins the server's certificate: the connection is accepted
	// only if the certificate it presents has this SHA-256 fingerprint, in the
	// AA:BB:... form the Proxmox web interface shows. It replaces chain and
	// hostname checking, as the fingerprint of a self-signed certificate does
	// in ssh's known_hosts. Insecure still overrides it.
	Fingerprint string
}

const (
	insecureKey    = "insecure"
	caCertKey      = "ca_cert"
	fingerprintKey = "fingerprint"
)

// NormalizeFingerprint checks that s is a SHA-256 fingerprint, written with or
// without colons in either case, and returns it as uppercase pairs separated
// by colons, the form the Proxmox web interface shows.
func NormalizeFingerprint(s string) (string, error) {
	hex := strings.ToUpper(strings.NewReplacer(":", "", " ", "", "-", "").Replace(strings.TrimSpace(s)))
	if len(hex) != 64 {
		return "", fmt.Errorf("%q is not a SHA-256 fingerprint: expected 64 hexadecimal digits (32 bytes), got %d", s, len(hex))
	}
	for _, r := range hex {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') {
			return "", fmt.Errorf("%q is not a SHA-256 fingerprint: %q is not a hexadecimal digit", s, r)
		}
	}
	pairs := make([]string, 0, 32)
	for i := 0; i < 64; i += 2 {
		pairs = append(pairs, hex[i:i+2])
	}
	return strings.Join(pairs, ":"), nil
}

// ReadTLSOptions reads the TLS settings saved in a profile. A missing profile
// or missing settings return the zero value (verify with system CAs).
func ReadTLSOptions(profile string) (TLSOptions, error) {
	var opts TLSOptions
	path, err := GetProfilePath(profile)
	if err != nil {
		return opts, err
	}
	values, _, err := readProfileFile(path)
	if os.IsNotExist(err) {
		return opts, nil
	}
	if err != nil {
		return opts, fmt.Errorf("failed to read profile file: %w", err)
	}
	opts.Insecure, _ = strconv.ParseBool(values[insecureKey])
	opts.CACert = values[caCertKey]
	opts.Fingerprint = values[fingerprintKey]
	return opts, nil
}

// SetTLSOptions saves TLS settings in an existing profile, keeping everything else
func SetTLSOptions(profile string, opts TLSOptions) error {
	path, err := GetProfilePath(profile)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read profile '%s': %w", profile, err)
	}

	var content strings.Builder
	var comments []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		key, _, _ := strings.Cut(strings.TrimSpace(line), "=")
		switch {
		case key == insecureKey || key == caCertKey || key == fingerprintKey:
			continue
		case strings.HasPrefix(strings.TrimSpace(line), "#"):
			comments = append(comments, line)
		default:
			content.WriteString(line + "\n")
		}
	}
	writeTLSOptions(&content, opts)
	for _, c := range comments {
		content.WriteString(c + "\n")
	}
	return writeProfileFile(path, content.String())
}

// writeTLSOptions appends non-default TLS settings in profile file format
func writeTLSOptions(b *strings.Builder, opts TLSOptions) {
	if opts.Insecure {
		fmt.Fprintf(b, "%s=true\n", insecureKey)
	}
	if opts.CACert != "" {
		fmt.Fprintf(b, "%s=%s\n", caCertKey, opts.CACert)
	}
	if opts.Fingerprint != "" {
		fmt.Fprintf(b, "%s=%s\n", fingerprintKey, opts.Fingerprint)
	}
}
