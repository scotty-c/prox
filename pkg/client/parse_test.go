package client

import "testing"

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

func TestValidateDiskSizeAndParse(t *testing.T) {
	for _, ok := range []string{"10G", "1024M", "1T", "0.5G", "10g"} {
		if err := ValidateDiskSize(ok); err != nil {
			t.Errorf("ValidateDiskSize(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "10", "10K", "abcG", "-5G", "0G"} {
		if err := ValidateDiskSize(bad); err == nil {
			t.Errorf("ValidateDiskSize(%q) should fail", bad)
		}
	}
	if b, err := ParseDiskSizeToBytes("2G"); err != nil || b != 2*1024*1024*1024 {
		t.Errorf("ParseDiskSizeToBytes(2G) = %d, %v", b, err)
	}
}

func TestValidateVMNameAndID(t *testing.T) {
	if err := ValidateVMName("web-1"); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"", "has space", "way-too-long-name-here", "under_score"} {
		if ValidateVMName(bad) == nil {
			t.Errorf("ValidateVMName(%q) should fail", bad)
		}
	}
	if ValidateVMID(99) == nil || ValidateVMID(100) != nil {
		t.Error("ValidateVMID boundary wrong")
	}
}

func TestExtractIPFromInterfaces(t *testing.T) {
	iface := func(name string, ips ...[2]string) map[string]interface{} {
		var list []interface{}
		for _, ip := range ips {
			list = append(list, map[string]interface{}{"ip-address": ip[0], "ip-address-type": ip[1]})
		}
		return map[string]interface{}{"name": name, "ip-addresses": list}
	}

	tests := []struct {
		name string
		in   []interface{}
		want string
	}{
		{"skips loopback, prefers primary", []interface{}{
			iface("lo", [2]string{"127.0.0.1", "ipv4"}),
			iface("docker0", [2]string{"172.17.0.1", "ipv4"}),
			iface("eth0", [2]string{"fe80::1", "ipv6"}, [2]string{"10.0.0.5", "ipv4"}),
		}, "10.0.0.5"},
		{"falls back to any IPv4", []interface{}{
			iface("lo", [2]string{"127.0.0.1", "ipv4"}),
			iface("wlan0", [2]string{"192.168.1.20", "ipv4"}),
		}, "192.168.1.20"},
		{"nothing usable", []interface{}{iface("lo", [2]string{"127.0.0.1", "ipv4"})}, "N/A"},
		{"malformed entries ignored", []interface{}{"junk", map[string]interface{}{"name": "eth0"}}, "N/A"},
	}
	for _, tt := range tests {
		if got := extractIPFromInterfaces(tt.in); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
