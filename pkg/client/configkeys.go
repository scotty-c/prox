package client

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	diskKeyPattern = regexp.MustCompile(`^(ide|sata|scsi|virtio)(\d+)$`)
	netKeyPattern  = regexp.MustCompile(`^net(\d+)$`)

	// busOrder is the order disk buses are listed in
	busOrder = map[string]int{"ide": 0, "sata": 1, "scsi": 2, "virtio": 3}
)

// DiskKeys returns the keys of a VM config that hold disks or drives
// (ide0, sata0, scsi0, virtio0, ...), in bus then numeric order. Proxmox
// allows up to scsi30 and virtio15, so keys are matched by pattern rather than
// from a fixed list. efidisk0 and tpmstate0 are not included.
func DiskKeys(config map[string]interface{}) []string {
	type key struct {
		name string
		bus  int
		n    int
	}
	var keys []key
	for name := range config {
		if m := diskKeyPattern.FindStringSubmatch(name); m != nil {
			n, _ := strconv.Atoi(m[2])
			keys = append(keys, key{name, busOrder[m[1]], n})
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bus != keys[j].bus {
			return keys[i].bus < keys[j].bus
		}
		return keys[i].n < keys[j].n
	})
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = k.name
	}
	return names
}

// NetKeys returns the network interface keys of a VM or container config
// (net0, net1, ...) in numeric order
func NetKeys(config map[string]interface{}) []string {
	type key struct {
		name string
		n    int
	}
	var keys []key
	for name := range config {
		if m := netKeyPattern.FindStringSubmatch(name); m != nil {
			n, _ := strconv.Atoi(m[1])
			keys = append(keys, key{name, n})
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].n < keys[j].n })
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = k.name
	}
	return names
}

// ConfigNumber reads a numeric value from a Proxmox config map. Proxmox
// returns some numbers as JSON numbers and others as strings ("2048"), so
// both are accepted.
func ConfigNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

// ConfigFlag reads an on/off setting from a Proxmox config map. It accepts
// 0/1 as a number or string, true/false, and the property-string form used by
// options such as agent ("1", "enabled=1,fstrim_cloned_disks=1").
func ConfigFlag(v interface{}) (on, ok bool) {
	switch x := v.(type) {
	case float64:
		return x != 0, true
	case bool:
		return x, true
	case string:
		for i, part := range strings.Split(x, ",") {
			part = strings.TrimSpace(part)
			if value, found := strings.CutPrefix(part, "enabled="); found {
				return parseFlag(value)
			}
			if i == 0 {
				if on, ok := parseFlag(part); ok {
					return on, true
				}
			}
		}
	}
	return false, false
}

func parseFlag(s string) (on, ok bool) {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}
