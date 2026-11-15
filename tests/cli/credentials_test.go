package cli

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scotty-c/prox/internal/fakepve"
	"github.com/scotty-c/prox/pkg/crypto"
)

// keyFilePath is where the harness's HOME puts the key file (XDG_CONFIG_HOME
// is set to HOME/.config; macOS ignores that and uses Library/Application Support)
func keyFilePaths(home string) []string {
	return []string{
		filepath.Join(home, ".config", "prox", "key"),
		filepath.Join(home, "Library", "Application Support", "prox", "key"),
	}
}

func findKeyFile(t *testing.T, home string) string {
	t.Helper()
	for _, p := range keyFilePaths(home) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func profileFile(t *testing.T, home, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".prox", "profiles", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeLegacyProfile saves a profile the way versions before this one did: the
// username and password encrypted with the key derived from the user and host
func writeLegacyProfile(t *testing.T, home, name, url, user, password, extra string) {
	t.Helper()
	u, err := crypto.Encrypt(user)
	if err != nil {
		t.Fatal(err)
	}
	p, err := crypto.Encrypt(password)
	if err != nil {
		t.Fatal(err)
	}
	fp, _ := crypto.GetKeyFingerprint()
	content := "username=" + u + "\npassword=" + p + "\nurl=" + url + "\n" + extra + "# Encrypted config - key fingerprint: " + fp + "\n"
	dir := filepath.Join(home, ".prox", "profiles")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNewProfilesHoldNoReadableSecretAndUseAPrivateKeyFile(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	r := runInHome(t, home, srv, "", "config", "create", "lab", "-u", "root@pam", "-p", "hunter2-secret", "-l", "https://pve:8006")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}

	contains(t, "create output", r.stdout, "Profile 'lab' created successfully (credentials stored sealed with a key file)")

	saved := profileFile(t, home, "lab")
	excludes(t, "profile file", saved, "root@pam", "hunter2-secret")
	contains(t, "profile file", saved, "username=enc:v2:", "password=enc:v2:", "url=https://pve:8006", "key fingerprint")

	key := findKeyFile(t, home)
	if key == "" {
		t.Fatalf("no key file under %v", keyFilePaths(home))
	}
	if strings.Contains(key, ".prox") {
		t.Errorf("the key must not live beside the profiles: %s", key)
	}
	if info, _ := os.Stat(key); info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode %v, want 0600", info.Mode().Perm())
	}

	// it reads back, and list says how it is stored
	r = runInHome(t, home, srv, "", "config", "read", "--profile", "lab", "--show-password")
	contains(t, "config read", r.stdout, "root@pam", "hunter2-secret")
	r = runInHome(t, home, srv, "", "config", "list")
	contains(t, "config list", r.stdout, "key file")
	excludes(t, "config list", r.stdout+r.stderr, "hunter2-secret", "enc:v2")
}

func TestAnOldProfileKeepsWorkingAndSaysHowToUpgrade(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	writeLegacyProfile(t, home, "default", srv.URL, "root@pam", "old-secret", "")

	r := runWithProfile(t, home, "", "vm", "list")
	if r.code != 0 {
		t.Fatalf("an old profile must keep working: exit %d: %s", r.code, r.stderr)
	}
	contains(t, "vm list", r.stdout, "web")
	contains(t, "stderr", r.stderr, "old storage scheme", "prox config migrate")
	if n := strings.Count(r.stderr, "old storage scheme"); n != 1 {
		t.Errorf("the nudge should appear once per run, saw it %d times", n)
	}

	// it can be silenced like any progress message
	r = runWithProfile(t, home, "", "vm", "list", "-q")
	excludes(t, "quiet stderr", r.stderr, "old storage scheme")
	// and it does not touch the output formats
	r = runWithProfile(t, home, "", "vm", "list", "-o", "json")
	excludes(t, "json stderr", r.stderr, "old storage scheme")
	if !strings.HasPrefix(strings.TrimSpace(r.stdout), "[") {
		t.Errorf("stdout should still be JSON: %.80s", r.stdout)
	}

	r = runWithProfile(t, home, "", "config", "list")
	contains(t, "config list", r.stdout, "legacy", "prox config migrate")
}

func TestMigrateMovesOldProfilesAndKeepsEverythingElse(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	writeLegacyProfile(t, home, "default", srv.URL, "root@pam", "old-secret", "insecure=true\n")
	oldFile := profileFile(t, home, "default")

	r := runWithProfile(t, home, "", "config", "migrate")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "migrate", r.stdout, "default", "legacy", "key file", "moved")

	now := profileFile(t, home, "default")
	if now == oldFile {
		t.Fatal("the profile was not rewritten")
	}
	contains(t, "profile file", now, "username=enc:v2:", "password=enc:v2:", "insecure=true", "url="+srv.URL)
	for _, line := range strings.Split(oldFile, "\n") {
		if strings.HasPrefix(line, "password=") && strings.Contains(now, line) {
			t.Errorf("the old ciphertext is still in the file: %s", line)
		}
	}

	// same login, and no more nudging
	r = runWithProfile(t, home, "", "config", "read", "--show-password")
	contains(t, "config read", r.stdout, "root@pam", "old-secret")
	r = runWithProfile(t, home, "", "vm", "list")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	excludes(t, "stderr", r.stderr, "old storage scheme")

	// running it again has nothing to do
	r = runWithProfile(t, home, "", "config", "migrate")
	contains(t, "second migrate", r.stdout, "already secure")
	excludes(t, "second migrate", r.stdout, "moved")
}

