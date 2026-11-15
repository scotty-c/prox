package config

import (
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
		wantNote bool
	}{
		{"https://pve.example.com:8006", "https://pve.example.com:8006", false},
		{"pve.example.com:8006", "https://pve.example.com:8006", false},
		{"  https://pve:8006/  ", "https://pve:8006", false},
		{"https://pve:8006/api2/json", "https://pve:8006", false},
		{"https://pve:8006/api2/json/", "https://pve:8006", false},
		{"https://pve:8006?x=1#frag", "https://pve:8006", false},
		{"http://192.168.1.10:8006", "http://192.168.1.10:8006", false},
		{"https://[fd00::1]:8006", "https://[fd00::1]:8006", false},
		{"https://proxy.example.com/proxmox/", "https://proxy.example.com/proxmox", true},
		{"https://proxy.example.com/proxmox/api2/json", "https://proxy.example.com/proxmox", true},
		{"pve", "https://pve", true},
		{"https://pve.example.com", "https://pve.example.com", true},
	}
	for _, tt := range tests {
		got, note, err := NormalizeURL(tt.in)
		if err != nil || got != tt.want || (note != "") != tt.wantNote {
			t.Errorf("NormalizeURL(%q) = %q, note %q, err %v; want %q, note=%v", tt.in, got, note, err, tt.want, tt.wantNote)
		}
	}
}

func TestNormalizeURLRejectsWhatCannotWork(t *testing.T) {
	for in, wantErr := range map[string]string{
		"":                    "empty",
		"   ":                 "empty",
		"ftp://pve:8006":      "must start with https://",
		"ssh://root@pve":      "must start with https://",
		"https://":            "not a valid URL",
		"https://:8006":       "not a valid URL",
		"http://exa mple.com": "not a valid URL",
	} {
		_, _, err := NormalizeURL(in)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("NormalizeURL(%q) error = %v, want one containing %q", in, err, wantErr)
		}
	}
}

func TestNormalizeURLNoteMentionsThePortOnlyWhenMissing(t *testing.T) {
	_, note, _ := NormalizeURL("https://pve.example.com")
	if !strings.Contains(note, "8006") || !strings.Contains(note, "reverse proxy") {
		t.Errorf("note = %q", note)
	}
}
