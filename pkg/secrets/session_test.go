package secrets

import (
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestSessionFreshness(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		s    Session
		want bool
	}{
		{"just saved", Session{"t", "c", now}, true},
		{"an hour old", Session{"t", "c", now.Add(-time.Hour)}, true},
		{"one minute before the limit", Session{"t", "c", now.Add(-SessionMaxAge + time.Minute)}, true},
		{"at the limit", Session{"t", "c", now.Add(-SessionMaxAge)}, false},
		{"two hours old", Session{"t", "c", now.Add(-2 * time.Hour)}, false},
		{"saved in the future (clock moved)", Session{"t", "c", now.Add(time.Hour)}, false},
		{"no ticket", Session{"", "c", now}, false},
		{"no CSRF token", Session{"t", "", now}, false},
		{"never saved", Session{"t", "c", time.Time{}}, false},
		{"empty", Session{}, false},
	}
	for _, tt := range tests {
		if got := tt.s.Fresh(now); got != tt.want {
			t.Errorf("%s: Fresh = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestASavedSessionComesBackWithTheCredentialsIntact(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "keychain")
	sealed, err := Seal("prod", creds)
	if err != nil {
		t.Fatal(err)
	}
	if got, _, _ := Open("prod", sealed.Entries); got.Session.Ticket != "" {
		t.Fatalf("a new profile has no session: %+v", got.Session)
	}

	saved := time.Now().Truncate(time.Second)
	if err := SaveSession("prod", Session{Ticket: "PVE:root@pam:ABC", CSRF: "csrf-token", Saved: saved}); err != nil {
		t.Fatal(err)
	}

	got, method, err := Open("prod", sealed.Entries)
	if err != nil || method != MethodKeychain {
		t.Fatalf("Open: %q, %v", method, err)
	}
	if got.Username != creds.Username || got.Password != creds.Password {
		t.Errorf("saving a session must keep the login: %+v", got)
	}
	if got.Session.Ticket != "PVE:root@pam:ABC" || got.Session.CSRF != "csrf-token" || !got.Session.Saved.Equal(saved) {
		t.Errorf("session = %+v", got.Session)
	}
	if !got.Session.Fresh(time.Now()) {
		t.Error("a session saved just now should be fresh")
	}
}

func TestSavingTheCredentialsAgainDropsTheOldSession(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "keychain")
	sealed, _ := Seal("prod", creds)
	_ = SaveSession("prod", Session{Ticket: "t", CSRF: "c", Saved: time.Now()})

	// what 'config update' does: a changed password makes the old ticket useless
	if _, err := Seal("prod", Credentials{Username: creds.Username, Password: "new-password"}); err != nil {
		t.Fatal(err)
	}
	got, _, _ := Open("prod", sealed.Entries)
	if got.Password != "new-password" || got.Session.Ticket != "" {
		t.Errorf("after re-saving: %+v", got)
	}
}

func TestSavingASessionNeverDisturbsAnotherProfile(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "keychain")
	a, _ := Seal("a", Credentials{Username: "ua", Password: "pa"})
	b, _ := Seal("b", Credentials{Username: "ub", Password: "pb"})
	_ = SaveSession("a", Session{Ticket: "ta", CSRF: "ca", Saved: time.Now()})

	gotB, _, _ := Open("b", b.Entries)
	gotA, _, _ := Open("a", a.Entries)
	if gotB.Session.Ticket != "" || gotB.Password != "pb" || gotA.Session.Ticket != "ta" || gotA.Password != "pa" {
		t.Errorf("a = %+v, b = %+v", gotA, gotB)
	}
}

func TestItemsSavedBeforeSessionsExistedStillOpen(t *testing.T) {
	sandbox(t)
	// what the previous version wrote: no ticket fields at all
	if err := keyring.Set(keychainService, "old", `{"username":"root@pam","password":"pw"}`); err != nil {
		t.Fatal(err)
	}
	got, method, err := Open("old", map[string]string{"credentials": "keychain"})
	if err != nil || method != MethodKeychain || got.Username != "root@pam" || got.Password != "pw" || got.Session.Fresh(time.Now()) {
		t.Errorf("%+v, %q, %v", got, method, err)
	}
}

func TestSavingASessionForAProfileNotInTheKeychainFails(t *testing.T) {
	sandbox(t)
	if err := SaveSession("nowhere", Session{Ticket: "t", CSRF: "c", Saved: time.Now()}); err == nil {
		t.Error("there is no item to add a session to; callers rely on an error here, not a new item")
	}
	if _, err := keychainGet("nowhere"); err == nil {
		t.Error("SaveSession must not create an item with no credentials")
	}
}

func TestSessionCacheCanBeSwitchedOff(t *testing.T) {
	for value, want := range map[string]bool{"": true, "on": true, "1": true, "off": false, "OFF": false, "0": false, "false": false, "no": false, " off ": false} {
		t.Setenv(EnvSessionCache, value)
		if got := SessionCacheEnabled(); got != want {
			t.Errorf("%s=%q: enabled = %v, want %v", EnvSessionCache, value, got, want)
		}
	}
}
