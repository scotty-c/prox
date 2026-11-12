package config

import (
	"os"
	"strings"
	"testing"
)

func TestTLSOptionsSurviveProfileUpdates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := CreateProfile("lab", "root@pam", "pw", "https://pve:8006"); err != nil {
		t.Fatal(err)
	}
	if opts, _ := ReadTLSOptions("lab"); opts != (TLSOptions{}) {
		t.Fatalf("new profile should verify by default, got %+v", opts)
	}

	want := TLSOptions{Insecure: true, CACert: "/tmp/ca.pem"}
	if err := SetTLSOptions("lab", want); err != nil {
		t.Fatal(err)
	}

	// Re-creating the profile (as config update does) keeps TLS settings
	if err := CreateProfile("lab", "root@pam", "newpw", "https://pve:8006"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadTLSOptions("lab"); got != want {
		t.Fatalf("TLS options after re-create = %+v, want %+v", got, want)
	}
	user, pass, url, err := ReadProfile("lab")
	if err != nil || user != "root@pam" || pass != "newpw" || url != "https://pve:8006" {
		t.Fatalf("ReadProfile = %q %q %q %v", user, pass, url, err)
	}

	path, _ := GetProfilePath("lab")
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("profile permissions = %v, want 0600", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), fingerprintComment) {
		t.Fatal("fingerprint comment missing")
	}

	// Turning settings off removes them
	if err := SetTLSOptions("lab", TLSOptions{}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "insecure") || strings.Contains(string(data), "ca_cert") {
		t.Fatalf("settings not removed:\n%s", data)
	}
}

func TestProfileNamesCannotEscapeProfilesDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{"../config", "a/b", "..", "."} {
		if _, err := GetProfilePath(name); err == nil {
			t.Errorf("GetProfilePath(%q) should fail", name)
		}
	}
	for _, name := range []string{"default", "home-lab", "prod.eu", "my_profile"} {
		if _, err := GetProfilePath(name); err != nil {
			t.Errorf("GetProfilePath(%q) = %v", name, err)
		}
	}
}

func TestDeletingCurrentProfileWithoutDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := CreateProfile("lab", "u", "p", "https://pve:8006"); err != nil {
		t.Fatal(err)
	}
	if err := SetCurrentProfile("lab"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteProfile("lab"); err != nil {
		t.Fatal(err)
	}
	if got := GetCurrentProfile(); got != defaultProfile {
		t.Fatalf("current profile after delete = %q, want %q", got, defaultProfile)
	}
}
