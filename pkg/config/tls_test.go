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
