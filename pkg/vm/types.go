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
	ID        int
	Name      string
	Status    string
	Node      string
	CPUs      int
	Memory    uint64
	MaxMemory uint64
	Disk      uint64
	MaxDisk   uint64
	Uptime    string
	IP        string
}

// NewTask creates a new Task instance
func NewTask(id string) *Task {
	return &Task{
		ID: id,
	}
}
