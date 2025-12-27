package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/scotty-c/prox/pkg/crypto"
	"github.com/scotty-c/prox/pkg/output"
)

// Config writes a local file to $HOME/.prox/config to hold sensitive information

func Check() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	configFile := home + "/.prox/config"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

// Create writes a local file to $HOME/.prox/config with encrypted credentials
func Create(username string, password string, url string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	configDir := home + "/.prox"
	configFile := configDir + "/config"

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.Mkdir(configDir, 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// Encrypt sensitive data
	encryptedUsername, err := crypto.Encrypt(username)
	if err != nil {
		return fmt.Errorf("failed to encrypt username: %w", err)
	}

	encryptedPassword, err := crypto.Encrypt(password)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Set restrictive permissions
	if err := file.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Write encrypted data
	fmt.Fprintf(file, "username=%s\n", encryptedUsername)
	fmt.Fprintf(file, "password=%s\n", encryptedPassword)
	fmt.Fprintf(file, "url=%s\n", url)

	// Add a marker to indicate this config uses encryption
	fingerprint, _ := crypto.GetKeyFingerprint()
	fmt.Fprintf(file, "# Encrypted config - key fingerprint: %s\n", fingerprint)

	return nil
}

// Read reads the local config file and decrypts sensitive data
func Read() (string, string, string, error) {
	if !Check() {
		return "", "", "", fmt.Errorf("config file does not exist")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := home + "/.prox/config"

	file, err := os.Open(configFile)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// Read the file line by line to handle comments and parse key=value pairs
	scanner := bufio.NewScanner(file)
	var username, password, url string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "username":
			username = value
		case "password":
			password = value
		case "url":
			url = value
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", "", fmt.Errorf("error reading config file: %w", err)
	}

	// Decrypt sensitive data
	decryptedUsername, err := crypto.Decrypt(username)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decrypt username: %w", err)
	}

	decryptedPassword, err := crypto.Decrypt(password)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	return decryptedUsername, decryptedPassword, url, nil
}

// Delete deletes the local config file
func Delete() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := home + "/.prox/config"

	if err := os.Remove(configFile); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}

	output.Resultln("Config deleted successfully")
	return nil
}

// Update updates the local config file with encrypted data
func Update(username string, password string, url string) error {
	if !Check() {
		return fmt.Errorf("config file does not exist")
	}

	// Read current values
	currentUser, currentPass, currentURL, err := Read()
	if err != nil {
		return fmt.Errorf("failed to read current config: %w", err)
	}

	// Use new values if provided, otherwise keep current
	newUsername := currentUser
	newPassword := currentPass
	newURL := currentURL

	if len(strings.TrimSpace(username)) > 0 {
		newUsername = username
	}
	if len(strings.TrimSpace(password)) > 0 {
		newPassword = password
	}
	if len(strings.TrimSpace(url)) > 0 {
		newURL = url
	}

	// Create new config with updated values
	if err := Create(newUsername, newPassword, newURL); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	output.Resultln("Config updated successfully")
	return nil
}

// FirstRun creates config if it doesn't exist
func FirstRun(username string, password string, url string) error {
	if !Check() {
		return Create(username, password, url)
	}

	output.Resultln("Config already exists")
	return nil
}

// MigrateConfig migrates an existing plain text config to encrypted format
func MigrateConfig() error {
	if !Check() {
		return fmt.Errorf("config file does not exist")
	}

	// Try to read with the new method first
	username, password, url, err := Read()
	if err != nil {
		// If new read fails, try legacy read and migrate
		legacyUsername, legacyPassword, legacyURL := readLegacy()
		if legacyUsername == "" && legacyPassword == "" && legacyURL == "" {
			return fmt.Errorf("failed to read config in any format")
		}

		// Create new encrypted config
		if err := Create(legacyUsername, legacyPassword, legacyURL); err != nil {
			return fmt.Errorf("failed to migrate config: %w", err)
		}

		output.Resultln("Config migrated to encrypted format")
		return nil
	}

	// If we can read it successfully, check if it needs encryption
	_, wasEncrypted1, err := crypto.MigrateToEncrypted(username)
	if err != nil {
		return fmt.Errorf("failed to encrypt username: %w", err)
	}

	_, wasEncrypted2, err := crypto.MigrateToEncrypted(password)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	if wasEncrypted1 || wasEncrypted2 {
		// Re-create config with encrypted values
		if err := Create(username, password, url); err != nil {
			return fmt.Errorf("failed to update config with encryption: %w", err)
		}
		output.Resultln("Config updated with encryption")
	}

	return nil
}

// readLegacy reads config in the old plain text format
func readLegacy() (string, string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", ""
	}
	configFile := home + "/.prox/config"

	file, err := os.Open(configFile)
	if err != nil {
		return "", "", ""
	}
	defer file.Close()

	var username, password, url string
	fmt.Fscanf(file, "username=%s\npassword=%s\nurl=%s\n", &username, &password, &url)
	return username, password, url
}
