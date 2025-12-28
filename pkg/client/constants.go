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

	// Disk usage estimation divisors
	DiskUsageDivisorRunning = 5  // Estimate 20% usage (1/5) for running VMs
	DiskUsageDivisorStopped = 10 // Estimate 10% usage (1/10) for stopped VMs

	// Task polling
	TaskPollIntervalSeconds = 2 // Interval for polling task status

	// Worker pool
	MaxConcurrentIPLookups = 10 // Maximum concurrent IP lookup workers

	// Caching
	ClusterResourcesCacheTTL = 10 // Time-to-live for cluster resources cache (seconds)
	IPCacheTTL               = 60 // Time-to-live for IP address cache (seconds)

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

	// API paths
	APIBasePath          = "/api2/json"
	AuthPath             = "/access/ticket"
	NodesPath            = "/nodes"
	ClusterResourcesPath = "/cluster/resources"
	VersionPath          = "/version"
)

// Common VM statuses
var VMStatuses = map[string]string{
	"running":   "Running",
	"stopped":   "Stopped",
	"paused":    "Paused",
	"suspended": "Suspended",
}

// Common container statuses
var ContainerStatuses = map[string]string{
	"running": "Running",
	"stopped": "Stopped",
	"paused":  "Paused",
}

// Boot order mappings for human-readable display
var BootOrderMappings = map[string]string{
	"c":     "Hard Disk",
	"d":     "CD/DVD",
	"n":     "Network (PXE)",
	"a":     "Floppy",
	"disk":  "Hard Disk",
	"net":   "Network (PXE)",
	"cdrom": "CD/DVD",
}

// Common disk interfaces
var DiskInterfaces = []string{
	"virtio0", "virtio1", "virtio2", "virtio3",
	"scsi0", "scsi1", "scsi2", "scsi3",
	"ide0", "ide1", "ide2", "ide3",
	"sata0", "sata1", "sata2", "sata3",
}

// Container template shortcuts for common distributions
var ContainerTemplateShortcuts = map[string]string{
	"ubuntu:20.04": "ubuntu-20.04",
	"ubuntu:22.04": "ubuntu-22.04",
	"ubuntu:24.04": "ubuntu-24.04",
	"debian:11":    "debian-11",
	"debian:12":    "debian-12",
	"centos:7":     "centos-7",
	"centos:8":     "centos-8",
	"alpine:3.18":  "alpine-3.18",
	"alpine:3.19":  "alpine-3.19",
}
