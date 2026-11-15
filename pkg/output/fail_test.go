package output

import (
	"errors"
	"fmt"
	"testing"

	"github.com/scotty-c/prox/pkg/apierr"
)

func TestTipForMatchesTheKindOfError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string // substring of the tip; "" means no tip
	}{
		{"unreachable", fmt.Errorf("failed to find VM: %w at https://x: boom", apierr.ErrUnreachable), "Check the URL"},
		{"auth", fmt.Errorf("x: %w", apierr.ErrAuth), "Check your credentials"},
		{"permission", fmt.Errorf("x: %w", apierr.ErrPermission), "lacks the privileges"},
		{"no config", fmt.Errorf("%w: profile 'default' does not exist", apierr.ErrNoConfig), "prox config setup"},
		{"certificate explains itself", fmt.Errorf("x: %w", apierr.ErrCertificate), ""},
		{"unknown", errors.New("something else"), ""},
		// The words in the text must not matter, only the error's kind
		{"text alone is not enough", errors.New("connection refused: authentication failed"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tipFor(tt.err)
			if tt.want == "" && got != "" || tt.want != "" && !contains(got, tt.want) {
				t.Fatalf("tipFor = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
