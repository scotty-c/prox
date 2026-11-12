package cmd

import (
	"strings"
	"testing"
)

func TestGenerateSSHConfigEntry(t *testing.T) {
	got := generateSSHConfigEntry("web", "10.0.0.5", "alice", 22, "", false)
	want := "Host web\n    HostName 10.0.0.5\n    User alice\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	got = generateSSHConfigEntry("web", "10.0.0.5", "", 2222, "~/.ssh/id", true)
	for _, line := range []string{"Port 2222", "IdentityFile ~/.ssh/id", "StrictHostKeyChecking no"} {
		if !strings.Contains(got, line) {
			t.Errorf("missing %q in %q", line, got)
		}
	}
	if strings.Contains(got, "    User ") {
		t.Errorf("empty user should omit User line: %q", got)
	}
}

func TestLocalUsernameHasNoDomain(t *testing.T) {
	if name := localUsername(); strings.Contains(name, `\`) {
		t.Fatalf("localUsername() = %q still has a domain", name)
	}
}

func TestSSHCommandArgs(t *testing.T) {
	tests := []struct {
		ip, user, key string
		port          int
		skip          bool
		want          string
	}{
		{"10.0.0.5", "alice", "", 22, false, "ssh alice@10.0.0.5"},
		{"10.0.0.5", "", "", 2222, false, "ssh -p 2222 10.0.0.5"},
		{"fd00::5", "root", "~/.ssh/k", 22, true, "ssh -i ~/.ssh/k -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@fd00::5"},
	}
	for _, tt := range tests {
		got := strings.Join(sshCommandArgs(tt.ip, tt.user, tt.port, tt.key, tt.skip), " ")
		if got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"alice@10.0.0.5":     "alice@10.0.0.5",
		"~/.ssh/id_ed25519":  "~/.ssh/id_ed25519",
		"/path with space/k": "'/path with space/k'",
		"it's":               `'it'"'"'s'`,
		"":                   "''",
		"$(rm -rf /)":        "'$(rm -rf /)'",
	}
	for in, want := range tests {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
