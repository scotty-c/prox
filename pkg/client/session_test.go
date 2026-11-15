package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scotty-c/prox/pkg/config"
	"github.com/scotty-c/prox/pkg/secrets"
	"github.com/zalando/go-keyring"
)

// loginServer is a Proxmox that counts logins and only accepts the ticket
// "good-ticket" (and the one a login hands out)
type loginServer struct {
	*httptest.Server
	logins  int32
	cookies []string // the cookie of each API request, in order
	mu      sync.Mutex
}

func newLoginServer(t *testing.T) *loginServer {
	t.Helper()
	ls := &loginServer{}
	ls.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			n := atomic.AddInt32(&ls.logins, 1)
			w.Write([]byte(`{"data":{"ticket":"fresh-ticket-` + string(rune('0'+n)) + `","CSRFPreventionToken":"fresh-csrf"}}`))
			return
		}
		cookie := r.Header.Get("Cookie")
		ls.mu.Lock()
		ls.cookies = append(ls.cookies, cookie)
		ls.mu.Unlock()
		if strings.Contains(cookie, "stale-ticket") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"data":{"version":"8.2"}}`))
	}))
	t.Cleanup(ls.Close)
	return ls
}

type saved struct{ ticket, csrf string }

func connect(t *testing.T, ls *loginServer, session secrets.Session, save func(ticket, csrf string)) *ProxmoxClient {
	t.Helper()
	c, err := newClient(connection{URL: ls.URL, User: "root@pam", Password: "pw", Session: session, SaveSession: save}, TLSOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAFreshSessionSkipsTheLogin(t *testing.T) {
	ls := newLoginServer(t)
	var saves []saved
	c := connect(t, ls, secrets.Session{Ticket: "good-ticket", CSRF: "good-csrf", Saved: time.Now().Add(-10 * time.Minute)},
		func(ticket, csrf string) { saves = append(saves, saved{ticket, csrf}) })

	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&ls.logins); n != 0 {
		t.Errorf("%d logins, want 0 with a fresh session", n)
	}
	if len(ls.cookies) == 0 || !strings.Contains(ls.cookies[0], "good-ticket") {
		t.Errorf("the kept ticket should be used: %v", ls.cookies)
	}
	if len(saves) != 0 {
		t.Errorf("nothing new to save: %v", saves)
	}
}

func TestAStaleSessionIsNotUsed(t *testing.T) {
	ls := newLoginServer(t)
	var saves []saved
	old := secrets.Session{Ticket: "good-ticket", CSRF: "c", Saved: time.Now().Add(-secrets.SessionMaxAge - time.Minute)}
	c := connect(t, ls, old, func(ticket, csrf string) { saves = append(saves, saved{ticket, csrf}) })

	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&ls.logins); n != 1 {
		t.Errorf("%d logins, want 1", n)
	}
	if strings.Contains(ls.cookies[0], "good-ticket") {
		t.Errorf("an old ticket should not be sent: %v", ls.cookies)
	}
	if len(saves) != 1 || saves[0].ticket != "fresh-ticket-1" || saves[0].csrf != "fresh-csrf" {
		t.Errorf("the new login should be kept for next time: %v", saves)
	}
}

func TestWithoutASessionTheFirstRequestLogsInOnceAndKeepsIt(t *testing.T) {
	ls := newLoginServer(t)
	var saves []saved
	c := connect(t, ls, secrets.Session{}, func(ticket, csrf string) { saves = append(saves, saved{ticket, csrf}) })

	for i := 0; i < 3; i++ {
		if _, err := c.GetVersion(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if n := atomic.LoadInt32(&ls.logins); n != 1 {
		t.Errorf("%d logins for 3 requests, want 1", n)
	}
	if len(saves) != 1 {
		t.Errorf("saved %d times, want once: %v", len(saves), saves)
	}
}

func TestATicketTheServerRejectsCostsOneLoginAndIsReplaced(t *testing.T) {
	ls := newLoginServer(t)
	var saves []saved
	// looks fresh, but the server (say after a password change) turns it down
	c := connect(t, ls, secrets.Session{Ticket: "stale-ticket", CSRF: "c", Saved: time.Now()},
		func(ticket, csrf string) { saves = append(saves, saved{ticket, csrf}) })

	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatalf("the command should still succeed: %v", err)
	}
	if n := atomic.LoadInt32(&ls.logins); n != 1 {
		t.Errorf("%d logins, want exactly 1", n)
	}
	if len(ls.cookies) != 2 || !strings.Contains(ls.cookies[0], "stale-ticket") || !strings.Contains(ls.cookies[1], "fresh-ticket-1") {
		t.Errorf("expected the old ticket tried, then the new one: %v", ls.cookies)
	}
	if len(saves) != 1 || saves[0].ticket != "fresh-ticket-1" {
		t.Errorf("the replacement should be kept, so the next run does not hit the same 401: %v", saves)
	}
}

func TestAPITokensNeverLogInOrKeepAnything(t *testing.T) {
	ls := newLoginServer(t)
	var saves []saved
	c, err := newClient(connection{
		URL: ls.URL, User: "ci@pve!deploy", Password: "secret",
		Session:     secrets.Session{Ticket: "good-ticket", CSRF: "c", Saved: time.Now()},
		SaveSession: func(ticket, csrf string) { saves = append(saves, saved{ticket, csrf}) },
	}, TLSOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&ls.logins); n != 0 || len(saves) != 0 {
		t.Errorf("logins %d, saves %v; a token has no login to cache", n, saves)
	}
	if strings.Contains(strings.Join(ls.cookies, ""), "good-ticket") {
		t.Errorf("a token must not send a cached ticket: %v", ls.cookies)
	}
}

func TestNoSaveHookIsFine(t *testing.T) {
	ls := newLoginServer(t)
	c := connect(t, ls, secrets.Session{}, nil)
	if _, err := c.GetVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&ls.logins); n != 1 {
		t.Errorf("%d logins, want 1", n)
	}
}

// --- where the connection comes from

func sandboxConfig(t *testing.T, store string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home+"/.config")
	t.Setenv("PROX_URL", "")
	t.Setenv("PROX_USER", "")
	t.Setenv("PROX_PASSWORD", "")
	t.Setenv(secrets.EnvStore, store)
	t.Setenv(secrets.EnvSessionCache, "")
	keyring.MockInit()
}

func TestEnvironmentCredentialsNeverKeepASession(t *testing.T) {
	sandboxConfig(t, "file")
	t.Setenv("PROX_URL", "https://pve:8006")
	t.Setenv("PROX_USER", "root@pam")
	t.Setenv("PROX_PASSWORD", "pw")
	conn, err := readConnection()
	if err != nil || conn.User != "root@pam" || conn.SaveSession != nil || conn.Session.Ticket != "" {
		t.Errorf("%+v, %v", conn, err)
	}
}

func TestAKeychainProfileKeepsAndReusesASession(t *testing.T) {
	sandboxConfig(t, "keychain")
	if _, err := config.SaveProfile("default", "root@pam", "pw", "https://pve:8006", config.TLSOptions{}); err != nil {
		t.Fatal(err)
	}

	conn, err := readConnection()
	if err != nil || conn.SaveSession == nil || conn.Session.Ticket != "" {
		t.Fatalf("first run: %+v, %v", conn, err)
	}
	conn.SaveSession("PVE:root@pam:TICKET", "csrf-1") // what a login does

	detail, err := config.OpenProfile("default") // a later run reads it from the keychain
	if err != nil {
		t.Fatal(err)
	}
	if detail.Session.Ticket != "PVE:root@pam:TICKET" || detail.Session.CSRF != "csrf-1" || !detail.Session.Fresh(time.Now()) {
		t.Errorf("session = %+v", detail.Session)
	}
	if detail.Username != "root@pam" || detail.Password != "pw" {
		t.Errorf("the login must survive saving a session: %+v", detail)
	}
}

func TestChangingTheProfileDropsTheKeptSession(t *testing.T) {
	sandboxConfig(t, "keychain")
	_, _ = config.SaveProfile("default", "root@pam", "pw", "https://pve:8006", config.TLSOptions{})
	conn, _ := readConnection()
	conn.SaveSession("t", "c")

	if _, err := config.SaveProfile("default", "root@pam", "new-pw", "https://pve:8006", config.TLSOptions{}); err != nil { // as 'config update' does
		t.Fatal(err)
	}
	conn, err := readConnection()
	if err != nil || conn.Session.Ticket != "" || conn.Password != "new-pw" {
		t.Errorf("%+v, %v", conn, err)
	}
}

func TestTheSwitchTurnsTheSessionCacheOff(t *testing.T) {
	sandboxConfig(t, "keychain")
	_, _ = config.SaveProfile("default", "root@pam", "pw", "https://pve:8006", config.TLSOptions{})
	t.Setenv(secrets.EnvSessionCache, "off")
	conn, err := readConnection()
	if err != nil || conn.SaveSession != nil || conn.Session.Ticket != "" {
		t.Errorf("%+v, %v", conn, err)
	}
}

func TestAKeyFileProfileKeepsNothing(t *testing.T) {
	sandboxConfig(t, "file")
	_, _ = config.SaveProfile("default", "root@pam", "pw", "https://pve:8006", config.TLSOptions{})
	conn, err := readConnection()
	if err != nil || conn.SaveSession != nil || conn.Session.Ticket != "" || conn.User != "root@pam" {
		t.Errorf("%+v, %v; only the keychain keeps a session", conn, err)
	}
}
