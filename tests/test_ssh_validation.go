package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/scotty-c/prox/pkg/container"
)

func main() {
	// Get the directory where this test file is located
	_, filename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filename)
	fmt.Printf("Running SSH key validation tests from: %s\n\n", testDir)

	// Test valid SSH keys
	validKeys := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vbqajDhA+example+key+data user@example.com
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGQcGfXZqYfkSIWECJYHLfbdQKgkSXnJDXF7Ns user@example.com`

	// Test invalid SSH key
	invalidKeys := `invalid-key-format
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vbqajDhA+example+key+data user@example.com`

	// Test empty string
	emptyKeys := ""

	fmt.Println("Testing SSH key validation...")

	// Test valid keys
	count, err := container.ValidateSSHKeys(validKeys)
	if err != nil {
		log.Printf("Valid keys test failed: %v", err)
	} else {
		fmt.Printf("✅ Valid keys test passed: found %d keys\n", count)
	}

	// Test invalid keys
	count, err = container.ValidateSSHKeys(invalidKeys)
	if err != nil {
		fmt.Printf("✅ Invalid keys test passed: %v\n", err)
	} else {
		log.Printf("Invalid keys test failed: should have returned error but got %d keys", count)
	}

	// Test empty keys
	count, err = container.ValidateSSHKeys(emptyKeys)
	if err != nil {
		log.Printf("Empty keys test failed: %v", err)
	} else {
		fmt.Printf("✅ Empty keys test passed: found %d keys\n", count)
	}

	fmt.Println("\nSSH key validation tests completed.")
}
