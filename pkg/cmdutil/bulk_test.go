package cmdutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNameOrTagArgs(t *testing.T) {
	tests := []struct {
		flags   []string
		args    []string
		wantErr bool
	}{
		{nil, []string{"web"}, false},
		{nil, nil, true},
		{nil, []string{"a", "b"}, true},
		{[]string{"--tag", "web"}, nil, false},
		{[]string{"--tag", "web"}, []string{"x"}, true},
	}
	for _, tt := range tests {
		cmd := &cobra.Command{Use: "x"}
		AddTagFlags(cmd, "VM")
		if err := cmd.ParseFlags(tt.flags); err != nil {
			t.Fatal(err)
		}
		if err := NameOrTagArgs(cmd, tt.args); (err != nil) != tt.wantErr {
			t.Errorf("flags %v args %v: err = %v, wantErr %v", tt.flags, tt.args, err, tt.wantErr)
		}
	}
}

func TestRunBulkContinuesAfterFailure(t *testing.T) {
	var ran []string
	target := func(name string, err error) BulkTarget {
		return BulkTarget{Label: name, Run: func() error { ran = append(ran, name); return err }}
	}
	err := RunBulk("start", "VM", "web", []BulkTarget{
		target("a", nil), target("b", errors.New("boom")), target("c", nil),
	}, true)
	if err == nil || !strings.Contains(err.Error(), "1 of 3 VM(s) failed to start") {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(ran, ",") != "a,b,c" {
		t.Fatalf("ran = %v", ran)
	}

	if err := RunBulk("start", "VM", "none", nil, true); err == nil || !strings.Contains(err.Error(), "no VMs have the tag 'none'") {
		t.Fatalf("empty: err = %v", err)
	}
}
