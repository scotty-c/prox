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
