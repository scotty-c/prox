package client

import (
	"context"
	"net"
	"strings"
)

// Network-related utility functions for Proxmox operations

// ValidateIP checks if a string is a valid IP address
func ValidateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// GetVMNetworkInterfaces attempts to get all network interfaces for a VM
func (c *ProxmoxClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error) {
	var interfaces []NetworkInterface

	// Try to get interfaces from guest agent
	if guestInterfaces := c.getVMInterfacesFromGuestAgent(ctx, node, vmid); len(guestInterfaces) > 0 {
		interfaces = append(interfaces, guestInterfaces...)
	}

	// If we didn't get interfaces from guest agent, try other methods
	if len(interfaces) == 0 {
		// Fallback to getting primary IP
		if ip, err := c.GetVMIP(ctx, node, vmid); err == nil && ip != "" {
			interfaces = append(interfaces, NetworkInterface{
				Name:      "primary",
				IPAddress: ip,
			})
		}
	}

	return interfaces, nil
}

// getVMInterfacesFromGuestAgent attempts to get network interfaces from guest agent
func (c *ProxmoxClient) getVMInterfacesFromGuestAgent(ctx context.Context, node string, vmid int) []NetworkInterface {
	// This is a placeholder for more detailed guest agent interface detection
	// The actual implementation would call the guest agent API
	var interfaces []NetworkInterface

	// For now, try to get the primary IP through existing methods
	if ip, err := c.GetVMIP(ctx, node, vmid); err == nil && ip != "" {
		interfaces = append(interfaces, NetworkInterface{
			Name:      "eth0", // Default name
			IPAddress: ip,
		})
	}

	return interfaces
}

// GetContainerNetworkInterfaces attempts to get all network interfaces for a container
func (c *ProxmoxClient) GetContainerNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error) {
	var interfaces []NetworkInterface

	// Try to get container IP
	if ip, err := c.GetContainerIP(ctx, node, vmid); err == nil && ip != "" {
		interfaces = append(interfaces, NetworkInterface{
			Name:      "eth0", // Default container interface name
			IPAddress: ip,
		})
	}

	return interfaces, nil
}

// FormatIPAddress formats an IP address for display
func FormatIPAddress(ip string) string {
	if ip == "" {
		return "No IP assigned"
	}

	// Remove any CIDR notation for display
	if strings.Contains(ip, "/") {
		parts := strings.Split(ip, "/")
		return parts[0]
	}

	return ip
}

// IsPrivateIP checks if an IP address is in a private range
func IsPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check for private IP ranges
	private10 := net.ParseIP("10.0.0.0")
	private172 := net.ParseIP("172.16.0.0")
	private192 := net.ParseIP("192.168.0.0")

	// Check 10.0.0.0/8
	if parsedIP.To4() != nil {
		return parsedIP.Mask(net.CIDRMask(8, 32)).Equal(private10) ||
			parsedIP.Mask(net.CIDRMask(12, 32)).Equal(private172) ||
			parsedIP.Mask(net.CIDRMask(16, 32)).Equal(private192)
	}

	return false
}
