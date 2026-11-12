package config

import (
	"bufio"
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
}

const (
	insecureKey = "insecure"
	caCertKey   = "ca_cert"
)

// ReadTLSOptions reads the TLS settings saved in a profile. A missing profile
// or missing settings return the zero value (verify with system CAs).
func ReadTLSOptions(profile string) (TLSOptions, error) {
	var opts TLSOptions
	path, err := GetProfilePath(profile)
	if err != nil {
		return opts, err
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return opts, nil
	}
	if err != nil {
		return opts, fmt.Errorf("failed to open profile file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case insecureKey:
			opts.Insecure, _ = strconv.ParseBool(strings.TrimSpace(value))
		case caCertKey:
			opts.CACert = strings.TrimSpace(value)
		}
	}
	return opts, scanner.Err()
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
		case key == insecureKey || key == caCertKey:
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
}
