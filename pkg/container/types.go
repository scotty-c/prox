package container

// Template represents a container template
type Template struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OS          string `json:"os"`
	Version     string `json:"version"`
	Size        string `json:"size"`
	SizeBytes   uint64 `json:"size_bytes"`
	Node        string `json:"node"`
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

	// Machine-friendly forms of the fields above (see vm.VM)
	CPUPercent    float64  `json:"cpu_percent"`
	CPUCount      int      `json:"cpu_count"`
	UptimeSeconds int64    `json:"uptime_seconds"`
	TagList       []string `json:"tag_list"`
	IPAddress     string   `json:"ip_address,omitempty"` // only when an address is known
}

// ResolvedTemplate represents a resolved template with its node location
type ResolvedTemplate struct {
	Template string
	Node     string
}
