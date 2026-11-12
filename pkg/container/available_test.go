package container

import (
	"testing"

	c "github.com/scotty-c/prox/pkg/client"
)

func TestMatchAvailable(t *testing.T) {
	templates := toAvailable([]c.Appliance{
		{Template: "debian-12-standard_12.2-1_amd64.tar.zst", Section: "system"},
		{Template: "debian-12-standard_12.7-1_amd64.tar.zst", Section: "system"},
		{Template: "debian-12-turnkey-core_18.0-1_amd64.tar.gz", Section: "system"},
		{Template: "ubuntu-24.04-standard_24.04-2_amd64.tar.zst", Section: "system"},
		{Template: "alpine-3.20-default_20240908_amd64.tar.xz", Section: "system"},
		{Template: "turnkey-wordpress_18.1-1_amd64.tar.gz", Section: "turnkeylinux"},
	})
	if len(templates) != 5 {
		t.Fatalf("expected 5 system templates, got %d", len(templates))
	}

	tests := []struct {
		want, got string
		wantErr   bool
	}{
		{"debian:12", "debian-12-standard_12.7-1_amd64.tar.zst", false}, // newest standard
		{"Ubuntu:24.04", "ubuntu-24.04-standard_24.04-2_amd64.tar.zst", false},
		{"alpine:3.20", "alpine-3.20-default_20240908_amd64.tar.xz", false},
		{"debian-12-standard_12.2-1_amd64.tar.zst", "debian-12-standard_12.2-1_amd64.tar.zst", false},
		{"debian:11", "", true},
		{"nonsense", "", true},
	}
	for _, tt := range tests {
		got, err := matchAvailable(templates, tt.want)
		if (err != nil) != tt.wantErr || got.Name != tt.got {
			t.Errorf("matchAvailable(%q) = %q, %v; want %q, error %v", tt.want, got.Name, err, tt.got, tt.wantErr)
		}
	}
}

func TestPickTemplateStorage(t *testing.T) {
	if s, _ := pickTemplateStorage([]string{"nfs", "local"}); s != "local" {
		t.Errorf("got %q, want local", s)
	}
	if s, _ := pickTemplateStorage([]string{"nfs"}); s != "nfs" {
		t.Errorf("got %q, want nfs", s)
	}
	if _, err := pickTemplateStorage(nil); err == nil {
		t.Error("expected error with no storage")
	}
}
