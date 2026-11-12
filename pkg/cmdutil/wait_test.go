package cmdutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveWait(t *testing.T) {
	tests := []struct {
		args        []string
		defaultWait bool
		want        bool
	}{
		{nil, true, true},
		{nil, false, false},
		{[]string{"--wait"}, false, true},
		{[]string{"-w"}, false, true},
		{[]string{"--no-wait"}, true, false},
		{[]string{"--wait=false"}, true, false},
		{[]string{"--wait", "--no-wait"}, false, true},
	}
	for _, tt := range tests {
		cmd := &cobra.Command{Use: "x", Run: func(*cobra.Command, []string) {}}
		AddWaitFlags(cmd, tt.defaultWait)
		if err := cmd.ParseFlags(tt.args); err != nil {
			t.Fatal(err)
		}
		if got := ResolveWait(cmd, tt.defaultWait); got != tt.want {
			t.Errorf("args %v default %v: got %v, want %v", tt.args, tt.defaultWait, got, tt.want)
		}
	}
}
