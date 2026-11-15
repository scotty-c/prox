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
	"strings"
)

// sealedPrefix marks a value sealed with a random key (see Seal). Unlike the
// older scheme, whose values are bare base64 that cannot be told from plain
// text, a sealed value says what it is.
const sealedPrefix = "enc:v2:"

// NewKey returns a random 256-bit key
func NewKey() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// KeyFingerprint identifies key without revealing it, so that a mismatch
// between a file and a key can be reported
func KeyFingerprint(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:8])
}

// IsSealed reports whether value was produced by Seal
func IsSealed(value string) bool {
	return strings.HasPrefix(value, sealedPrefix)
}

// Seal encrypts plaintext with AES-256-GCM under key and returns it as an
// "enc:v2:" value. The empty string stays empty.
func Seal(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	encoded, err := encryptWithKey(key, plaintext)
	if err != nil {
		return "", err
	}
	return sealedPrefix + encoded, nil
}

// Open reverses Seal. It never treats its input as plain text: a value
// without the prefix, a damaged value and a value sealed under another key
// are all errors (ErrWrongKey for the last).
func Open(key []byte, sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	encoded, ok := strings.CutPrefix(sealed, sealedPrefix)
	if !ok {
		return "", errors.New("value is not sealed with a stored key")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < minCiphertextSize {
		return "", errors.New("sealed value is malformed")
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
