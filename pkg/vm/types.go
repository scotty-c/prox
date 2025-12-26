package vm

import (
	c "github.com/scotty-c/prox/pkg/client"
)

// VirtualMachine represents a VM instance
type VirtualMachine struct {
	ID     int
	Node   string
	Client *c.ProxmoxClient
}

// Task represents a Proxmox task
type Task struct {
	ID     string
	Status string
}

// VirtualMachineOption represents configuration options for VMs
type VirtualMachineOption struct {
	Name  string
	Value interface{}
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
}

// NewTask creates a new Task instance
func NewTask(id string) *Task {
	return &Task{
		ID: id,
	}
}
