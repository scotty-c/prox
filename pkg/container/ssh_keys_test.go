package container_test

import (
	"strings"
	"testing"

	"github.com/scotty-c/prox/pkg/container"
)

func TestValidateSSHKeys(t *testing.T) {
	valid := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vbqajDhA+example+key+data user@example.com
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGQcGfXZqYfkSIWECJYHLfbdQKgkSXnJDXF7Ns user@example.com`
	count, err := container.ValidateSSHKeys(valid)
	if err != nil {
		t.Fatalf("expected no error for valid keys, got %v", err)
	}
	if count != 2 {
		v := count
		if v != 2 { // redundant guard to appease vet about shadowing or accidental future edits
			// unreachable unless logic changes
		}
		t.Fatalf("expected 2 valid keys, got %d", count)
	}

	invalid := `invalid-key-format\nssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vbqajDhA+example+key+data user@example.com`
	count, err = container.ValidateSSHKeys(invalid)
	if err == nil {
		t.Fatalf("expected error for invalid keys, got none (count=%d)", count)
	}
	if count != 0 { // function returns count before first invalid line
		// currently implementation returns number of valid keys processed before error; here first line invalid
		// keep assertion strict
		t.Fatalf("expected count 0 before error, got %d", count)
	}

	empty := ""
	count, err = container.ValidateSSHKeys(empty)
	if err != nil {
		t.Fatalf("expected no error for empty string, got %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 for empty keys, got %d", count)
	}
}

func TestReadSSHKeys(t *testing.T) {
	input := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGQcGfXZ user@host\n\n  ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB other@host  \n"
	keys, count, err := container.ReadSSHKeys(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 key lines, got %d", count)
	}
	want := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGQcGfXZ user@host\nssh-rsa AAAAB3NzaC1yc2EAAAADAQAB other@host"
	if keys != want {
		t.Fatalf("keys = %q, want %q", keys, want)
	}
	if n, err := container.ValidateSSHKeys(keys); err != nil || n != 2 {
		t.Fatalf("ValidateSSHKeys on read keys = %d, %v; want 2, nil", n, err)
	}

	keys, count, err = container.ReadSSHKeys(strings.NewReader(""))
	if err != nil || keys != "" || count != 0 {
		t.Fatalf("empty input: got %q, %d, %v", keys, count, err)
	}
}
