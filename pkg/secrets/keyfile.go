package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/scotty-c/prox/pkg/crypto"
)

// KeyFilePath is where the random key lives: prox/key in the user's config
// directory (~/.config on Linux, ~/Library/Application Support on macOS,
// %AppData% on Windows). It is kept apart from ~/.prox, where the profiles
// are, so that copying, backing up or committing the profiles does not also
// copy the key that opens them.
func KeyFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("cannot find a place for the key file: %w", err)
		}
		return filepath.Join(home, ".prox", "key"), nil
	}
	return filepath.Join(dir, "prox", "key"), nil
}

// loadKey reads the key file. With create it makes one first if there is none.
func loadKey(create bool) (key []byte, path string, err error) {
	path, err = KeyFilePath()
	if err != nil {
		return nil, "", err
	}

	key, err = readKeyFile(path)
	if errors.Is(err, os.ErrNotExist) && create {
		if err = createKeyFile(path); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, path, err
		}
		// created now, or by another prox that got there first
		key, err = readKeyFile(path)
	}
	return key, path, err
}

func readKeyFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	// Like ssh with a private key: a key that other users can read protects nothing
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("the key file %s can be read by other users (mode %04o); run: chmod 600 %s", path, info.Mode().Perm(), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("the key file %s is damaged (expected a 256-bit key)", path)
	}
	return key, nil
}

// createKeyFile writes a new random key. It fails with os.ErrExist rather than
// overwrite an existing key, which would make everything sealed with it
// unreadable.
func createKeyFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}
	key, err := crypto.NewKey()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(base64.StdEncoding.EncodeToString(key) + "\n"); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("failed to write the key file: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("failed to write the key file: %w", err)
	}
	return nil
}
