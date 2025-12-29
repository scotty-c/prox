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
}

// ResolvedTemplate represents a resolved template with its node location
type ResolvedTemplate struct {
	Template string
	Node     string
}
