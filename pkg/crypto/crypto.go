package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"
	"strings"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // GCM nonce size
)

// generateKey creates a deterministic encryption key based on system info and user
func generateKey() ([]byte, error) {
	// Get system and user information
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Create a deterministic key from system info
	keyMaterial := fmt.Sprintf("%s:%s:%s:%s",
		currentUser.Uid,
		currentUser.Username,
		hostname,
		runtime.GOOS,
	)

	// Hash the key material to get a consistent 32-byte key
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:], nil
}

// encryptedPrefix marks a value as encrypted. Decrypt accepts it, but Encrypt
// does not write it yet so that config files stay readable by older versions.
const encryptedPrefix = "enc:v1:"

// minCiphertextSize is the smallest possible AES-GCM output: nonce plus tag
const minCiphertextSize = nonceSize + 16

// ErrWrongKey is returned when a value was encrypted with a different key,
// for example after the hostname or username changed.
var ErrWrongKey = errors.New("failed to decrypt credentials: they were encrypted with a different key (the hostname or username may have changed)")

// Encrypt encrypts plaintext using AES-GCM
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := generateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return encryptWithKey(key, plaintext)
}

func encryptWithKey(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-GCM.
//
// Values that do not look like ciphertext are returned unchanged, so plain
// text values from older config files keep working. A value that looks like
// ciphertext but fails to decrypt returns ErrWrongKey.
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key, err := generateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return decryptWithKey(key, ciphertext)
}

func decryptWithKey(key []byte, ciphertext string) (string, error) {
	value, prefixed := strings.CutPrefix(ciphertext, encryptedPrefix)

	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(data) < minCiphertextSize {
		if prefixed {
			return "", errors.New("failed to decrypt credentials: encrypted value is malformed")
		}
		// Not ciphertext: assume plain text (backward compatibility)
		return ciphertext, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", ErrWrongKey
	}

	return string(plaintext), nil
}

// GetKeyFingerprint returns a fingerprint of the encryption key for verification
func GetKeyFingerprint() (string, error) {
	key, err := generateKey()
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(key)
	return hex.EncodeToString(hash[:8]), nil // First 8 bytes as hex
}
