// Package conversion provides utility functions for converting between different units
// and data representations used in Proxmox VE operations, such as byte-to-megabyte conversions.
package conversion

func ConvertBytesToMB(bytes uint64) float64 {
	const bytesPerMB = 1048576 // 1 MB = 1024 * 1024 bytes
	return float64(bytes) / bytesPerMB
}
