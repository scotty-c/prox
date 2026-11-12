package container

import "testing"

func tmpl(volID, node string) Template {
	return Template{Name: volID, OS: parseTemplateOS(volID), Version: parseTemplateVersion(volID), Node: node}
}

func TestResolveTemplateFrom(t *testing.T) {
	const deb = "nfs:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst"
	const ubu = "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
	// debian template on shared storage (visible from both nodes); ubuntu only on pve1
	all := []Template{tmpl(deb, "pve1"), tmpl(ubu, "pve1"), tmpl(deb, "pve2")}

	tests := []struct {
		name, short, preferred string
		wantTemplate, wantNode string
	}{
		{"short, no preference", "debian:12", "", deb, "pve1"},
		{"short, preferred node has it", "debian:12", "pve2", deb, "pve2"},
		{"short, preferred node lacks it falls back", "ubuntu:22.04", "pve2", ubu, "pve1"},
		{"full, preferred node has it", deb, "pve2", deb, "pve2"},
		{"full, no preference", deb, "", deb, "pve1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTemplateFrom(all, tt.short, tt.preferred)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Template != tt.wantTemplate || got.Node != tt.wantNode {
				t.Fatalf("got %s on %s, want %s on %s", got.Template, got.Node, tt.wantTemplate, tt.wantNode)
			}
		})
	}

	if _, err := resolveTemplateFrom(all, "alpine:3.20", ""); err == nil {
		t.Fatal("expected error for missing template")
	}
}
