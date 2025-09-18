package client

// This file contains extended type definitions for the Proxmox client
// These types are shared across all client modules and complement the base types

// ClientConfig represents the configuration for a Proxmox client
type ClientConfig struct {
	BaseURL            string
	Username           string
	Password           string
	InsecureSkipVerify bool
	Timeout            int // seconds
}

// ConnectionInfo represents information about a Proxmox connection
type ConnectionInfo struct {
	Connected     bool
	Version       *Version
	Authenticated bool
	Nodes         []Node
}

// VMOperation represents a VM operation result
type VMOperation struct {
	TaskID  string
	VMID    int
	Node    string
	Success bool
	Message string
}

// ContainerOperation represents a container operation result
type ContainerOperation struct {
	TaskID      string
	ContainerID int
	Node        string
	Success     bool
	Message     string
}

// NetworkInterface represents a network interface
type NetworkInterface struct {
	Name       string                 `json:"name"`
	IPAddress  string                 `json:"ip-address,omitempty"`
	MacAddress string                 `json:"mac-address,omitempty"`
	Statistics map[string]interface{} `json:"statistics,omitempty"`
}

// DiskInfo represents disk information
type DiskInfo struct {
	Device    string `json:"device"`
	Size      uint64 `json:"size"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
	Path      string `json:"path,omitempty"`
}

// ClusterStatus represents the overall cluster status
type ClusterStatus struct {
	Nodes     []Node     `json:"nodes"`
	Resources []Resource `json:"resources"`
	Version   *Version   `json:"version,omitempty"`
}
