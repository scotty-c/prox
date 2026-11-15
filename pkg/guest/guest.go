// Package guest is the row shown for a VM or a container in lists. Proxmox
// reports both the same way, so one type and one set of formatters serve
// both (vm.VM and container.Container are aliases of it).
package guest

import (
	"context"
	"fmt"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/util"
)

// Guest is a VM or container as listed. The first group is formatted for
// people (cpus is a percentage, uptime reads "3d 0h 0m", ip is "N/A" when
// unknown); the second is the same information for scripts. Both are printed
// with --output json so that existing consumers keep working.
type Guest struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Node      string `json:"node"`
	CPUs      int    `json:"cpus"`
	Memory    uint64 `json:"memory"`
	MaxMemory uint64 `json:"max_memory"`
	Disk      uint64 `json:"disk"`
	MaxDisk   uint64 `json:"max_disk"`
	Uptime    string `json:"uptime"`
	IP        string `json:"ip"`
	Tags      string `json:"tags,omitempty"` // Semicolon-separated tags

	CPUPercent    float64  `json:"cpu_percent"`
	CPUCount      int      `json:"cpu_count"`
	UptimeSeconds int64    `json:"uptime_seconds"`
	TagList       []string `json:"tag_list"`
	IPAddress     string   `json:"ip_address,omitempty"` // only when an address is known
}

// FromResource converts a cluster resource (which must have a VMID) into a row
func FromResource(r c.Resource) Guest {
	g := Guest{
		ID:      *r.VMID,
		Name:    r.Name,
		Status:  r.Status,
		Node:    r.Node,
		Tags:    r.Tags,
		TagList: util.SplitTags(r.Tags),
		IP:      "N/A", // until looked up (and only on request)
	}
	if r.MaxMem != nil {
		g.MaxMemory = uint64(*r.MaxMem)
	}
	if r.Mem != nil {
		g.Memory = uint64(*r.Mem)
	}
	if r.MaxDisk != nil {
		g.MaxDisk = uint64(*r.MaxDisk)
	}
	if r.Disk != nil {
		g.Disk = uint64(*r.Disk)
	}
	if r.CPU != nil {
		g.CPUPercent = *r.CPU * c.CPUPercentageMultiplier
		g.CPUs = int(g.CPUPercent)
	}
	if r.MaxCPU != nil {
		g.CPUCount = *r.MaxCPU
	}
	if r.Uptime != nil {
		g.Uptime = util.FormatUptime(*r.Uptime)
		g.UptimeSeconds = *r.Uptime
	}
	return g
}

// MemoryText is memory use for a table cell: "1.0 GB/2.0 GB (50.0%)"
func (g Guest) MemoryText() string {
	if g.MaxMemory == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%s/%s (%.1f%%)", util.FormatSize(g.Memory), util.FormatSize(g.MaxMemory), float64(g.Memory)/float64(g.MaxMemory)*100)
}

// CPUText is CPU use for a table cell: "12%"
func (g Guest) CPUText() string {
	return fmt.Sprintf("%d%%", g.CPUs)
}

// UptimeText is uptime for a table cell, "N/A" if there is none
func (g Guest) UptimeText() string {
	if g.Uptime == "" {
		return "N/A"
	}
	return g.Uptime
}

// LookupIPs looks up the address of every running guest, several at a time,
// with lookup, and stores it in the row. A guest without a known address
// keeps "N/A" in IP and has no IPAddress.
func LookupIPs(ctx context.Context, guests []Guest, lookup func(ctx context.Context, node string, id int) (string, error)) {
	var running []int
	for i := range guests {
		if guests[i].Status == "running" {
			running = append(running, i)
		}
	}
	util.ForEach(len(running), c.MaxConcurrentIPLookups, func(k int) {
		g := &guests[running[k]]
		if ip, err := lookup(ctx, g.Node, g.ID); err == nil && ip != "" {
			g.IP, g.IPAddress = ip, ip
		}
	})
}
