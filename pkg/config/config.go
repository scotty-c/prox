package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scotty-c/prox/pkg/crypto"
	"github.com/scotty-c/prox/pkg/output"
)

// Config writes a local file to $HOME/.prox/config to hold sensitive information

// Profile management constants
const (
	defaultProfile = "default"
)

// profileOverride holds a temporary profile override set via --profile flag
var profileOverride string

// SetProfileOverride sets a temporary profile override for the current execution
func SetProfileOverride(profile string) {
	profileOverride = profile
}

// GetProfilesDir returns the path to the profiles directory
func GetProfilesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".prox", "profiles"), nil
}

// GetCurrentProfilePath returns the path to the current-profile file
func GetCurrentProfilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".prox", "current-profile"), nil
}

// GetProfilePath returns the path to a specific profile's config file
func GetProfilePath(profile string) (string, error) {
	if profile == "" {
		profile = defaultProfile
	}
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(profilesDir, profile), nil
}

// GetCurrentProfile returns the name of the current active profile
func GetCurrentProfile() string {
	// Check for profile override first (set via --profile flag)
	if profileOverride != "" {
		return profileOverride
	}

	currentProfilePath, err := GetCurrentProfilePath()
	if err != nil {
		return defaultProfile
	}

	data, err := os.ReadFile(currentProfilePath)
	if err != nil {
		return defaultProfile
	}

	profile := strings.TrimSpace(string(data))
	if profile == "" {
		return defaultProfile
	}
	return profile
}

// SetCurrentProfile sets the active profile
func SetCurrentProfile(profile string) error {
	// Ensure profile exists
	if !ProfileExists(profile) {
		return fmt.Errorf("profile '%s' does not exist", profile)
	}

	currentProfilePath, err := GetCurrentProfilePath()
	if err != nil {
		return err
	}

	// Ensure .prox directory exists
	proxDir := filepath.Dir(currentProfilePath)
	if err := os.MkdirAll(proxDir, 0700); err != nil {
		return fmt.Errorf("failed to create .prox directory: %w", err)
	}

	if err := os.WriteFile(currentProfilePath, []byte(profile), 0600); err != nil {
		return fmt.Errorf("failed to set current profile: %w", err)
	}

	return nil
}

// ListProfiles returns a list of all available profiles
func ListProfiles() ([]string, error) {
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return nil, err
	}

	// If profiles directory doesn't exist, return empty list
	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	profiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			profiles = append(profiles, entry.Name())
		}
	}

	return profiles, nil
}

// ProfileExists checks if a profile exists
func ProfileExists(profile string) bool {
	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return false
	}
	_, err = os.Stat(profilePath)
	return err == nil
}

// CreateProfile creates a new profile with the given credentials
func CreateProfile(profile, username, password, url string) error {
	if profile == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return err
	}

	// Ensure profiles directory exists
	profilesDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profilesDir, 0700); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
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

	file, err := os.Create(profilePath)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
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

// DeleteProfile deletes a profile
func DeleteProfile(profile string) error {
	if profile == defaultProfile {
		return fmt.Errorf("cannot delete the default profile")
	}

	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return err
	}

	if err := os.Remove(profilePath); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	// If this was the current profile, switch to default
	if GetCurrentProfile() == profile {
		if err := SetCurrentProfile(defaultProfile); err != nil {
			// Ignore error if default doesn't exist
			return nil
		}
	}

	return nil
}

// ReadProfile reads a specific profile's configuration
func ReadProfile(profile string) (string, string, string, error) {
	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return "", "", "", err
	}

	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return "", "", "", fmt.Errorf("profile '%s' does not exist", profile)
	}

	file, err := os.Open(profilePath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open profile file: %w", err)
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
		return "", "", "", fmt.Errorf("error reading profile file: %w", err)
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

// MigrateToProfiles migrates the old single config file to the new profile system
func MigrateToProfiles() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	oldConfigFile := filepath.Join(home, ".prox", "config")

	// Check if old config exists
	if _, err := os.Stat(oldConfigFile); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}

	// Check if already migrated
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(profilesDir); err == nil {
		// Profiles directory exists, assume already migrated
		return nil
	}

	// Read old config using the old read logic
	username, password, url, err := readOldConfig()
	if err != nil {
		return fmt.Errorf("failed to read old config: %w", err)
	}

	// Create default profile
	if err := CreateProfile(defaultProfile, username, password, url); err != nil {
		return fmt.Errorf("failed to create default profile: %w", err)
	}

	// Set as current profile
	if err := SetCurrentProfile(defaultProfile); err != nil {
		return fmt.Errorf("failed to set current profile: %w", err)
	}

	// Backup and remove old config
	backupFile := oldConfigFile + ".backup"
	if err := os.Rename(oldConfigFile, backupFile); err != nil {
		return fmt.Errorf("failed to backup old config: %w", err)
	}

	output.Resultln("Migrated configuration to profile system")
	output.Resultln("Old config backed up to: " + backupFile)

	return nil
}

// readOldConfig reads the old single config file
func readOldConfig() (string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := filepath.Join(home, ".prox", "config")

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

func Check() bool {
	// Try profile system first
	profile := GetCurrentProfile()
	if ProfileExists(profile) {
		return true
	}

	// Fall back to old config location for backwards compatibility
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	configFile := filepath.Join(home, ".prox", "config")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return false
	}
	return true
}

// Create writes a config for the current profile with encrypted credentials
func Create(username string, password string, url string) error {
	profile := GetCurrentProfile()
	if err := CreateProfile(profile, username, password, url); err != nil {
		return err
	}

	// Set as current profile if not already set
	if err := SetCurrentProfile(profile); err != nil {
		// Ignore error if it's already the current profile
	}

	return nil
}

// Read reads the current profile's config file and decrypts sensitive data
func Read() (string, string, string, error) {
	// Try to migrate old config if it exists
	if err := MigrateToProfiles(); err != nil {
		// Ignore migration errors, might not be needed
	}

	profile := GetCurrentProfile()
	return ReadProfile(profile)
}

// Delete deletes the current profile's config file
func Delete() error {
	profile := GetCurrentProfile()
	if err := DeleteProfile(profile); err != nil {
		return err
	}

	output.Resultln("Profile deleted successfully")
	return nil
}

// Update updates the current profile's config file with encrypted data
func Update(username string, password string, url string) error {
	if !Check() {
		return fmt.Errorf("config file does not exist")
	}

	profile := GetCurrentProfile()

	// Read current values
	currentUser, currentPass, currentURL, err := ReadProfile(profile)
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
	if err := CreateProfile(profile, newUsername, newPassword, newURL); err != nil {
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
	configFile := filepath.Join(home, ".prox", "config")

	file, err := os.Open(configFile)
	if err != nil {
		return "", "", ""
	}
	defer file.Close()

	var username, password, url string
	fmt.Fscanf(file, "username=%s\npassword=%s\nurl=%s\n", &username, &password, &url)
	return username, password, url
}
