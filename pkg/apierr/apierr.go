// Package apierr holds the error kinds that prox treats specially. Lower
// layers wrap them (with %w) so the command layer can pick an accurate tip
// with errors.Is instead of searching the error text.
package apierr

import "errors"

var (
	// ErrUnreachable means the Proxmox server could not be contacted
	ErrUnreachable = errors.New("cannot reach the Proxmox server")
	// ErrAuth means Proxmox rejected the credentials or the session
	ErrAuth = errors.New("authentication failed")
	// ErrPermission means the user is not allowed to do what was asked
	ErrPermission = errors.New("permission denied")
	// ErrCertificate means the server's TLS certificate was not trusted
	ErrCertificate = errors.New("TLS certificate verification failed")
	// ErrNoConfig means there is no saved connection to use
	ErrNoConfig = errors.New("no configuration found")
)
