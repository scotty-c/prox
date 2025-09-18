package client

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Validation utilities for Proxmox operations

// ValidateVMID checks if a VM ID is valid (typically 100-999999999)
func ValidateVMID(vmid int) error {
	if vmid < 100 || vmid > 999999999 {
		return fmt.Errorf("VMID must be between 100 and 999999999, got %d", vmid)
	}
	return nil
}

// ValidateVMName checks if a VM name is valid according to Proxmox rules
func ValidateVMName(name string) error {
	if name == "" {
		return fmt.Errorf("VM name cannot be empty")
	}

	if len(name) > 15 {
		return fmt.Errorf("VM name cannot exceed 15 characters, got %d", len(name))
	}

	// VM names should only contain alphanumeric characters and hyphens
	matched, _ := regexp.MatchString("^[a-zA-Z0-9-]+$", name)
	if !matched {
		return fmt.Errorf("VM name can only contain alphanumeric characters and hyphens")
	}

	return nil
}

// ValidateMemorySize checks if memory size is valid (in MB)
func ValidateMemorySize(memory int) error {
	if memory < 16 {
		return fmt.Errorf("memory must be at least 16 MB, got %d", memory)
	}

	if memory > 4194304 { // 4TB in MB
		return fmt.Errorf("memory cannot exceed 4TB (4194304 MB), got %d", memory)
	}

	return nil
}

// ValidateCPUCores checks if CPU core count is valid
func ValidateCPUCores(cores int) error {
	if cores < 1 {
		return fmt.Errorf("CPU cores must be at least 1, got %d", cores)
	}

	if cores > 128 {
		return fmt.Errorf("CPU cores cannot exceed 128, got %d", cores)
	}

	return nil
}

// ValidateDiskSize checks if disk size string is valid (e.g., "10G", "1024M")
func ValidateDiskSize(size string) error {
	if size == "" {
		return fmt.Errorf("disk size cannot be empty")
	}

	size = strings.ToUpper(size)

	// Check for valid suffixes
	if !strings.HasSuffix(size, "M") && !strings.HasSuffix(size, "G") && !strings.HasSuffix(size, "T") {
		return fmt.Errorf("disk size must end with M, G, or T (e.g., '10G', '1024M')")
	}

	// Extract the numeric part
	numStr := size[:len(size)-1]
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return fmt.Errorf("invalid disk size format: %s", size)
	}

	if num <= 0 {
		return fmt.Errorf("disk size must be positive, got %s", size)
	}

	// Check for reasonable limits based on suffix
	suffix := size[len(size)-1:]
	switch suffix {
	case "M":
		if num < 1 {
			return fmt.Errorf("disk size in MB must be at least 1, got %s", size)
		}
	case "G":
		if num < 0.001 { // 1MB in GB
			return fmt.Errorf("disk size in GB must be at least 0.001, got %s", size)
		}
	case "T":
		if num < 0.000001 { // 1MB in TB
			return fmt.Errorf("disk size in TB must be at least 0.000001, got %s", size)
		}
	}

	return nil
}

// ValidateNodeName checks if a node name is valid
func ValidateNodeName(node string) error {
	if node == "" {
		return fmt.Errorf("node name cannot be empty")
	}

	// Node names should follow hostname rules
	matched, _ := regexp.MatchString("^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$", node)
	if !matched {
		return fmt.Errorf("node name must be a valid hostname")
	}

	return nil
}

// ValidateContainerTemplate checks if a container template string is valid
func ValidateContainerTemplate(template string) error {
	if template == "" {
		return fmt.Errorf("container template cannot be empty")
	}

	// Templates typically follow format like "ubuntu:22.04" or "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
	if !strings.Contains(template, ":") {
		return fmt.Errorf("container template must include storage reference (e.g., 'ubuntu:22.04')")
	}

	return nil
}

// ParseDiskSizeToBytes converts disk size string to bytes
func ParseDiskSizeToBytes(size string) (uint64, error) {
	if err := ValidateDiskSize(size); err != nil {
		return 0, err
	}

	size = strings.ToUpper(size)
	suffix := size[len(size)-1:]
	numStr := size[:len(size)-1]

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid disk size format: %s", size)
	}

	var bytes uint64
	switch suffix {
	case "M":
		bytes = uint64(num * 1024 * 1024)
	case "G":
		bytes = uint64(num * 1024 * 1024 * 1024)
	case "T":
		bytes = uint64(num * 1024 * 1024 * 1024 * 1024)
	}

	return bytes, nil
}
