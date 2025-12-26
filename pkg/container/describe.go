package container

import (
	"context"
	"fmt"
	"strings"

	c "github.com/scotty-c/prox/pkg/client"
)

// DescribeContainer shows detailed information about a container
func DescribeContainer(nameOrID string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	fmt.Printf("Getting container details for %s...\n", nameOrID)

	// Find the container
	container, err := findContainer(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	// Get container configuration
	config, err := client.GetContainerConfig(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to get container config: %w", err)
	}

	// Get container status
	status, err := client.GetContainerStatus(context.Background(), container.Node, container.ID)
	if err != nil {
		return fmt.Errorf("failed to get container status: %w", err)
	}

	// Display container information
	displayContainerDetails(container, config, status)

	return nil
}

// displayContainerDetails displays detailed container information
func displayContainerDetails(container *Container, config map[string]interface{}, status map[string]interface{}) {
	fmt.Printf("\n📦 Container Details\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════════════════════════════\n")

	// Basic Information
	fmt.Printf("Basic Information:\n")
	fmt.Printf("   Name: %s\n", container.Name)
	fmt.Printf("   ID: %d\n", container.ID)
	fmt.Printf("   Node: %s\n", container.Node)
	fmt.Printf("   Status: %s\n", container.Status)

	// Template information
	if osTemplate, ok := config["ostemplate"].(string); ok {
		fmt.Printf("   Template: %s\n", osTemplate)
	}

	// Hostname
	if hostname, ok := config["hostname"].(string); ok {
		fmt.Printf("   Hostname: %s\n", hostname)
	}

	fmt.Printf("\n")

	// Resource Configuration
	fmt.Printf("Resource Configuration:\n")
	if memory, ok := config["memory"].(float64); ok {
		fmt.Printf("   Memory: %.0f MB\n", memory)
	}
	if swap, ok := config["swap"].(float64); ok {
		fmt.Printf("   Swap: %.0f MB\n", swap)
	}
	if cores, ok := config["cores"].(float64); ok {
		fmt.Printf("   CPU Cores: %.0f\n", cores)
	}
	if cpuLimit, ok := config["cpulimit"].(float64); ok {
		fmt.Printf("   CPU Limit: %.1f\n", cpuLimit)
	}
	if cpuUnits, ok := config["cpuunits"].(float64); ok {
		fmt.Printf("   CPU Units: %.0f\n", cpuUnits)
	}

	fmt.Printf("\n")

	// Storage Information
	fmt.Printf("💽 Storage:\n")
	if rootfs, ok := config["rootfs"].(string); ok {
		fmt.Printf("   Root Filesystem: %s\n", rootfs)
	}

	// Mount points
	for key, value := range config {
		if strings.HasPrefix(key, "mp") {
			if mountPoint, ok := value.(string); ok {
				fmt.Printf("   Mount Point %s: %s\n", strings.TrimPrefix(key, "mp"), mountPoint)
			}
		}
	}

	fmt.Printf("\n")

	// Network Configuration
	fmt.Printf("🌐 Network:\n")
	for key, value := range config {
		if strings.HasPrefix(key, "net") {
			if netConfig, ok := value.(string); ok {
				fmt.Printf("   Network %s: %s\n", strings.TrimPrefix(key, "net"), netConfig)
			}
		}
	}

	fmt.Printf("\n")

	// Runtime Status (if running)
	if container.Status == "running" {
		fmt.Printf("📊 Runtime Status:\n")

		if vmStatus, ok := status["status"].(string); ok {
			fmt.Printf("   VM Status: %s\n", vmStatus)
		}

		if uptime, ok := status["uptime"].(float64); ok {
			fmt.Printf("   Uptime: %s\n", formatUptime(int64(uptime)))
		}

		if cpuUsage, ok := status["cpu"].(float64); ok {
			fmt.Printf("   CPU Usage: %.2f%%\n", cpuUsage*100)
		}

		if memUsage, ok := status["mem"].(float64); ok {
			if memMax, ok := status["maxmem"].(float64); ok {
				memPercent := (memUsage / memMax) * 100
				fmt.Printf("   Memory Usage: %s / %s (%.1f%%)\n",
					formatSize(uint64(memUsage)),
					formatSize(uint64(memMax)),
					memPercent)
			}
		}

		if diskUsage, ok := status["disk"].(float64); ok {
			if diskMax, ok := status["maxdisk"].(float64); ok {
				diskPercent := (diskUsage / diskMax) * 100
				fmt.Printf("   Disk Usage: %s / %s (%.1f%%)\n",
					formatSize(uint64(diskUsage)),
					formatSize(uint64(diskMax)),
					diskPercent)
			}
		}

		if swapUsage, ok := status["swap"].(float64); ok {
			if swapMax, ok := status["maxswap"].(float64); ok {
				swapPercent := (swapUsage / swapMax) * 100
				fmt.Printf("   Swap Usage: %s / %s (%.1f%%)\n",
					formatSize(uint64(swapUsage)),
					formatSize(uint64(swapMax)),
					swapPercent)
			}
		}

		fmt.Printf("\n")
	}

	// Security Settings
	fmt.Printf("🔒 Security:\n")
	if unprivileged, ok := config["unprivileged"].(float64); ok {
		if unprivileged == 1 {
			fmt.Printf("   Unprivileged: Yes\n")
		} else {
			fmt.Printf("   Unprivileged: No\n")
		}
	}
	if protection, ok := config["protection"].(float64); ok {
		if protection == 1 {
			fmt.Printf("   Protection: Enabled\n")
		} else {
			fmt.Printf("   Protection: Disabled\n")
		}
	}

	fmt.Printf("\n")

	// Additional Configuration
	fmt.Printf("Additional Configuration:\n")
	if description, ok := config["description"].(string); ok {
		fmt.Printf("   Description: %s\n", description)
	}
	if tags, ok := config["tags"].(string); ok {
		fmt.Printf("   Tags: %s\n", tags)
	}
	if onboot, ok := config["onboot"].(float64); ok {
		if onboot == 1 {
			fmt.Printf("   Start on boot: Yes\n")
		} else {
			fmt.Printf("   Start on boot: No\n")
		}
	}
	if startup, ok := config["startup"].(string); ok {
		fmt.Printf("   Startup order: %s\n", startup)
	}

	fmt.Printf("\n")
}
