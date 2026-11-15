package vm

import (
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/guest"
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

// VM is a virtual machine as shown in lists
type VM = guest.Guest

// NewTask creates a new Task instance
func NewTask(id string) *Task {
	return &Task{
		ID: id,
	}
}
