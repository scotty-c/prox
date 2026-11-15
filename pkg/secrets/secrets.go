// Package secrets protects the credentials saved in a profile.
//
// A profile's username and password are kept in one of two ways:
//
//   - in the operating system's keychain (macOS Keychain, Windows Credential
//     Manager, the Linux Secret Service). The profile file then holds only
//     "credentials=keychain" and no secret at all.
//   - sealed with a random key kept in a key file (see KeyFilePath), used when
//     there is no keychain to use, as on a headless server. The profile file
//     holds "enc:v2:" values.
//
// Profiles written by older versions are still read: their values were
// encrypted with a key derived from the user name and host name (so anyone who
// can run code as the user could derive it, and renaming the machine broke
// them) or were plain text. They are upgraded the next time the profile is
// written, or by 'prox config migrate'.
//
// What this does and does not give you is spelled out in docs/SECURITY.md.
package secrets

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/scotty-c/prox/pkg/crypto"
)

// Credentials are what a profile logs in with: a username and a password, or
// an API token ID and its secret. Session is the login Proxmox last gave
// prox for them, if it was kept (only profiles in the keychain keep one).
type Credentials struct {
	Username string
	Password string
	Session  Session
}

// Session is a login that was kept so the next run can skip logging in: the
// ticket and CSRF token Proxmox issued, and when
type Session struct {
	Ticket string
	CSRF   string
	Saved  time.Time
}

// SessionMaxAge is how long prox trusts a kept session. Proxmox tickets last
// two hours; stopping a little short avoids using one that is about to expire.
// A ticket the server turns down earlier (say after a password change) is
// handled by logging in again.
const SessionMaxAge = 110 * time.Minute

// Fresh reports whether the session is complete and young enough to use at now
func (s Session) Fresh(now time.Time) bool {
	age := now.Sub(s.Saved)
	return s.Ticket != "" && s.CSRF != "" && !s.Saved.IsZero() && age >= 0 && age < SessionMaxAge
}

// EnvSessionCache turns the kept session off when set to off, 0, false or no
const EnvSessionCache = "PROX_SESSION_CACHE"

// SessionCacheEnabled reports whether prox may keep and reuse a session
func SessionCacheEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvSessionCache))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

// Method says how credentials are, or were, protected
type Method string

const (
	MethodKeychain  Method = "keychain"
	MethodKeyFile   Method = "key file"
	MethodLegacy    Method = "legacy" // encrypted with a key derived from user and host
	MethodPlaintext Method = "plain text"
)

// Current reports whether the method is one new profiles are written with
func (m Method) Current() bool { return m == MethodKeychain || m == MethodKeyFile }

// EnvStore selects where new credentials go: "keychain", "file", or "auto"
// (the default), which tries the keychain and falls back to the key file
const EnvStore = "PROX_CREDENTIAL_STORE"

const (
	keychainEntry  = "credentials" // the profile file entry that points at the keychain
	keychainMarker = "keychain"
)

// allowAutoInTests lets this package's own tests exercise automatic mode
var allowAutoInTests bool

// underTest reports whether this is a "go test" binary. Tests must never write
// to the developer's real keychain, so automatic mode becomes "file" there.
func underTest() bool {
	return strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe")
}

func storeMode() (string, error) {
	switch v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvStore))); v {
	case "keychain", "file":
		return v, nil
	case "", "auto":
		if underTest() && !allowAutoInTests {
			return "file", nil
		}
		return "auto", nil
	default:
		return "", fmt.Errorf("invalid %s %q: use auto, keychain or file", EnvStore, v)
	}
}

// Sealed is credentials protected for storage
type Sealed struct {
	// Entries are the key=value lines for the profile file (never plain text)
	Entries map[string]string
	Method  Method
	// Fingerprint identifies the key file's key, for a comment in the profile
	// file that helps diagnose a mismatch; "" when no key file is used
	Fingerprint string
	// Note is something the user should be told, such as why the keychain was
	// not used; "" if there is nothing to say
	Note string
}

