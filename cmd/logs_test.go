package cmd

import "testing"

func TestParseUPIDNode(t *testing.T) {
	node, err := parseUPIDNode("UPID:pve-node1:00012345:001A2B3C:65A1B2C3:vzcreate:100:root@pam:")
	if err != nil || node != "pve-node1" {
		t.Fatalf("got %q, %v", node, err)
	}
	for _, bad := range []string{"", "UPID", "UPID::x"} {
		if _, err := parseUPIDNode(bad); err == nil {
			t.Errorf("parseUPIDNode(%q) should fail", bad)
		}
	}
}
