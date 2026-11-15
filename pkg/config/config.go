package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/scotty-c/prox/pkg/apierr"
	"github.com/scotty-c/prox/pkg/crypto"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/secrets"
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
	if err := validateProfileName(profile); err != nil {
		return "", err
	}
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(profilesDir, profile), nil
}

// validateProfileName rejects names that would point outside the profiles
// directory, such as "../config" or "a/b"
func validateProfileName(profile string) error {
	if profile == "." || profile == ".." || strings.ContainsAny(profile, `/\`) || filepath.Base(profile) != profile {
		return fmt.Errorf("invalid profile name %q: profile names cannot contain path separators", profile)
	}
	return nil
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
		// Skip directories and hidden files, such as temporary files left by
		// an interrupted save
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			profiles = append(profiles, entry.Name())
		}
	}

	return profiles, nil
}

// ProfileInfo describes a saved profile for listing. The password is never
// included; if the profile cannot be read (for example its credentials were
// encrypted on another machine) Error says why and the other fields are empty.
type ProfileInfo struct {
	Name    string
	URL     string
	User    string
	Current bool
	Storage string // how the credentials are protected: keychain, key file, legacy, plain text
	Error   string
}

// ProfileInfos describes every saved profile, marking the current one
func ProfileInfos() ([]ProfileInfo, error) {
	names, err := ListProfiles()
	if err != nil {
		return nil, err
	}
	current := GetCurrentProfile()
	infos := make([]ProfileInfo, 0, len(names))
	for _, name := range names {
		info := ProfileInfo{Name: name, Current: name == current}
		detail, err := OpenProfile(name)
		if err != nil {
			info.Error = err.Error()
		} else {
			info.User, info.URL, info.Storage = detail.Username, detail.URL, string(detail.Method)
		}
		infos = append(infos, info)
	}
	return infos, nil
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

// SaveProfile writes a profile with the given credentials, URL and TLS
// settings, replacing any profile of that name, and reports how the credentials
// were protected so the caller can tell the user. The credentials are
// protected by package secrets: in the OS keychain when there is one,
// otherwise sealed with a key file.
func SaveProfile(profile, username, password, url string, tlsOpts TLSOptions) (secrets.Method, error) {
	return createProfile(profile, username, password, url, tlsOpts)
}

// DescribeMethod says in a sentence where credentials protected by method live
func DescribeMethod(m secrets.Method) string {
	switch m {
	case secrets.MethodKeychain:
		return "in the OS keychain"
	case secrets.MethodKeyFile:
		return "sealed with a key file"
	}
	return "using the " + string(m) + " scheme"
}

// createProfile is SaveProfile
func createProfile(profile, username, password, url string, tlsOpts TLSOptions) (secrets.Method, error) {
	if profile == "" {
		return "", fmt.Errorf("profile name cannot be empty")
	}

	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return "", err
	}
	_, statErr := os.Stat(profilePath)
	existed := statErr == nil

	// Ensure profiles directory exists
	profilesDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profilesDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create profiles directory: %w", err)
	}

	sealed, err := secrets.Seal(profile, secrets.Credentials{Username: username, Password: password})
	if err != nil {
		return "", fmt.Errorf("failed to protect the credentials: %w", err)
	}
	if sealed.Note != "" {
		output.Errorln("Note: " + sealed.Note)
	}

	var content strings.Builder
	if marker, ok := sealed.Entries["credentials"]; ok {
		fmt.Fprintf(&content, "credentials=%s\n", marker)
	} else {
		fmt.Fprintf(&content, "username=%s\n", sealed.Entries["username"])
		fmt.Fprintf(&content, "password=%s\n", sealed.Entries["password"])
	}
	fmt.Fprintf(&content, "url=%s\n", url)
	writeTLSOptions(&content, tlsOpts)
	if sealed.Fingerprint != "" {
		// helps diagnose a key file that has been replaced
		fmt.Fprintf(&content, "%s %s\n", fingerprintComment, sealed.Fingerprint)
	}

	forgetOpened()
	if err := writeProfileFile(profilePath, content.String()); err != nil {
		if !existed {
			_ = secrets.Forget(profile, sealed.Entries) // don't leave an orphaned keychain item
		}
		return "", err
	}
	return sealed.Method, nil
}

// writeProfileFile writes a profile with 0600 permissions from the start, via
// a temporary file and rename so a failure never leaves a half-written profile.
func writeProfileFile(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write profile file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to save profile file: %w", err)
	}
	return nil
}

// DeleteProfile deletes a profile, including its keychain item if it has one
func DeleteProfile(profile string) error {
	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return err
	}

	forgetOpened()

	// what the profile says about where its credentials are, before it goes
	entries, _, _ := readProfileFile(profilePath)

	if err := os.Remove(profilePath); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	// If this was the current profile, switch to default. If there is no
	// default profile, remove the pointer so it doesn't name a deleted profile.
	if currentProfileSetting() == profile {
		if err := SetCurrentProfile(defaultProfile); err != nil {
			if path, err := GetCurrentProfilePath(); err == nil {
				_ = os.Remove(path)
			}
		}
	}

	if err := secrets.Forget(profile, entries); err != nil {
		return fmt.Errorf("the profile was deleted, but %w", err)
	}
	return nil
}

// currentProfileSetting returns the profile saved by 'prox config use',
// ignoring any --profile override
func currentProfileSetting() string {
	path, err := GetCurrentProfilePath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ProfileDetail is a saved profile with its credentials opened
type ProfileDetail struct {
	Profile  string
	Username string
	Password string
	URL      string
	Method   secrets.Method  // how the credentials are protected
	Session  secrets.Session // the last login, if the keychain item kept one
}

// OpenProfile reads a profile and opens its credentials, whichever way they
// were protected
func OpenProfile(profile string) (ProfileDetail, error) {
	profilePath, err := GetProfilePath(profile)
	if err != nil {
		return ProfileDetail{}, err
	}

	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return ProfileDetail{}, fmt.Errorf("%w: profile '%s' does not exist", apierr.ErrNoConfig, profile)
	}

	values, fingerprint, err := readProfileFile(profilePath)
	if err != nil {
		return ProfileDetail{}, fmt.Errorf("failed to read profile '%s': %w", profile, err)
	}

	creds, method, err := secrets.Open(profile, values)
	if err != nil {
		if errors.Is(err, crypto.ErrWrongKey) { // the older scheme, tied to the host name
			return ProfileDetail{}, decryptError(profile, fingerprint, err)
		}
		return ProfileDetail{}, err
	}
	return ProfileDetail{Profile: profile, Username: creds.Username, Password: creds.Password, URL: values["url"], Method: method, Session: creds.Session}, nil
}

// ReadProfile reads a specific profile's configuration
func ReadProfile(profile string) (string, string, string, error) {
	detail, err := OpenProfile(profile)
	if err != nil {
		return "", "", "", err
	}
	noteIfLegacy(detail)
	return detail.Username, detail.Password, detail.URL, nil
}

// noteIfLegacy points a profile that still uses an old storage scheme at the
// command that upgrades it, once per run
func noteIfLegacy(detail ProfileDetail) {
	if detail.Method == secrets.MethodLegacy || detail.Method == secrets.MethodPlaintext {
		legacyNote.Do(func() {
			output.Info("Note: the credentials of profile '%s' use the old storage scheme (%s). Run 'prox config migrate' to move them into the keychain.\n", detail.Profile, detail.Method)
		})
	}
}

// opened remembers the current profile once it has been read, so that the
// several parts of a command that ask for the connection settings cost one
// read of the keychain, not one each. Writing or deleting a profile forgets it.
var opened struct {
	sync.Mutex
	detail ProfileDetail
	ok     bool
}

func forgetOpened() {
	opened.Lock()
	defer opened.Unlock()
	opened.ok = false
}

// ReadDetail reads the current profile for connecting: its login and, if the
// keychain item kept one, the last session. It is read once per run.
func ReadDetail() (ProfileDetail, error) {
	// Try to migrate old config if it exists
	// Ignore migration errors: migration might not be needed, and reading the
	// profile below reports any real problem
	_ = MigrateToProfiles()

	profile := GetCurrentProfile()

	opened.Lock()
	defer opened.Unlock()
	if opened.ok && opened.detail.Profile == profile {
		return opened.detail, nil
	}
	detail, err := OpenProfile(profile)
	if err != nil {
		return ProfileDetail{}, err
	}
	noteIfLegacy(detail)
	opened.detail, opened.ok = detail, true
	return detail, nil
}

// SaveSession keeps a login in the profile's keychain item so the next run can
// skip logging in. It does nothing useful for a profile that is not in the
// keychain, and an error means only that the next run will log in again.
func SaveSession(profile, ticket, csrf string) error {
	return secrets.SaveSession(profile, secrets.Session{Ticket: ticket, CSRF: csrf, Saved: time.Now()})
}

// legacyNote makes the nudge to migrate appear once per run
var legacyNote sync.Once

// fingerprintComment is the comment written to profile files recording the key fingerprint
const fingerprintComment = "# Encrypted config - key fingerprint:"

// decryptError explains a failure to decrypt a profile and how to fix it
func decryptError(profile, storedFingerprint string, err error) error {
	if !errors.Is(err, crypto.ErrWrongKey) {
		return fmt.Errorf("failed to decrypt profile '%s': %w", profile, err)
	}
	detail := ""
	if current, fpErr := crypto.GetKeyFingerprint(); fpErr == nil && storedFingerprint != "" && storedFingerprint != current {
		detail = fmt.Sprintf(" (profile key fingerprint %s, current %s)", storedFingerprint, current)
	}
	return fmt.Errorf("%w%s. Re-enter your credentials with: prox config setup %s -u <user> -p <password> -l <url>", err, detail, profile)
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
	if _, err := SaveProfile(defaultProfile, username, password, url, TLSOptions{}); err != nil {
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

	// Report on stderr: this can run in the middle of any command, including
	// ones whose stdout is JSON
	output.Errorln("Migrated configuration to profile system")
	output.Errorln("Old config backed up to: " + backupFile)

	return nil
}

// readOldConfig reads the old single config file
func readOldConfig() (string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := filepath.Join(home, ".prox", "config")

	values, _, err := readProfileFile(configFile)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read config file: %w", err)
	}
	username, password, url := values["username"], values["password"], values["url"]

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

// Read reads the current profile's config file and decrypts sensitive data
func Read() (string, string, string, error) {
	detail, err := ReadDetail()
	if err != nil {
		return "", "", "", err
	}
	return detail.Username, detail.Password, detail.URL, nil
}

// UpgradeResult is what happened to one profile in UpgradeProfiles
type UpgradeResult struct {
	Profile string
	From    secrets.Method
	To      secrets.Method // the same as From if nothing was done
	Err     error
}

// UpgradeProfiles moves profiles whose credentials use an old scheme (a key
// derived from the user and host name, or plain text) to the current one: the
// keychain, or the key file where there is none. only names one profile; ""
// means all of them.
//
// Each profile is checked after it is written: its credentials must read back
// the same, or the original file is put back and the new keychain item
// removed, so a failed upgrade leaves the profile as it was.
func UpgradeProfiles(only string) ([]UpgradeResult, error) {
	names := []string{only}
	if only == "" {
		var err error
		if names, err = ListProfiles(); err != nil {
			return nil, err
		}
	}

	var results []UpgradeResult
	for _, name := range names {
		results = append(results, upgradeProfile(name))
	}
	return results, nil
}

func upgradeProfile(profile string) UpgradeResult {
	result := UpgradeResult{Profile: profile}

	before, err := OpenProfile(profile)
	if err != nil {
		result.Err = err
		return result
	}
	result.From, result.To = before.Method, before.Method
	if before.Method.Current() {
		return result // already protected properly
	}

	path, err := GetProfilePath(profile)
	if err != nil {
		result.Err = err
		return result
	}
	original, err := os.ReadFile(path)
	if err != nil {
		result.Err = fmt.Errorf("failed to read profile '%s': %w", profile, err)
		return result
	}

	// the TLS settings are kept as they were
	tlsOpts, _ := ReadTLSOptions(profile)
	method, err := createProfile(profile, before.Username, before.Password, before.URL, tlsOpts)
	if err != nil {
		// the original file is untouched; don't leave a keychain item behind
		_ = secrets.Forget(profile, map[string]string{"credentials": "keychain"})
		result.Err = err
		return result
	}

	// Read it back before trusting it
	after, err := OpenProfile(profile)
	if err != nil || after.Username != before.Username || after.Password != before.Password || after.URL != before.URL {
		if err == nil {
			err = fmt.Errorf("the credentials did not read back the same")
		}
		// put everything back as it was
		_ = writeProfileFile(path, string(original))
		_ = secrets.Forget(profile, map[string]string{"credentials": "keychain"})
		result.Err = fmt.Errorf("the upgrade of profile '%s' was undone: %w", profile, err)
		return result
	}
	result.To = method
	return result
}
