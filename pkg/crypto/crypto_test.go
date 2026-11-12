package crypto

import (
	"crypto/sha256"
	"errors"
	"testing"
)

func testKey(material string) []byte {
	h := sha256.Sum256([]byte(material))
	return h[:]
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey("a")
	for _, plain := range []string{"root@pam", "s3cret!", "password", "a much longer password with spaces"} {
		enc, err := encryptWithKey(key, plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		got, err := decryptWithKey(key, enc)
		if err != nil || got != plain {
			t.Fatalf("decrypt %q = %q, %v", plain, got, err)
		}
		// Future prefixed format is also accepted
		got, err = decryptWithKey(key, encryptedPrefix+enc)
		if err != nil || got != plain {
			t.Fatalf("decrypt prefixed %q = %q, %v", plain, got, err)
		}
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	key := testKey("a")
	// These are all valid base64, which previously caused them to be mangled
	for _, plain := range []string{"password", "admin123", "Passw0rd", "secret!!", "root@pam", "https://pve:8006"} {
		got, err := decryptWithKey(key, plain)
		if err != nil || got != plain {
			t.Errorf("decrypt legacy %q = %q, %v; want unchanged", plain, got, err)
		}
	}
}

func TestDecryptWrongKey(t *testing.T) {
	enc, err := encryptWithKey(testKey("host-a"), "password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptWithKey(testKey("host-b"), enc); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("expected ErrWrongKey, got %v", err)
	}
}
