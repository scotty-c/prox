package container

import "github.com/scotty-c/prox/pkg/guest"

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

// Container is an LXC container as shown in lists
type Container = guest.Guest

// ResolvedTemplate represents a resolved template with its node location
type ResolvedTemplate struct {
	Template string
	Node     string
}
