package client

import (
	"regexp"
	"sort"
	"strconv"
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
