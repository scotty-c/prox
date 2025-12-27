package vm

import (
	"context"
	"fmt"
	"strings"

	c "github.com/scotty-c/prox/pkg/client"
)

// DescribeVM displays detailed VM information in a modern, sectioned format
func DescribeVM(nameOrID string, node string) error {
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	fmt.Printf("Getting VM details for %s...\n", nameOrID)

	// Find the VM by name or ID
	vm, err := FindByNameOrID(client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find VM: %w", err)
	}

	// Use the discovered node if no node was specified
	if node == "" {
		node = vm.Node
	}

	// Get VM configuration
	config, err := client.GetVMConfig(context.Background(), node, vm.ID)
	if err != nil {
		return fmt.Errorf("failed to get VM config: %w", err)
	}

	// Get VM status
	status, err := client.GetVMStatus(context.Background(), node, vm.ID)
	if err != nil {
		return fmt.Errorf("failed to get VM status: %w", err)
	}

	// Use the VM name from the found VM
	vmName := vm.Name

	// Display VM information
	displayVMDetails(vm.ID, vmName, node, config, status)

	return nil
}

// displayVMDetails displays detailed VM information
func displayVMDetails(id int, name, node string, config, status map[string]interface{}) {
	fmt.Printf("\nVirtual Machine Details\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════════════════════════════\n")

	// Basic Information
	fmt.Printf("Basic Information:\n")
	fmt.Printf("   Name: %s\n", name)
	fmt.Printf("   ID: %d\n", id)
	fmt.Printf("   Node: %s\n", node)

	// VM status
	if vmStatus, ok := status["status"].(string); ok {
		fmt.Printf("   Status: %s\n", vmStatus)
	}

	// VM agent status
	if agent, ok := config["agent"].(float64); ok && agent == 1 {
		fmt.Printf("   QEMU Agent: Enabled\n")
	} else {
		fmt.Printf("   QEMU Agent: Disabled\n")
	}

	fmt.Printf("\n")

	// Resource Configuration
	fmt.Printf("Resource Configuration:\n")

	// Memory
	if memory, ok := config["memory"].(float64); ok {
		fmt.Printf("   Memory: %.0f MB\n", memory)
	}

	// Balloon memory
	if balloon, ok := config["balloon"].(float64); ok {
		fmt.Printf("   Balloon Memory: %.0f MB\n", balloon)
	}

	// CPU
	if sockets, ok := config["sockets"].(float64); ok {
		fmt.Printf("   CPU Sockets: %.0f\n", sockets)
	}
	if cores, ok := config["cores"].(float64); ok {
		fmt.Printf("   CPU Cores: %.0f\n", cores)
	}
	if vcpus, ok := config["vcpus"].(float64); ok {
		fmt.Printf("   vCPUs: %.0f\n", vcpus)
	}
	if cpuType, ok := config["cpu"].(string); ok {
		fmt.Printf("   CPU Type: %s\n", cpuType)
	}

	fmt.Printf("\n")

	// Storage Information
	fmt.Printf("💽 Storage:\n")

	// Display disks
	diskKeys := []string{"ide0", "ide1", "ide2", "ide3", "sata0", "sata1", "sata2", "sata3", "scsi0", "scsi1", "scsi2", "scsi3", "virtio0", "virtio1", "virtio2", "virtio3"}
	for _, diskKey := range diskKeys {
		if disk, ok := config[diskKey].(string); ok {
			fmt.Printf("   %s: %s\n", strings.ToUpper(diskKey), disk)
		}
	}

	fmt.Printf("\n")

	// Network Configuration
	fmt.Printf("🌐 Network:\n")

	// Display network interfaces
	for i := 0; i < 10; i++ {
		netKey := fmt.Sprintf("net%d", i)
		if net, ok := config[netKey].(string); ok {
			fmt.Printf("   Network %d: %s\n", i, net)
		}
	}

	// Get VM IP if available
	if vmIP := GetIp(id, node); vmIP != "" && vmIP != "Error getting IP" {
		fmt.Printf("   IP Address: %s\n", vmIP)
	}

	fmt.Printf("\n")

	// Runtime Status (if running)
	if vmStatus, ok := status["status"].(string); ok && vmStatus == "running" {
		fmt.Printf("📊 Runtime Status:\n")

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

		// Try to get disk usage info, first from status, then from improved method
		diskUsage, diskUsageOk := status["disk"].(float64)
		diskMax, diskMaxOk := status["maxdisk"].(float64)

		if !diskUsageOk || !diskMaxOk || diskMax == 0 {
			// Use improved disk info method
			diskClient, err := c.CreateClient()
			if err == nil {
				if maxDisk, usedDisk, err := diskClient.GetVMDiskInfo(context.Background(), node, id); err == nil {
					if maxDisk > 0 {
						diskMax = float64(maxDisk)
						diskMaxOk = true
					}
					if usedDisk > 0 {
						diskUsage = float64(usedDisk)
						diskUsageOk = true
					}
				}
			}
		}

		if diskUsageOk && diskMaxOk && diskMax > 0 {
			diskPercent := (diskUsage / diskMax) * 100
			fmt.Printf("   Disk Usage: %s / %s (%.1f%%)\n",
				formatSize(uint64(diskUsage)),
				formatSize(uint64(diskMax)),
				diskPercent)
		}

		if netin, ok := status["netin"].(float64); ok {
			fmt.Printf("   Network In: %s\n", formatSize(uint64(netin)))
		}

		if netout, ok := status["netout"].(float64); ok {
			fmt.Printf("   Network Out: %s\n", formatSize(uint64(netout)))
		}

		fmt.Printf("\n")
	}

	// Boot Configuration
	fmt.Printf("Boot Configuration:\n")

	if bootOrder, ok := config["boot"].(string); ok {
		fmt.Printf("   Boot Order: %s (%s)\n", bootOrder, decodeBootOrder(bootOrder))
	}

	if bios, ok := config["bios"].(string); ok {
		fmt.Printf("   BIOS: %s\n", bios)
	}

	if machine, ok := config["machine"].(string); ok {
		fmt.Printf("   Machine Type: %s\n", machine)
	}

	fmt.Printf("\n")

	// Security Settings
	fmt.Printf("🔒 Security:\n")

	if protection, ok := config["protection"].(float64); ok {
		if protection == 1 {
			fmt.Printf("   Protection: Enabled\n")
		} else {
			fmt.Printf("   Protection: Disabled\n")
		}
	}

	if startupPolicy, ok := config["startup"].(string); ok {
		fmt.Printf("   Startup Policy: %s\n", startupPolicy)
	}

	if onboot, ok := config["onboot"].(float64); ok {
		if onboot == 1 {
			fmt.Printf("   Start on boot: Yes\n")
		} else {
			fmt.Printf("   Start on boot: No\n")
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

	if ostype, ok := config["ostype"].(string); ok {
		fmt.Printf("   OS Type: %s\n", ostype)
	}

	fmt.Printf("\n")
}

// decodeBootOrder converts Proxmox boot order codes to human-readable descriptions
func decodeBootOrder(bootOrder string) string {
	if bootOrder == "" {
		return "Not configured"
	}

	// Handle new format (order=scsi0;ide2;net0)
	if strings.Contains(bootOrder, "order=") {
		// Extract the order part
		parts := strings.Split(bootOrder, ",")
		for _, part := range parts {
			if strings.HasPrefix(part, "order=") {
				orderPart := strings.TrimPrefix(part, "order=")
				devices := strings.Split(orderPart, ";")
				var descriptions []string
				for i, device := range devices {
					if device != "" {
						descriptions = append(descriptions, fmt.Sprintf("%d. %s", i+1, decodeBootDevice(device)))
					}
				}
				return strings.Join(descriptions, ", ")
			}
		}
	}

	// Handle legacy format (single character codes)
	var descriptions []string
	for i, char := range bootOrder {
		device := string(char)
		description := decodeLegacyBootDevice(device)
		if description != "Unknown" {
			descriptions = append(descriptions, fmt.Sprintf("%d. %s", i+1, description))
		}
	}

	if len(descriptions) == 0 {
		return "Unknown format"
	}

	return strings.Join(descriptions, ", ")
}

// decodeBootDevice converts device names to human-readable descriptions
func decodeBootDevice(device string) string {
	switch {
	case strings.HasPrefix(device, "scsi"):
		return fmt.Sprintf("SCSI Disk (%s)", device)
	case strings.HasPrefix(device, "virtio"):
		return fmt.Sprintf("VirtIO Disk (%s)", device)
	case strings.HasPrefix(device, "ide"):
		return fmt.Sprintf("IDE Disk (%s)", device)
	case strings.HasPrefix(device, "sata"):
		return fmt.Sprintf("SATA Disk (%s)", device)
	case strings.HasPrefix(device, "net"):
		return fmt.Sprintf("Network PXE (%s)", device)
	case device == "cdrom":
		return "CD/DVD Drive"
	default:
		return fmt.Sprintf("Device (%s)", device)
	}
}

// decodeLegacyBootDevice converts legacy single-character boot codes
func decodeLegacyBootDevice(code string) string {
	switch code {
	case "c":
		return "Hard Disk"
	case "d":
		return "CD/DVD Drive"
	case "n":
		return "Network PXE"
	case "a":
		return "Floppy Disk"
	default:
		return "Unknown"
	}
}
