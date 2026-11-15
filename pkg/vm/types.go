package vm

import (
	c "github.com/scotty-c/prox/pkg/client"
)

// VirtualMachine represents a VM instance
type VirtualMachine struct {
	ID     int
	Node   string
	Client c.ProxmoxClientInterface
}

// Task represents a Proxmox task
type Task struct {
	ID     string
	Status string
}

// VM represents a virtual machine for display purposes
type VM struct {
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

	// Machine-friendly forms of the fields above, which are formatted for
	// people (cpus is a percentage, uptime is "3d 0h 0m", ip is "N/A" when
	// unknown). Added alongside so existing consumers keep working.
	CPUPercent    float64  `json:"cpu_percent"`
	CPUCount      int      `json:"cpu_count"`
	UptimeSeconds int64    `json:"uptime_seconds"`
	TagList       []string `json:"tag_list"`
	IPAddress     string   `json:"ip_address,omitempty"` // only when an address is known
}

// NewTask creates a new Task instance
func NewTask(id string) *Task {
	return &Task{
		ID: id,
	}
}
