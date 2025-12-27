package output

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// QuietMode indicates whether to suppress non-essential output
var QuietMode bool

// Info prints informational messages (suppressed in quiet mode)
func Info(format string, a ...interface{}) {
	if !QuietMode {
		fmt.Printf(format, a...)
	}
}

// Infoln prints informational messages with newline (suppressed in quiet mode)
func Infoln(a ...interface{}) {
	if !QuietMode {
		fmt.Println(a...)
	}
}

// Result prints final results (always shown, even in quiet mode)
func Result(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

// Resultln prints final results with newline (always shown, even in quiet mode)
func Resultln(a ...interface{}) {
	fmt.Println(a...)
}

// Error prints error messages to stderr (always shown)
func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// Errorln prints error messages to stderr with newline (always shown)
func Errorln(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

// SetQuiet enables or disables quiet mode
func SetQuiet(quiet bool) {
	QuietMode = quiet
}

// IsQuiet returns whether quiet mode is enabled
func IsQuiet() bool {
	return QuietMode
}

// Confirm prompts the user for confirmation before a destructive action
// Returns true if the user confirms (y/yes), false otherwise
// The prompt should be phrased as a yes/no question
func Confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// ClientError prints an error message for client creation failures with actionable advice
func ClientError(err error) {
	fmt.Fprintf(os.Stderr, "Error: Failed to connect to Proxmox VE\n")
	fmt.Fprintf(os.Stderr, "Details: %v\n", err)

	// Check for common error patterns and provide specific advice
	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "config file does not exist") || strings.Contains(errMsg, "failed to read config") {
		fmt.Fprintf(os.Stderr, "\nTip: No configuration found. Run 'prox config setup' to configure your connection\n")
	} else if strings.Contains(errMsg, "authentication") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "unauthorized") {
		fmt.Fprintf(os.Stderr, "\nTip: Authentication failed. Check your credentials with 'prox config setup'\n")
	} else if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "timeout") {
		fmt.Fprintf(os.Stderr, "\nTip: Cannot reach Proxmox server. Verify:\n")
		fmt.Fprintf(os.Stderr, "  • Server URL is correct ('prox config setup' to update)\n")
		fmt.Fprintf(os.Stderr, "  • Proxmox server is running and accessible\n")
		fmt.Fprintf(os.Stderr, "  • Network connectivity\n")
	} else if strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "ssl") {
		fmt.Fprintf(os.Stderr, "\nTip: TLS/SSL certificate issue. Verify the server URL is correct\n")
	} else if strings.Contains(errMsg, "decrypt") || strings.Contains(errMsg, "encryption") {
		fmt.Fprintf(os.Stderr, "\nTip: Configuration decryption failed. Run 'prox config setup' to reconfigure\n")
	} else {
		fmt.Fprintf(os.Stderr, "\nTip: Run 'prox config setup' to verify your configuration\n")
	}
}

// AuthError prints an error message for authentication failures with actionable advice
func AuthError(err error) {
	fmt.Fprintf(os.Stderr, "Error: Authentication failed\n")
	fmt.Fprintf(os.Stderr, "Details: %v\n", err)
	fmt.Fprintf(os.Stderr, "\nTip: Check your credentials:\n")
	fmt.Fprintf(os.Stderr, "  • Username should include realm (e.g., root@pam)\n")
	fmt.Fprintf(os.Stderr, "  • Password must be correct\n")
	fmt.Fprintf(os.Stderr, "  • Run 'prox config setup' to update credentials\n")
}

// ConnectionError prints an error message for connection failures with actionable advice
func ConnectionError(operation string, err error) {
	fmt.Fprintf(os.Stderr, "Error: Failed to %s\n", operation)
	fmt.Fprintf(os.Stderr, "Details: %v\n", err)

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "timeout") {
		fmt.Fprintf(os.Stderr, "\nTip: Connection issue. Check:\n")
		fmt.Fprintf(os.Stderr, "  • Proxmox server is running\n")
		fmt.Fprintf(os.Stderr, "  • Network connectivity to server\n")
		fmt.Fprintf(os.Stderr, "  • Firewall allows port 8006\n")
	} else if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
		fmt.Fprintf(os.Stderr, "\nTip: Resource not found. Verify the resource name or ID is correct\n")
	} else {
		fmt.Fprintf(os.Stderr, "\nTip: Run 'prox status' to verify cluster connectivity\n")
	}
}

// APIError prints an error message for API operation failures with actionable advice
func APIError(operation string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s failed\n", operation)
	fmt.Fprintf(os.Stderr, "Details: %v\n", err)

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "403") {
		fmt.Fprintf(os.Stderr, "\nTip: Permission denied. Check that your user has the required privileges\n")
	} else if strings.Contains(errMsg, "exists") || strings.Contains(errMsg, "duplicate") {
		fmt.Fprintf(os.Stderr, "\nTip: Resource already exists. Use a different name or ID\n")
	} else if strings.Contains(errMsg, "not found") {
		fmt.Fprintf(os.Stderr, "\nTip: Resource not found. Use 'prox vm list' or 'prox ct list' to see available resources\n")
	}
}
