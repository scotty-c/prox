package container

import "testing"

func TestParseTemplateFields(t *testing.T) {
	tests := []struct {
		volID       string
		os          string
		version     string
		description string
	}{
		{"local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst", "Ubuntu", "22.04", "ubuntu-22.04-standard 22.04-1 amd64"},
		{"local:vztmpl/ubuntu-20.04-standard_20.04-1_amd64.tar.gz", "Ubuntu", "20.04", "ubuntu-20.04-standard 20.04-1 amd64"},
		{"local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", "Debian", "12", "debian-12-standard 12.7-1 amd64"},
		{"local:vztmpl/debian-11-standard_11.7-1_amd64.tar.zst", "Debian", "11", "debian-11-standard 11.7-1 amd64"},
		{"local:vztmpl/alpine-3.20-default_20240908_amd64.tar.xz", "Alpine", "3.20", "alpine-3.20-default 20240908 amd64"},
		{"local:vztmpl/centos-9-stream-default_20240828_amd64.tar.xz", "CentOS", "9", "centos-9-stream-default 20240828 amd64"},
		{"local:vztmpl/fedora-40-default_20240909_amd64.tar.xz", "Fedora", "40", "fedora-40-default 20240909 amd64"},
		{"local:vztmpl/rockylinux-9-default_20240912_amd64.tar.xz", "Rockylinux", "9", "rockylinux-9-default 20240912 amd64"},
		// Non-standard names fall back to the legacy heuristics
		{"local:vztmpl/turnkey-wordpress-18.1-1_amd64.tar.gz", "Turnkey", "18.1", "turnkey-wordpress-18.1-1 amd64"},
		{"local:vztmpl/my-ubuntu-custom.tar.gz", "Ubuntu", "Unknown", "my-ubuntu-custom"},
		{"nopath", "Unknown", "Unknown", "nopath"},
	}

	for _, tt := range tests {
		t.Run(tt.volID, func(t *testing.T) {
			if got := parseTemplateOS(tt.volID); got != tt.os {
				t.Errorf("parseTemplateOS() = %q, want %q", got, tt.os)
			}
			if got := parseTemplateVersion(tt.volID); got != tt.version {
				t.Errorf("parseTemplateVersion() = %q, want %q", got, tt.version)
			}
			if got := parseTemplateDescription(tt.volID); got != tt.description {
				t.Errorf("parseTemplateDescription() = %q, want %q", got, tt.description)
			}
		})
	}
}
