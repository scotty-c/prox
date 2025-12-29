// Package node provides functionality for managing Proxmox VE nodes, including
// querying node information, formatting node data for display, and node-level operations.
package node

import (
	"fmt"
)

// formatSize formats bytes into human-readable size (same style as vm.formatSize)
func formatSize(sizeBytes uint64) string {
	const unit = 1024
	if sizeBytes < unit {
		return fmt.Sprintf("%d B", sizeBytes)
	}
	div, exp := uint64(unit), 0
	for n := sizeBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(sizeBytes)/float64(div), "KMGTPE"[exp])
}

// formatUptime formats uptime seconds into human-readable format (same as vm.formatUptime)
func formatUptime(uptimeSeconds int64) string {
	if uptimeSeconds <= 0 {
		return "0s"
	}

	days := uptimeSeconds / 86400
	hours := (uptimeSeconds % 86400) / 3600
	minutes := (uptimeSeconds % 3600) / 60
	seconds := uptimeSeconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}
