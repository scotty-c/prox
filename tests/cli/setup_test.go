package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scotty-c/prox/internal/fakepve"
)

func profileExists(home, name string) bool {
	_, err := os.Stat(filepath.Join(home, ".prox", "profiles", name))
	return err == nil
}

// A server with the self-signed certificate Proxmox ships with

func TestSetupNeverTrustsAnUnknownCertificateWithoutBeingTold(t *testing.T) {
	srv, home := fakepve.NewTLS(t), t.TempDir()
	r := runInHome(t, home, srv, "yes\n", "config", "setup", "lab", "-u", "root@pam", "-p", "pw", "-l", srv.URL)
	if r.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", r.code, r.stderr)
	}
	// the fingerprint to compare is shown, with the way to accept it
	contains(t, "stderr", r.stderr, srv.Fingerprint(), "--fingerprint "+srv.Fingerprint(), "nothing was saved", "Proxmox web interface")
	if profileExists(home, "lab") {
		t.Error("a profile was saved for a server nobody trusted")
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("no HTTP request may reach an untrusted server, got %v", srv.Requests())
	}
}

func TestSetupPinsTheFingerprintYouGiveAndLaterRunsUseIt(t *testing.T) {
	srv, home := fakepve.NewTLS(t), t.TempDir()
	// the Proxmox web interface shows upper case with colons; lower case without is accepted too
	given := strings.ToLower(strings.ReplaceAll(srv.Fingerprint(), ":", ""))
	r := runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam", "-p", "pw", "-l", srv.URL, "--fingerprint", given)
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stdout", r.stdout, "Profile 'lab' saved", "Connected to Proxmox VE 8", "only the certificate with SHA-256 fingerprint "+srv.Fingerprint())
	contains(t, "profile", profileFile(t, home, "lab"), "fingerprint="+srv.Fingerprint())

	// the saved profile now connects, with certificate checking still on
	r = runWithProfile(t, home, "", "vm", "list", "--profile", "lab")
	if r.code != 0 {
		t.Fatalf("vm list with the pinned profile: exit %d\n%s", r.code, r.stderr)
	}
	contains(t, "vm list", r.stdout, "web")
}

func TestAWrongFingerprintSavesNothing(t *testing.T) {
	srv, home := fakepve.NewTLS(t), t.TempDir()
	wrong := strings.Repeat("AB:", 31) + "AB"
	r := runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam", "-p", "pw", "-l", srv.URL, "--fingerprint", wrong)
	if r.code != 1 {
		t.Fatalf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "does not match", wrong, srv.Fingerprint())
	if profileExists(home, "lab") {
		t.Error("a profile was saved with a fingerprint that does not match")
	}
	if len(srv.Requests()) != 0 {
		t.Errorf("credentials must not reach a server whose certificate does not match: %v", srv.Requests())
	}
}

func TestAMalformedFingerprintIsRejectedBeforeAnythingElse(t *testing.T) {
	srv := fakepve.NewTLS(t)
	for _, bad := range []string{"abc", strings.Repeat("zz", 32), "SHA256:" + strings.Repeat("ab", 32)} {
		r := runInHome(t, t.TempDir(), srv, "", "config", "setup", "lab", "-u", "u@pam", "-p", "pw", "-l", srv.URL, "--fingerprint", bad)
		if r.code != 1 || !strings.Contains(r.stderr, "fingerprint") {
			t.Errorf("--fingerprint %q: exit %d\n%s", bad, r.code, r.stderr)
		}
	}
	r := runInHome(t, t.TempDir(), srv, "", "config", "setup", "lab", "-u", "u@pam", "-p", "pw", "-l", srv.URL, "--fingerprint", srv.Fingerprint(), "--ca-cert", "/x.pem")
	if r.code != 1 || !strings.Contains(r.stderr, "not both") {
		t.Errorf("--ca-cert with --fingerprint: exit %d\n%s", r.code, r.stderr)
	}
}

