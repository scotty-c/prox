package container

import (
	"context"
	"fmt"
	"strings"
	"sync"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/util"
)

// ContainerDetails holds all information about a container for JSON output
type ContainerDetails struct {
	Container *Container             `json:"container"`
	Config    map[string]interface{} `json:"config"`
	Status    map[string]interface{} `json:"status"`
}

// GetContainerDetails fetches detailed container information
func GetContainerDetails(nameOrID string) (*ContainerDetails, error) {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}

	// Find the container
	container, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("failed to find container: %w", err)
	}

	// Fetch config and status in parallel
	var (
		config               map[string]interface{}
		status               map[string]interface{}
		configErr, statusErr error
		wg                   sync.WaitGroup
	)

	wg.Add(2)

	// Get container configuration
	go func() {
		defer wg.Done()
		config, configErr = client.GetContainerConfig(ctx, container.Node, container.ID)
	}()

	// Get container status
	go func() {
		defer wg.Done()
		status, statusErr = client.GetContainerStatus(ctx, container.Node, container.ID)
	}()

	wg.Wait()

	// Check for errors
	if configErr != nil {
		return nil, fmt.Errorf("failed to get container config: %w", configErr)
	}
	if statusErr != nil {
		return nil, fmt.Errorf("failed to get container status: %w", statusErr)
	}

	return &ContainerDetails{
		Container: container,
		Config:    config,
		Status:    status,
	}, nil
}

// DescribeContainer shows detailed information about a container
func DescribeContainer(nameOrID string) error {
	details, err := GetContainerDetails(nameOrID)
	if err != nil {
		return err
	}

	output.Info("Getting container details for %s...\n", nameOrID)

	// Display container information
	displayContainerDetails(details.Container, details.Config, details.Status)

	return nil
}

// displayContainerDetails displays detailed container information
func displayContainerDetails(container *Container, config map[string]interface{}, status map[string]interface{}) {
	output.Result("\nContainer Details\n")
	output.Result("═══════════════════════════════════════════════════════════════════════════════════════\n")

	// Basic Information
	output.Result("Basic Information:\n")
	output.Result("   Name: %s\n", container.Name)
	output.Result("   ID: %d\n", container.ID)
	output.Result("   Node: %s\n", container.Node)
	output.Result("   Status: %s\n", container.Status)

	// Template information
	if osTemplate, ok := config["ostemplate"].(string); ok {
		output.Result("   Template: %s\n", osTemplate)
	}

	// Hostname
	if hostname, ok := config["hostname"].(string); ok {
		output.Result("   Hostname: %s\n", hostname)
	}

	output.Result("\n")

	// Resource Configuration
	output.Result("Resource Configuration:\n")
	if memory, ok := config["memory"].(float64); ok {
		output.Result("   Memory: %.0f MB\n", memory)
	}
	if swap, ok := config["swap"].(float64); ok {
		output.Result("   Swap: %.0f MB\n", swap)
	}
	if cores, ok := config["cores"].(float64); ok {
		output.Result("   CPU Cores: %.0f\n", cores)
	}
	if cpuLimit, ok := config["cpulimit"].(float64); ok {
		output.Result("   CPU Limit: %.1f\n", cpuLimit)
	}
	if cpuUnits, ok := config["cpuunits"].(float64); ok {
		output.Result("   CPU Units: %.0f\n", cpuUnits)
	}

	output.Result("\n")

	// Storage Information
	output.Result("Storage:\n")
	if rootfs, ok := config["rootfs"].(string); ok {
		output.Result("   Root Filesystem: %s\n", rootfs)
	}

	// Mount points
	for key, value := range config {
		if strings.HasPrefix(key, "mp") {
			if mountPoint, ok := value.(string); ok {
				output.Result("   Mount Point %s: %s\n", strings.TrimPrefix(key, "mp"), mountPoint)
			}
		}
	}

	output.Result("\n")

	// Network Configuration
	output.Result("Network:\n")
	for key, value := range config {
		if strings.HasPrefix(key, "net") {
			if netConfig, ok := value.(string); ok {
				output.Result("   Network %s: %s\n", strings.TrimPrefix(key, "net"), netConfig)
			}
		}
	}

	output.Result("\n")

	// Runtime Status (if running)
	if container.Status == "running" {
		output.Result("Runtime Status:\n")

		if vmStatus, ok := status["status"].(string); ok {
			output.Result("   VM Status: %s\n", vmStatus)
		}

		if uptime, ok := status["uptime"].(float64); ok {
			output.Result("   Uptime: %s\n", util.FormatUptime(int64(uptime)))
		}

		if cpuUsage, ok := status["cpu"].(float64); ok {
			output.Result("   CPU Usage: %.2f%%\n", cpuUsage*100)
		}

		if memUsage, ok := status["mem"].(float64); ok {
			if memMax, ok := status["maxmem"].(float64); ok {
				memPercent := (memUsage / memMax) * 100
				output.Result("   Memory Usage: %s / %s (%.1f%%)\n",
					util.FormatSize(uint64(memUsage)),
					util.FormatSize(uint64(memMax)),
					memPercent)
			}
		}

		if diskUsage, ok := status["disk"].(float64); ok {
			if diskMax, ok := status["maxdisk"].(float64); ok {
				diskPercent := (diskUsage / diskMax) * 100
				output.Result("   Disk Usage: %s / %s (%.1f%%)\n",
					util.FormatSize(uint64(diskUsage)),
					util.FormatSize(uint64(diskMax)),
					diskPercent)
			}
		}

		if swapUsage, ok := status["swap"].(float64); ok {
			if swapMax, ok := status["maxswap"].(float64); ok {
				swapPercent := (swapUsage / swapMax) * 100
				output.Result("   Swap Usage: %s / %s (%.1f%%)\n",
					util.FormatSize(uint64(swapUsage)),
					util.FormatSize(uint64(swapMax)),
					swapPercent)
			}
		}

		output.Result("\n")
	}

	// Security Settings
	output.Result("Security:\n")
	if unprivileged, ok := config["unprivileged"].(float64); ok {
		if unprivileged == 1 {
			output.Result("   Unprivileged: Yes\n")
		} else {
			output.Result("   Unprivileged: No\n")
		}
	}
	if protection, ok := config["protection"].(float64); ok {
		if protection == 1 {
			output.Result("   Protection: Enabled\n")
		} else {
			output.Result("   Protection: Disabled\n")
		}
	}

	output.Result("\n")

	// Additional Configuration
	output.Result("Additional Configuration:\n")
	if description, ok := config["description"].(string); ok {
		output.Result("   Description: %s\n", description)
	}
	if tags, ok := config["tags"].(string); ok {
		output.Result("   Tags: %s\n", tags)
	}
	if onboot, ok := config["onboot"].(float64); ok {
		if onboot == 1 {
			output.Result("   Start on boot: Yes\n")
		} else {
			output.Result("   Start on boot: No\n")
		}
	}
	if startup, ok := config["startup"].(string); ok {
		output.Result("   Startup order: %s\n", startup)
	}

	output.Result("\n")
}
