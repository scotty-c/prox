package client

import (
	"encoding/json"
	"testing"
)

func TestParseDiskSize(t *testing.T) {
	tests := map[string]uint64{
		"":      0,
		"512K":  512 * 1024,
		"1024M": 1024 * 1024 * 1024,
		"32G":   32 * 1024 * 1024 * 1024,
		"1.5T":  uint64(1.5 * 1024 * 1024 * 1024 * 1024),
		"32g":   32 * 1024 * 1024 * 1024,
		"bad":   0,
	}
	for in, want := range tests {
		if got := parseDiskSize(in); got != want {
			t.Errorf("parseDiskSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestExtractIPFromInterfaces(t *testing.T) {
	// Decode from JSON, as the guest agent reports them
	parse := func(js string) []agentInterface {
		var out []agentInterface
		if err := json.Unmarshal([]byte(js), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"skips loopback, prefers primary", `[
			{"name":"lo","ip-addresses":[{"ip-address":"127.0.0.1","ip-address-type":"ipv4"}]},
			{"name":"docker0","ip-addresses":[{"ip-address":"172.17.0.1","ip-address-type":"ipv4"}]},
			{"name":"eth0","ip-addresses":[{"ip-address":"fe80::1","ip-address-type":"ipv6"},{"ip-address":"10.0.0.5","ip-address-type":"ipv4"}]}]`, "10.0.0.5"},
		{"falls back to any IPv4", `[
			{"name":"lo","ip-addresses":[{"ip-address":"127.0.0.1","ip-address-type":"ipv4"}]},
			{"name":"wlan0","ip-addresses":[{"ip-address":"192.168.1.20","ip-address-type":"ipv4"}]}]`, "192.168.1.20"},
		{"nothing usable", `[{"name":"lo","ip-addresses":[{"ip-address":"127.0.0.1","ip-address-type":"ipv4"}]}]`, ""},
		{"IPv6 only", `[{"name":"eth0","ip-addresses":[{"ip-address":"fe80::1","ip-address-type":"ipv6"}]}]`, ""},
		{"interface without addresses", `[{"name":"eth0"}]`, ""},
		{"none at all", `[]`, ""},
	}
	for _, tt := range tests {
		if got := extractIPFromInterfaces(parse(tt.in)); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestIPFromCloudInitConfig(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   string
	}{
		{"static", map[string]interface{}{"ipconfig0": "ip=192.168.1.100/24,gw=192.168.1.1"}, "192.168.1.100"},
		{"dhcp then static", map[string]interface{}{"ipconfig0": "ip=dhcp", "ipconfig1": "gw=10.0.0.1,ip=10.0.0.5/16"}, "10.0.0.5"},
		{"ipv6 only", map[string]interface{}{"ipconfig0": "ip6=auto"}, ""},
		{"net line is not cloud-init", map[string]interface{}{"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"}, ""},
		{"empty", map[string]interface{}{}, ""},
	}
	for _, tt := range tests {
		if got := ipFromCloudInitConfig(tt.config); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