func TestACertificateThatChangesAfterPinningIsRefusedWithTheWayForward(t *testing.T) {
	srv, home := fakepve.NewTLS(t), t.TempDir()
	runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam", "-p", "pw", "-l", srv.URL, "--fingerprint", srv.Fingerprint())

	// the server's certificate is renewed: the pin no longer matches
	path := filepath.Join(home, ".prox", "profiles", "lab")
	data, _ := os.ReadFile(path)
	renewed := strings.Replace(string(data), srv.Fingerprint(), strings.Repeat("CD:", 31)+"CD", 1)
	if renewed == string(data) {
		t.Fatal("profile did not hold the fingerprint")
	}
	if err := os.WriteFile(path, []byte(renewed), 0o600); err != nil {
		t.Fatal(err)
	}

	before := len(srv.Requests()) // what setup's own check sent
	r := runWithProfile(t, home, "", "vm", "list", "--profile", "lab")
	if r.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", r.code, r.stderr)
	}
	contains(t, "stderr", r.stderr, "no longer matches", "prox config setup --fingerprint "+srv.Fingerprint())
	if after := srv.Requests(); len(after) != before {
		t.Errorf("the refused run sent requests to the server: %v", after[before:])
	}
}

func TestTheFingerprintCanBeGivenToAnyCommandWithoutASavedProfile(t *testing.T) {
	srv := fakepve.NewTLS(t)
	if r := run(t, srv, "", "vm", "list"); r.code != 1 || !strings.Contains(r.stderr, "certificate") {
		t.Fatalf("without trust: exit %d\n%s", r.code, r.stderr)
	}
	if r := run(t, srv, "", "vm", "list", "--fingerprint", srv.Fingerprint()); r.code != 0 {
		t.Fatalf("with the right fingerprint: exit %d\n%s", r.code, r.stderr)
	}
	if r := run(t, srv, "", "vm", "list", "--fingerprint", strings.Repeat("AB:", 31)+"AB"); r.code != 1 {
		t.Fatalf("with the wrong fingerprint: exit %d", r.code)
	}
}

func TestSetupWithInsecureSavesIt(t *testing.T) {
	srv, home := fakepve.NewTLS(t), t.TempDir()
	r := runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam", "-p", "pw", "-l", srv.URL, "--insecure")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stdout", r.stdout, "certificate verification is turned off")
	contains(t, "profile", profileFile(t, home, "lab"), "insecure=true")
}

// Checking the connection before saving

func TestSetupChecksTheLoginAndSavesNothingWhenItFails(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	srv.FailRequestWithStatus("POST /access/ticket", 401, "authentication failure")
	r := runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam", "-p", "wrong", "-l", srv.URL)
	if r.code != 1 {
		t.Fatalf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "nothing was saved", "--no-verify")
	if profileExists(home, "lab") {
		t.Error("the profile was saved even though the login failed")
	}

	// --no-verify saves it regardless, without contacting the server
	before := len(srv.Requests())
	r = runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "-u", "root@pam", "-p", "wrong", "-l", srv.URL)
	if r.code != 0 || !profileExists(home, "lab") {
		t.Fatalf("--no-verify: exit %d\n%s", r.code, r.stderr)
	}
	if len(srv.Requests()) != before {
		t.Error("--no-verify must not connect")
	}
}

func TestSetupReportsWhatItConnectedTo(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	r := runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam", "-p", "pw", "-l", srv.URL)
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stdout", r.stdout, "Profile 'lab' saved", "Connected to Proxmox VE", "as root@pam")
	if n := srv.Count("POST", "/access/ticket"); n != 1 {
		t.Errorf("%d logins to check the details, want 1", n)
	}
}

// One command for new and existing profiles

func TestSetupChangesOnlyWhatYouGiveOnAnExistingProfile(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "-u", "alice@pam", "-p", "first-secret", "-l", "https://one.example.com:8006", "--insecure")

	// a new URL: the user, the password and the TLS choice stay
	if r := runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "-l", "https://two.example.com:8006"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	r := runInHome(t, home, srv, "", "config", "read", "--profile", "lab", "--show-password")
	contains(t, "config read", r.stdout, "alice@pam", "first-secret", "two.example.com")
	contains(t, "profile", profileFile(t, home, "lab"), "insecure=true")

	// a new password from stdin: the rest stays
	if r := runInHome(t, home, srv, "second-secret\n", "config", "setup", "lab", "--no-verify", "--password-stdin"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	r = runInHome(t, home, srv, "", "config", "read", "--profile", "lab", "--show-password")
	contains(t, "config read", r.stdout, "alice@pam", "second-secret", "two.example.com")

	// asking for verification again turns checking back on
	if r := runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "--insecure=false"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	excludes(t, "profile", profileFile(t, home, "lab"), "insecure=true")
}

