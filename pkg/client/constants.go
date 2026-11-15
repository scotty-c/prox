package client

// Constants and default values for Proxmox operations

const (
	// Default timeout for HTTP requests (seconds)
	DefaultTimeout = 30

	// CPU percentage conversion
	CPUPercentageMultiplier = 100

	// Worker pool
	MaxConcurrentLookups = 10 // Maximum concurrent per-guest lookups (IP addresses, disk details)

	// Caching
	ClusterResourcesCacheTTL = 10 // Time-to-live for cluster resources cache (seconds)

	// Proxmox API boolean value
	ProxmoxBoolTrue = 1
)
