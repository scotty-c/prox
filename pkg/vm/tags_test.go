package vm

import "testing"

func TestHasTag(t *testing.T) {
	tests := []struct {
		tags, tag string
		want      bool
	}{
		{"", "web", false},
		{"web", "web", true},
		{"prod;web;db", "web", true},
		{"prod; web", "web", true},
		{"webserver", "web", false},
	}
	for _, tt := range tests {
		if got := hasTag(tt.tags, tt.tag); got != tt.want {
			t.Errorf("hasTag(%q, %q) = %v, want %v", tt.tags, tt.tag, got, tt.want)
		}
	}
}
