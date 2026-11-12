package vm

import (
	"net/url"
	"reflect"
	"testing"
)

func TestEncodeSSHKeys(t *testing.T) {
	keys := "ssh-ed25519 AAAAC3Nza+C1/lZD= me@host\nssh-rsa AAAAB3 you@host"
	got := encodeSSHKeys(keys)
	want := "ssh-ed25519%20AAAAC3Nza%2BC1%2FlZD%3D%20me%40host%0Assh-rsa%20AAAAB3%20you%40host"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	// Proxmox decodes it back to the original
	if back, err := url.PathUnescape(got); err != nil || back != keys {
		t.Fatalf("round trip = %q, %v", back, err)
	}
}

func TestCloudInitConfig(t *testing.T) {
	tests := []struct {
		name    string
		opts    CreateOptions
		want    map[string]interface{}
		wantErr bool
	}{
		{"nothing", CreateOptions{}, map[string]interface{}{}, false},
		{"resources and dhcp", CreateOptions{Cores: 2, MemoryMB: 4096, IP: "dhcp"},
			map[string]interface{}{"cores": 2, "memory": 4096, "ipconfig0": "ip=dhcp"}, false},
		{"static with gateway and user", CreateOptions{IP: "10.0.0.20/24", Gateway: "10.0.0.1", CIUser: "admin"},
			map[string]interface{}{"ipconfig0": "ip=10.0.0.20/24,gw=10.0.0.1", "ciuser": "admin"}, false},
		{"keys", CreateOptions{SSHKeys: "ssh-ed25519 AAA me@h\n"},
			map[string]interface{}{"sshkeys": "ssh-ed25519%20AAA%20me%40h"}, false},
		{"ip without prefix", CreateOptions{IP: "10.0.0.20"}, nil, true},
		{"gateway without ip", CreateOptions{Gateway: "10.0.0.1"}, nil, true},
		{"gateway with dhcp", CreateOptions{IP: "dhcp", Gateway: "10.0.0.1"}, nil, true},
	}
	for _, tt := range tests {
		got, err := cloudInitConfig(tt.opts)
		if (err != nil) != tt.wantErr || (!tt.wantErr && !reflect.DeepEqual(got, tt.want)) {
			t.Errorf("%s: got %v, %v; want %v, error %v", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestValidateNewVMName(t *testing.T) {
	for _, ok := range []string{"web", "web-1", "web.example.com", "A1"} {
		if err := ValidateNewVMName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-web", "web-", "has space", "under_score"} {
		if ValidateNewVMName(bad) == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
