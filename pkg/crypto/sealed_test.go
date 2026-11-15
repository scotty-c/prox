package crypto

import (
	"errors"
	"strings"
	"testing"
)

func TestSealAndOpenRoundTrip(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"root@pam", "p@ss=w0rd==", "ünïcödé ✓", strings.Repeat("x", 4000)} {
		sealed, err := Seal(key, plain)
		if err != nil {
			t.Fatal(err)
		}
		if !IsSealed(sealed) || strings.Contains(sealed, plain) {
			t.Errorf("sealed value should be marked and hide the plain text: %q", sealed)
		}
		if got, err := Open(key, sealed); err != nil || got != plain {
			t.Errorf("Open = %q, %v; want %q", got, err, plain)
		}
	}
}

func TestSealingTheSameValueTwiceGivesDifferentCiphertext(t *testing.T) {
	key, _ := NewKey()
	a, _ := Seal(key, "secret")
	b, _ := Seal(key, "secret")
	if a == b {
		t.Error("each seal needs a fresh nonce")
	}
}

func TestEmptyValuesStayEmpty(t *testing.T) {
	key, _ := NewKey()
	if s, err := Seal(key, ""); s != "" || err != nil {
		t.Errorf("Seal(\"\") = %q, %v", s, err)
	}
	if p, err := Open(key, ""); p != "" || err != nil {
		t.Errorf("Open(\"\") = %q, %v", p, err)
	}
}

func TestOpenNeverFallsBackToPlainText(t *testing.T) {
	key, _ := NewKey()
	other, _ := NewKey()
	sealed, _ := Seal(key, "secret")

	if _, err := Open(other, sealed); !errors.Is(err, ErrWrongKey) {
		t.Errorf("another key: %v, want ErrWrongKey", err)
	}
	for _, bad := range []string{"secret", "enc:v2:", "enc:v2:!!!not base64!!!", "enc:v2:AAAA", "enc:v1:" + sealed[len("enc:v2:"):]} {
		if p, err := Open(key, bad); err == nil {
			t.Errorf("Open(%q) = %q, want an error", bad, p)
		}
	}
	// a flipped bit is caught by the authentication tag
	raw := []byte(sealed)
	raw[len(raw)-3] ^= 1
	if _, err := Open(key, string(raw)); err == nil {
		t.Error("a tampered value should not open")
	}
}

func TestKeyFingerprintIsStableAndDoesNotRevealTheKey(t *testing.T) {
	key, _ := NewKey()
	fp := KeyFingerprint(key)
	if fp != KeyFingerprint(key) || len(fp) != 16 {
		t.Errorf("fingerprint %q", fp)
	}
	other, _ := NewKey()
	if fp == KeyFingerprint(other) {
		t.Error("different keys should have different fingerprints")
	}
}

func TestNewKeyIsRandomAndTheRightSize(t *testing.T) {
	a, _ := NewKey()
	b, _ := NewKey()
	if len(a) != 32 || string(a) == string(b) {
		t.Errorf("keys: %d bytes, equal=%v", len(a), string(a) == string(b))
	}
}