// Seal protects creds for the profile. In automatic mode it uses the keychain
// and falls back to the key file if the keychain cannot be used, saying why in
// Note.
func Seal(profile string, creds Credentials) (Sealed, error) {
	mode, err := storeMode()
	if err != nil {
		return Sealed{}, err
	}

	if mode != "file" {
		err := keychainSet(profile, creds)
		if err == nil {
			return Sealed{Entries: map[string]string{keychainEntry: keychainMarker}, Method: MethodKeychain}, nil
		}
		if mode == "keychain" {
			return Sealed{}, fmt.Errorf("could not store the credentials in the keychain: %w", err)
		}
		sealed, fileErr := sealWithKeyFile(creds)
		if fileErr != nil {
			return Sealed{}, fileErr
		}
		sealed.Note = fmt.Sprintf("No usable OS keychain (%v); the credentials are protected with a key file instead", err)
		return sealed, nil
	}
	return sealWithKeyFile(creds)
}

func sealWithKeyFile(creds Credentials) (Sealed, error) {
	key, _, err := loadKey(true)
	if err != nil {
		return Sealed{}, err
	}
	username, err := crypto.Seal(key, creds.Username)
	if err != nil {
		return Sealed{}, err
	}
	password, err := crypto.Seal(key, creds.Password)
	if err != nil {
		return Sealed{}, err
	}
	return Sealed{
		Entries:     map[string]string{"username": username, "password": password},
		Method:      MethodKeyFile,
		Fingerprint: crypto.KeyFingerprint(key),
	}, nil
}

// Open recovers the credentials of the profile from the entries read from its
// file, whichever way they were protected. A failure to open credentials that
// were sealed by a key or keychain item that is gone or does not match says so
// and how to fix it.
func Open(profile string, entries map[string]string) (Credentials, Method, error) {
	reenter := fmt.Sprintf("re-enter them with: prox config create %s -u <user> -p <password> -l <url>", profile)

	if entries[keychainEntry] == keychainMarker {
		creds, err := keychainGet(profile)
		switch {
		case errors.Is(err, errNotInKeychain):
			return Credentials{}, "", fmt.Errorf("the credentials for profile '%s' are not in the keychain (they may have been deleted); %s", profile, reenter)
		case err != nil:
			return Credentials{}, "", fmt.Errorf("could not read the credentials for profile '%s' from the keychain: %w (a locked keychain must be unlocked first)", profile, err)
		}
		return creds, MethodKeychain, nil
	}

	username, password := entries["username"], entries["password"]

	if crypto.IsSealed(username) || crypto.IsSealed(password) {
		key, path, err := loadKey(false)
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, "", fmt.Errorf("the credentials for profile '%s' are sealed with a key that is missing (expected at %s, and it cannot be recreated); %s", profile, path, reenter)
		}
		if err != nil {
			return Credentials{}, "", err
		}
		u, uErr := crypto.Open(key, username)
		p, pErr := crypto.Open(key, password)
		if err := errors.Join(uErr, pErr); err != nil {
			if errors.Is(err, crypto.ErrWrongKey) {
				return Credentials{}, "", fmt.Errorf("the key file %s (fingerprint %s) does not match the credentials of profile '%s' (it may have been replaced); %s", path, crypto.KeyFingerprint(key), profile, reenter)
			}
			return Credentials{}, "", fmt.Errorf("failed to read the credentials of profile '%s': %w", profile, err)
		}
		return Credentials{Username: u, Password: p}, MethodKeyFile, nil
	}

	// Older profiles: encrypted with the derived key, or plain text
	u, err := crypto.Decrypt(username)
	if err != nil {
		return Credentials{}, "", err
	}
	p, err := crypto.Decrypt(password)
	if err != nil {
		return Credentials{}, "", err
	}
	if u != username || p != password {
		return Credentials{Username: u, Password: p}, MethodLegacy, nil
	}
	return Credentials{Username: u, Password: p}, MethodPlaintext, nil
}

// Forget removes what is stored outside the profile file for a profile that
// is being deleted: its keychain item, if its entries say it has one. The key
// file is shared by all profiles and is left alone.
func Forget(profile string, entries map[string]string) error {
	if entries[keychainEntry] != keychainMarker {
		return nil
	}
	if err := keychainDelete(profile); err != nil {
		return fmt.Errorf("could not remove the keychain item for profile '%s': %w", profile, err)
	}
	return nil
}

// SaveSession keeps a session in the profile's keychain item, next to the
// credentials, so the next run can skip logging in. Only keychain profiles can
// keep one; for any other profile it does nothing. Failing to save is not
// serious (the next run just logs in), so callers may ignore the error.
func SaveSession(profile string, session Session) error {
	creds, err := keychainGet(profile)
	if err != nil {
		return err
	}
	creds.Session = session
	return keychainSet(profile, creds)
}
