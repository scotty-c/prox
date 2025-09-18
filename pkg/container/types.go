package container

// Template represents a container template
type Template struct {
	Name        string
	Description string
	OS          string
	Version     string
	Size        string
	Node        string
}

// Container represents an LXC container
type Container struct {
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

// ResolvedTemplate represents a resolved template with its node location
type ResolvedTemplate struct {
	Template string
	Node     string
}