func TestMigrateOnePlainTextProfileLeavesTheOthersAlone(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".prox", "profiles")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plain"), []byte("username=root@pam\npassword=short pw\nurl=https://pve:8006\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeLegacyProfile(t, home, "other", "https://other:8006", "u@pam", "other-secret", "")
	otherBefore := profileFile(t, home, "other")

	r := runWithProfile(t, home, "", "config", "migrate", "plain")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "migrate", r.stdout, "plain text", "key file", "moved")
	excludes(t, "plain profile", profileFile(t, home, "plain"), "short pw", "root@pam")
	if profileFile(t, home, "other") != otherBefore {
		t.Error("a profile that was not named must not change")
	}
}

func TestAMissingKeyFileIsExplainedAndNeverQuietlyReplaced(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	key := findKeyFile(t, home)
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}

	r := runWithProfile(t, home, "", "config", "read", "--profile", "lab")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "missing", "prox config create lab")
	if findKeyFile(t, home) != "" {
		t.Error("reading must not create a key: a new one could not open the old values")
	}
}

func TestReplacingTheKeyFileIsReportedNotSilentlyAccepted(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	key := findKeyFile(t, home)
	other, _ := crypto.NewKey()
	if err := os.WriteFile(key, []byte(encodeKey(other)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := runWithProfile(t, home, "", "config", "read", "--profile", "lab")
	if r.code != 1 {
		t.Errorf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "does not match", "fingerprint")
}

func TestDeletingAProfileKeepsTheSharedKeyForTheOthers(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "a", "-u", "u@pam", "-p", "pw-a", "-l", "https://a:8006")
	runInHome(t, home, srv, "", "config", "create", "b", "-u", "u@pam", "-p", "pw-b", "-l", "https://b:8006")
	if r := runInHome(t, home, srv, "", "config", "delete", "a", "--yes"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	r := runInHome(t, home, srv, "", "config", "read", "--profile", "b", "--show-password")
	contains(t, "config read b", r.stdout, "pw-b")
}

func TestListJSONSaysHowEachProfileIsStored(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "new", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	writeLegacyProfile(t, home, "old", "https://pve:8006", "u@pam", "pw", "")
	r := runInHome(t, home, srv, "", "config", "list", "-o", "json")
	contains(t, "json", r.stdout, `"storage": "key file"`, `"storage": "legacy"`)
	excludes(t, "json", r.stdout, "enc:v2", "password")
}

func encodeKey(key []byte) string { return base64.StdEncoding.EncodeToString(key) }

func TestSetupSaysWhereTheCredentialsWentToo(t *testing.T) {
	r := runInHome(t, t.TempDir(), fakepve.New(t), "", "config", "setup", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "setup output", r.stdout, "Config setup completed successfully (credentials stored sealed with a key file)")
}

func TestKeyFileProfilesLogInEveryRunAndKeepNoSession(t *testing.T) {
	home, srv := t.TempDir(), fakepve.New(t)
	runInHome(t, home, srv, "", "config", "create", "default", "-u", "root@pam", "-p", "pw", "-l", srv.URL)

	for i := 1; i <= 2; i++ {
		if r := runWithProfile(t, home, "", "vm", "list"); r.code != 0 {
			t.Fatalf("run %d: exit %d: %s", i, r.code, r.stderr)
		}
	}
	if n := srv.Count("POST", "/access/ticket"); n != 2 {
		t.Errorf("%d logins over 2 runs, want 2: only keychain profiles keep a session", n)
	}

	// the login's ticket is "T" in the fake; nothing under home may hold it
	filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if data, _ := os.ReadFile(path); strings.Contains(string(data), "CSRFPreventionToken") || strings.Contains(string(data), "PVEAuthCookie") {
				t.Errorf("%s looks like it holds a session", path)
			}
		}
		return nil
	})
}

func TestEnvironmentCredentialsLogInEveryRun(t *testing.T) {
	srv := fakepve.New(t)
	for i := 0; i < 2; i++ {
		run(t, srv, "", "vm", "list") // run() uses PROX_URL/PROX_USER/PROX_PASSWORD
	}
	if n := srv.Count("POST", "/access/ticket"); n != 2 {
		t.Errorf("%d logins over 2 runs, want 2", n)
	}
}
