package client

// Constants and default values for Proxmox operations

const (
	// Default timeout for HTTP requests (seconds)
	DefaultTimeout = 30

	// VM ID ranges
	MinVMID = 100
	MaxVMID = 999999999

	// Memory limits (MB)
	MinMemoryMB = 16
	MaxMemoryMB = 4194304 // 4TB

	// CPU limits
	MinCPUCores = 1
	MaxCPUCores = 128

	// CPU percentage conversion
	CPUPercentageMultiplier = 100

	// Worker pool
	MaxConcurrentIPLookups = 10 // Maximum concurrent IP lookup workers

	// Caching
	ClusterResourcesCacheTTL = 10 // Time-to-live for cluster resources cache (seconds)
	IPCacheTTL               = 60 // Time-to-live for IP address cache (seconds)

	// Proxmox API boolean value
	ProxmoxBoolTrue = 1

	// VM name constraints
	MaxVMNameLength = 15

	// Default VM settings
	DefaultVMMemory = 1024 // 1GB in MB
	DefaultVMCores  = 1
	DefaultVMDisk   = "10G"

	// Default container settings
	DefaultContainerMemory = 512 // 512MB
	DefaultContainerCores  = 1
	DefaultContainerDisk   = "8G"
)
