package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

// keychainService is the service name prox's items have in the keychain; the
// account is the profile name
const keychainService = "prox"

// keychainItem is what is stored for a profile: one item holds both values
type keychainItem struct {
	Username string `json:"username"`
	Password string `json:"password"`

	// the last login, if one was kept (see Session)
	Ticket string `json:"ticket,omitempty"`
	CSRF   string `json:"csrf,omitempty"`
	Saved  int64  `json:"saved,omitempty"` // Unix seconds
}

func keychainSet(profile string, creds Credentials) error {
	item := keychainItem{Username: creds.Username, Password: creds.Password}
	if s := creds.Session; s.Ticket != "" {
		item.Ticket, item.CSRF, item.Saved = s.Ticket, s.CSRF, s.Saved.Unix()
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return keyring.Set(keychainService, profile, string(data))
}

// errNotInKeychain means the keychain works but has no item for the profile
var errNotInKeychain = errors.New("no item for this profile")

func keychainGet(profile string) (Credentials, error) {
	data, err := keyring.Get(keychainService, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return Credentials{}, errNotInKeychain
	}
	if err != nil {
		return Credentials{}, err
	}
	var item keychainItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return Credentials{}, fmt.Errorf("the keychain item is not in the expected format: %w", err)
	}
	creds := Credentials{Username: item.Username, Password: item.Password}
	if item.Ticket != "" && item.Saved > 0 {
		creds.Session = Session{Ticket: item.Ticket, CSRF: item.CSRF, Saved: time.Unix(item.Saved, 0)}
	}
	return creds, nil
}

// keychainDelete removes the profile's item; an item that is not there is fine
func keychainDelete(profile string) error {
	if err := keyring.Delete(keychainService, profile); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}