func TestSettingACAOrAFingerprintReplacesTheOtherOne(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not checked here"), 0o600); err != nil {
		t.Fatal(err)
	}
	fp := strings.Repeat("EF:", 31) + "EF"

	runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006", "--fingerprint", fp)
	contains(t, "profile", profileFile(t, home, "lab"), "fingerprint="+fp)

	runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "--ca-cert", ca)
	saved := profileFile(t, home, "lab")
	contains(t, "profile", saved, "ca_cert="+ca)
	excludes(t, "profile", saved, "fingerprint=")

	runInHome(t, home, srv, "", "config", "setup", "lab", "--no-verify", "--fingerprint", fp)
	saved = profileFile(t, home, "lab")
	contains(t, "profile", saved, "fingerprint="+fp)
	excludes(t, "profile", saved, "ca_cert=")
}

func TestSetupWithoutANameWorksOnTheCurrentProfile(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	if r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if !profileExists(home, "default") {
		t.Fatal("no default profile")
	}
	r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "-u", "other@pam")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	contains(t, "stdout", r.stdout, "Profile 'default' saved")
	r = runInHome(t, home, srv, "", "config", "read")
	contains(t, "config read", r.stdout, "other@pam", "https://pve:8006")
}

func TestOnlyTheFirstNamedProfileBecomesCurrentUnlessAskedTo(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	setup := func(args ...string) {
		t.Helper()
		all := append([]string{"config", "setup", "--no-verify", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006"}, args...)
		if r := runInHome(t, home, srv, "", all...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	current := func() string {
		data, _ := os.ReadFile(filepath.Join(home, ".prox", "current-profile"))
		return strings.TrimSpace(string(data))
	}

	setup("first")
	if current() != "first" {
		t.Errorf("current = %q after the first profile, want first", current())
	}
	setup("second")
	if current() != "first" {
		t.Errorf("current = %q: naming another profile must not switch to it", current())
	}
	setup("third", "--use")
	if current() != "third" {
		t.Errorf("current = %q after --use, want third", current())
	}
}

func TestSetupStillRefusesToSaveAnIncompleteNewProfile(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	r := runInHome(t, home, srv, "", "config", "setup", "lab", "-u", "root@pam")
	if r.code != 1 {
		t.Fatalf("exit %d, want 1", r.code)
	}
	contains(t, "stderr", r.stderr, "--password", "--url")
	if profileExists(home, "lab") {
		t.Error("an incomplete profile was saved")
	}
}

func TestSetupRejectsProfileNamesThatEscapeTheProfilesDirectory(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	r := runInHome(t, home, srv, "", "config", "setup", "--no-verify", "../escape", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006")
	if r.code != 1 {
		t.Fatalf("exit %d, want 1", r.code)
	}
	if _, err := os.Stat(filepath.Join(home, ".prox", "escape")); err == nil {
		t.Error("a file was written outside the profiles directory")
	}
}

// create and update are gone

func TestTheRemovedCommandsSayWhatToUseInstead(t *testing.T) {
	srv, home := fakepve.New(t), t.TempDir()
	for _, args := range [][]string{
		{"config", "create", "lab", "-u", "u@pam", "-p", "pw", "-l", "https://pve:8006"},
		{"config", "update", "--url", "https://pve:8006"},
	} {
		r := runInHome(t, home, srv, "", args...)
		if r.code != 1 {
			t.Errorf("%v: exit %d, want 1", args, r.code)
		}
		contains(t, strings.Join(args, " "), r.stderr, "was removed in 1.0", "prox config setup")
		excludes(t, strings.Join(args, " "), r.stderr, "unknown flag")
	}
	if profileExists(home, "lab") {
		t.Error("the removed create command saved a profile")
	}

	help := runInHome(t, home, srv, "", "config", "--help")
	excludes(t, "config help", help.stdout, "create", "update")
}
