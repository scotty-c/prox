package vm

import (
	"context"
	"fmt"
	"strings"
	"sync"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/util"
)

// VMDetails holds all information about a VM for JSON output
type VMDetails struct {
	ID     int                    `json:"id"`
	Name   string                 `json:"name"`
	Node   string                 `json:"node"`
	Config map[string]interface{} `json:"config"`
	Status map[string]interface{} `json:"status"`
	IP     string                 `json:"ip,omitempty"`
}

// GetVMDetails fetches detailed VM information
func GetVMDetails(nameOrID string, node string) (*VMDetails, error) {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}

	// Find the VM by name or ID
	vm, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("failed to find VM: %w", err)
	}

	// Use the discovered node if no node was specified
	if node == "" {
		node = vm.Node
	}

	// Fetch config and status in parallel
	var (
		config               map[string]interface{}
		status               map[string]interface{}
		configErr, statusErr error
		wg                   sync.WaitGroup
	)

	wg.Add(2)

	// Get VM configuration
	go func() {
		defer wg.Done()
		config, configErr = client.GetVMConfig(ctx, node, vm.ID)
	}()

	// Get VM status
	go func() {
		defer wg.Done()
		status, statusErr = client.GetVMStatus(ctx, node, vm.ID)
	}()

	wg.Wait()

	// Check for errors
	if configErr != nil {
		return nil, fmt.Errorf("failed to get VM config: %w", configErr)
	}
	if statusErr != nil {
		return nil, fmt.Errorf("failed to get VM status: %w", statusErr)
	}

	// Get VM IP if available
	vmIP := GetIp(ctx, vm.ID, node)
	if vmIP == "Error getting IP" {
		vmIP = ""
	}

	return &VMDetails{
		ID:     vm.ID,
		Name:   vm.Name,
		Node:   node,
		Config: config,
		Status: status,
		IP:     vmIP,
	}, nil
}

// DescribeVM displays detailed VM information in a modern, sectioned format
func DescribeVM(nameOrID string, node string) error {
	details, err := GetVMDetails(nameOrID, node)
	if err != nil {
		return err
	}

	output.Info("Getting VM details for %s...\n", nameOrID)

	// Display VM information
	displayVMDetails(details.ID, details.Name, details.Node, details.Config, details.Status, details.IP)

	return nil
}

// displayVMDetails displays detailed VM information
func displayVMDetails(id int, name, node string, config, status map[string]interface{}, ip string) {
	output.Result("\nVirtual Machine Details\n")
	output.Result("═══════════════════════════════════════════════════════════════════════════════════════\n")

	// Basic Information
	output.Result("Basic Information:\n")
	output.Result("   Name: %s\n", name)
	output.Result("   ID: %d\n", id)
	output.Result("   Node: %s\n", node)

	// VM status
	if vmStatus, ok := status["status"].(string); ok {
		output.Result("   Status: %s\n", vmStatus)
	}

	// VM agent status
	if agent, ok := config["agent"].(float64); ok && agent == 1 {
		output.Result("   QEMU Agent: Enabled\n")
	} else {
		output.Result("   QEMU Agent: Disabled\n")
	}

	output.Result("\n")

	// Resource Configuration
	output.Result("Resource Configuration:\n")

	// Memory
	if memory, ok := config["memory"].(float64); ok {
		output.Result("   Memory: %.0f MB\n", memory)
	}

	// Balloon memory
	if balloon, ok := config["balloon"].(float64); ok {
		output.Result("   Balloon Memory: %.0f MB\n", balloon)
	}

	// CPU
	if sockets, ok := config["sockets"].(float64); ok {
		output.Result("   CPU Sockets: %.0f\n", sockets)
	}
	if cores, ok := config["cores"].(float64); ok {
		output.Result("   CPU Cores: %.0f\n", cores)
	}
	if vcpus, ok := config["vcpus"].(float64); ok {
		output.Result("   vCPUs: %.0f\n", vcpus)
	}
	if cpuType, ok := config["cpu"].(string); ok {
		output.Result("   CPU Type: %s\n", cpuType)
	}

	output.Result("\n")

	// Storage Information
	output.Result("Storage:\n")

	// Display disks
	diskKeys := []string{"ide0", "ide1", "ide2", "ide3", "sata0", "sata1", "sata2", "sata3", "scsi0", "scsi1", "scsi2", "scsi3", "virtio0", "virtio1", "virtio2", "virtio3"}
	for _, diskKey := range diskKeys {
		if disk, ok := config[diskKey].(string); ok {
			output.Result("   %s: %s\n", strings.ToUpper(diskKey), disk)
		}
	}

	output.Result("\n")

	// Network Configuration
	output.Result("Network:\n")

	// Display network interfaces
	for i := 0; i < 10; i++ {
		netKey := fmt.Sprintf("net%d", i)
		if net, ok := config[netKey].(string); ok {
			output.Result("   Network %d: %s\n", i, net)
		}
	}

	// Display VM IP if available
	if ip != "" {
		output.Result("   IP Address: %s\n", ip)
	}

	output.Result("\n")

	// Runtime Status (if running)
	if vmStatus, ok := status["status"].(string); ok && vmStatus == "running" {
		output.Result("Runtime Status:\n")

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
			output.Result("   Disk Usage: %s / %s (%.1f%%)\n",
				util.FormatSize(uint64(diskUsage)),
				util.FormatSize(uint64(diskMax)),
				diskPercent)
		}

		if netin, ok := status["netin"].(float64); ok {
			output.Result("   Network In: %s\n", util.FormatSize(uint64(netin)))
		}

		if netout, ok := status["netout"].(float64); ok {
			output.Result("   Network Out: %s\n", util.FormatSize(uint64(netout)))
		}

		output.Result("\n")
	}

	// Boot Configuration
	output.Result("Boot Configuration:\n")

	if bootOrder, ok := config["boot"].(string); ok {
		output.Result("   Boot Order: %s (%s)\n", bootOrder, decodeBootOrder(bootOrder))
	}

	if bios, ok := config["bios"].(string); ok {
		output.Result("   BIOS: %s\n", bios)
	}

	if machine, ok := config["machine"].(string); ok {
		output.Result("   Machine Type: %s\n", machine)
	}

	output.Result("\n")

	// Security Settings
	output.Result("Security:\n")

	if protection, ok := config["protection"].(float64); ok {
		if protection == 1 {
			output.Result("   Protection: Enabled\n")
		} else {
			output.Result("   Protection: Disabled\n")
		}
	}

	if startupPolicy, ok := config["startup"].(string); ok {
		output.Result("   Startup Policy: %s\n", startupPolicy)
	}

	if onboot, ok := config["onboot"].(float64); ok {
		if onboot == 1 {
			output.Result("   Start on boot: Yes\n")
		} else {
			output.Result("   Start on boot: No\n")
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

	if ostype, ok := config["ostype"].(string); ok {
		output.Result("   OS Type: %s\n", ostype)
	}

	output.Result("\n")
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
