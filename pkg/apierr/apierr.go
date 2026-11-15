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
	// ErrNotFound means no VM, container or node matched what was asked for
	ErrNotFound = errors.New("not found")
	// ErrAmbiguous means a name matched more than one VM or container
	ErrAmbiguous = errors.New("ambiguous name")
)

// WithKind returns an error that prints as msg but matches kind with
// errors.Is. It keeps a long, ready-made message (with suggestions) while
// letting callers tell "not found" from "ambiguous" without reading it.
func WithKind(kind error, msg string) error {
	return &kindError{kind: kind, msg: msg}
}

type kindError struct {
	kind error
	msg  string
}

func (e *kindError) Error() string { return e.msg }
func (e *kindError) Unwrap() error { return e.kind }
