package container

import "testing"

func TestBuildNet0(t *testing.T) {
	tests := []struct {
		name    string
		opts    NetworkOptions
		want    string
		wantErr bool
	}{
		{"defaults match earlier versions", NetworkOptions{}, "name=eth0,bridge=vmbr0,ip=dhcp", false},
		{"bridge and vlan", NetworkOptions{Bridge: "vmbr1", VLAN: 20}, "name=eth0,bridge=vmbr1,ip=dhcp,tag=20", false},
		{"static", NetworkOptions{IP: "10.0.0.5/24", Gateway: "10.0.0.1"}, "name=eth0,bridge=vmbr0,ip=10.0.0.5/24,gw=10.0.0.1", false},
		{"static without gateway", NetworkOptions{IP: "10.0.0.5/24"}, "name=eth0,bridge=vmbr0,ip=10.0.0.5/24", false},
		{"ip without prefix", NetworkOptions{IP: "10.0.0.5"}, "", true},
		{"gateway with dhcp", NetworkOptions{Gateway: "10.0.0.1"}, "", true},
		{"bad gateway", NetworkOptions{IP: "10.0.0.5/24", Gateway: "nope"}, "", true},
		{"vlan out of range", NetworkOptions{VLAN: 5000}, "", true},
		{"bridge injection", NetworkOptions{Bridge: "vmbr0,firewall=1"}, "", true},
	}
	for _, tt := range tests {
		got, err := buildNet0(tt.opts)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: buildNet0 = %q, %v; want %q, error %v", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestPickRootfsStorage(t *testing.T) {
	tests := []struct {
		available []string
		want      string
	}{
		{nil, "local-lvm"},
		{[]string{"local-zfs", "local-lvm"}, "local-lvm"},
		{[]string{"local-zfs", "ceph"}, "local-zfs"},
	}
	for _, tt := range tests {
		if got := pickRootfsStorage(tt.available); got != tt.want {
			t.Errorf("pickRootfsStorage(%v) = %q, want %q", tt.available, got, tt.want)
		}
	}
}
