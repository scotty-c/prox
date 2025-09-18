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

// Encrypt encrypts plaintext using AES-GCM
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := generateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

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

// Decrypt decrypts ciphertext using AES-GCM
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Check if this looks like encrypted data (base64)
	if !isBase64(ciphertext) {
		// Assume it's plain text (backward compatibility)
		return ciphertext, nil
	}

	key, err := generateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// If base64 decode fails, assume it's plain text
		return ciphertext, nil
	}

	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := data[:nonceSize]
	ciphertext_bytes := data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext_bytes, nil)
	if err != nil {
		// If decryption fails, assume it's plain text (backward compatibility)
		return string(data), nil
	}

	return string(plaintext), nil
}

// isBase64 checks if a string is valid base64
func isBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// MigrateToEncrypted encrypts plain text values if they aren't already encrypted
func MigrateToEncrypted(value string) (string, bool, error) {
	if value == "" {
		return "", false, nil
	}

	// Try to decrypt - if it works, it's already encrypted
	_, err := Decrypt(value)
	if err == nil && isBase64(value) {
		// Successfully decrypted, so it was already encrypted
		return value, false, nil
	}

	// If we get here, it's likely plain text, so encrypt it
	encrypted, err := Encrypt(value)
	if err != nil {
		return "", false, fmt.Errorf("failed to encrypt value: %w", err)
	}

	return encrypted, true, nil
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
