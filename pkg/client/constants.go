package client

// Constants and default values for Proxmox operations

const (
	// Default timeout for HTTP requests (seconds)
	DefaultTimeout = 30

	// CPU percentage conversion
	CPUPercentageMultiplier = 100

	// Worker pool
	MaxConcurrentIPLookups = 10 // Maximum concurrent IP lookup workers

	// Caching
	ClusterResourcesCacheTTL = 10 // Time-to-live for cluster resources cache (seconds)
	IPCacheTTL               = 60 // Time-to-live for IP address cache (seconds)

	// Proxmox API boolean value
	ProxmoxBoolTrue = 1
)
