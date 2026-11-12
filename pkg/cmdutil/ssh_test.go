package cmdutil

import (
	"reflect"
	"testing"
)

func TestNodeSSHArgs(t *testing.T) {
	got := NodeSSHArgs("10.0.0.2", "root", 22, "pct", "enter", "101")
	want := []string{"-t", "root@10.0.0.2", "pct", "enter", "101"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	got = NodeSSHArgs("pve1", "", 2222, "qm", "terminal", "100")
	want = []string{"-t", "-p", "2222", "pve1", "qm", "terminal", "100"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
