package config

import (
	"os"
	"strings"
	"testing"
)

func TestTLSOptionsAreSavedWithTheProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := SaveProfile("lab", "root@pam", "pw", "https://pve:8006", TLSOptions{}); err != nil {
		t.Fatal(err)
	}
	if opts, _ := ReadTLSOptions("lab"); opts != (TLSOptions{}) {
		t.Fatalf("new profile should verify by default, got %+v", opts)
	}

	fp, err := NormalizeFingerprint(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	want := TLSOptions{Insecure: true, CACert: "/tmp/ca.pem", Fingerprint: fp}
	if _, err := SaveProfile("lab", "root@pam", "newpw", "https://pve:8006", want); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadTLSOptions("lab"); got != want {
		t.Fatalf("TLS options after saving = %+v, want %+v", got, want)
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

	// Saving different TLS settings replaces them, and turning them off removes them
	if err := SetTLSOptions("lab", TLSOptions{CACert: "/tmp/other.pem"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadTLSOptions("lab"); got != (TLSOptions{CACert: "/tmp/other.pem"}) {
		t.Fatalf("after replacing: %+v", got)
	}
	if err := SetTLSOptions("lab", TLSOptions{}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	for _, key := range []string{"insecure", "ca_cert", "fingerprint="} {
		if strings.Contains(string(data), key) {
			t.Fatalf("%s not removed:\n%s", key, data)
		}
	}
}

func TestNormalizeFingerprint(t *testing.T) {
	const want = "AB:CD:EF:01:23:45:67:89:AB:CD:EF:01:23:45:67:89:AB:CD:EF:01:23:45:67:89:AB:CD:EF:01:23:45:67:89"
	for _, in := range []string{
		want,
		strings.ToLower(want),
		strings.ReplaceAll(want, ":", ""),
		"  " + strings.ReplaceAll(strings.ToLower(want), ":", " ") + "\n",
	} {
		got, err := NormalizeFingerprint(in)
		if err != nil || got != want {
			t.Errorf("NormalizeFingerprint(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", strings.Repeat("ab", 31), strings.Repeat("ab", 33), strings.Repeat("zz", 32), "SHA256:" + strings.Repeat("ab", 32)} {
		if got, err := NormalizeFingerprint(in); err == nil {
			t.Errorf("NormalizeFingerprint(%q) = %q, want an error", in, got)
		}
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
	if _, err := SaveProfile("lab", "u", "p", "https://pve:8006", TLSOptions{}); err != nil {
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
