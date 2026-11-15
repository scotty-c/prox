package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/scotty-c/prox/pkg/crypto"
	"github.com/zalando/go-keyring"
)

// sandbox gives the test its own home, config directory and in-memory keychain
func sandbox(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv(EnvStore, "")
	keyring.MockInit()
	allowAutoInTests = false
	t.Cleanup(func() { allowAutoInTests = false })
}

var creds = Credentials{Username: "root@pam", Password: "p@ss=w0rd=="}

func TestKeyFileStoreSealsTheCredentials(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "file")

	sealed, err := Seal("lab", creds)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Method != MethodKeyFile || sealed.Fingerprint == "" {
		t.Errorf("method %q, fingerprint %q", sealed.Method, sealed.Fingerprint)
	}
	for k, v := range sealed.Entries {
		if !crypto.IsSealed(v) || strings.Contains(v, "root") || strings.Contains(v, "w0rd") {
			t.Errorf("entry %s = %q should be sealed", k, v)
		}
	}

	got, method, err := Open("lab", sealed.Entries)
	if err != nil || got != creds || method != MethodKeyFile {
		t.Errorf("Open = %+v, %q, %v", got, method, err)
	}
}

func TestKeyFileIsPrivateSeparateFromProfilesAndReused(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "file")

	first, _ := Seal("a", creds)
	second, _ := Seal("b", Credentials{Username: "x", Password: "y"})
	if first.Fingerprint != second.Fingerprint {
		t.Error("every profile shares the one key")
	}

	path, _ := KeyFilePath()
	if strings.Contains(path, string(filepath.Separator)+".prox"+string(filepath.Separator)) {
		t.Errorf("the key should not live beside the profiles: %s", path)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("key file mode = %v, %v; want 0600", info.Mode().Perm(), err)
		}
		dir, _ := os.Stat(filepath.Dir(path))
		if dir.Mode().Perm() != 0o700 {
			t.Errorf("key directory mode = %v, want 0700", dir.Mode().Perm())
		}
	}
}

func TestKeychainStoreKeepsNoSecretInTheEntries(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "keychain")

	sealed, err := Seal("prod", creds)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Method != MethodKeychain || len(sealed.Entries) != 1 || sealed.Entries["credentials"] != "keychain" || sealed.Fingerprint != "" {
		t.Errorf("sealed = %+v", sealed)
	}
	got, method, err := Open("prod", sealed.Entries)
	if err != nil || got != creds || method != MethodKeychain {
		t.Errorf("Open = %+v, %q, %v", got, method, err)
	}
	// and no key file was needed
	if path, _ := KeyFilePath(); fileExists(path) {
		t.Errorf("the keychain store should not create a key file")
	}
}

func TestForgetRemovesTheKeychainItemOnlyWhenTheProfileHasOne(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "keychain")
	sealed, _ := Seal("prod", creds)

	if err := Forget("prod", sealed.Entries); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open("prod", sealed.Entries); err == nil || !strings.Contains(err.Error(), "not in the keychain") || !strings.Contains(err.Error(), "prox config create prod") {
		t.Errorf("after Forget: %v", err)
	}
	// a profile that does not use the keychain never touches it
	if err := Forget("other", map[string]string{"username": "enc:v2:x"}); err != nil {
		t.Errorf("Forget on a key-file profile: %v", err)
	}
}

func TestAutomaticModePrefersTheKeychain(t *testing.T) {
	sandbox(t)
	allowAutoInTests = true
	sealed, err := Seal("lab", creds)
	if err != nil || sealed.Method != MethodKeychain || sealed.Note != "" {
		t.Errorf("sealed = %+v, %v", sealed, err)
	}
}

func TestAutomaticModeFallsBackToTheKeyFileAndSaysWhy(t *testing.T) {
	sandbox(t)
	allowAutoInTests = true
	keyring.MockInitWithError(errors.New("no secret service on this machine"))

	sealed, err := Seal("lab", creds)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Method != MethodKeyFile || !strings.Contains(sealed.Note, "No usable OS keychain") || !strings.Contains(sealed.Note, "no secret service on this machine") {
		t.Errorf("sealed = %+v", sealed)
	}
	if got, _, err := Open("lab", sealed.Entries); err != nil || got != creds {
		t.Errorf("the fallback must be readable: %+v, %v", got, err)
	}
}

func TestAskingForTheKeychainWhenThereIsNoneIsAnError(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "keychain")
	keyring.MockInitWithError(errors.New("locked"))
	if _, err := Seal("lab", creds); err == nil || !strings.Contains(err.Error(), "could not store the credentials in the keychain") {
		t.Errorf("got %v; an explicit request must not silently use something else", err)
	}
}

func TestTestBinariesNeverUseTheRealKeychainByDefault(t *testing.T) {
	sandbox(t)
	// no EnvStore, no allowAutoInTests: what a stray test would get
	sealed, err := Seal("lab", creds)
	if err != nil || sealed.Method != MethodKeyFile {
		t.Errorf("sealed = %+v, %v; want the key file", sealed, err)
	}
}

func TestInvalidStoreSettingIsRejected(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "vault")
	if _, err := Seal("lab", creds); err == nil || !strings.Contains(err.Error(), "use auto, keychain or file") {
		t.Errorf("got %v", err)
	}
}

func TestOldProfilesStillOpen(t *testing.T) {
	sandbox(t)
	derivedUser, _ := crypto.Encrypt("root@pam")
	derivedPass, _ := crypto.Encrypt("old-secret")

	got, method, err := Open("old", map[string]string{"username": derivedUser, "password": derivedPass})
	if err != nil || got != (Credentials{"root@pam", "old-secret"}) || method != MethodLegacy {
		t.Errorf("legacy: %+v, %q, %v", got, method, err)
	}

	got, method, err = Open("plain", map[string]string{"username": "root@pam", "password": "short pw"})
	if err != nil || got != (Credentials{"root@pam", "short pw"}) || method != MethodPlaintext {
		t.Errorf("plaintext: %+v, %q, %v", got, method, err)
	}

	got, method, err = Open("empty", map[string]string{})
	if err != nil || got != (Credentials{}) || method.Current() {
		t.Errorf("empty profile: %+v, %q, %v", got, method, err)
	}
}

func TestAMissingKeyFileIsReportedNotRecreated(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "file")
	sealed, _ := Seal("lab", creds)
	path, _ := KeyFilePath()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	_, _, err := Open("lab", sealed.Entries)
	if err == nil || !strings.Contains(err.Error(), "missing") || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "prox config create lab") {
		t.Errorf("got %v", err)
	}
	if fileExists(path) {
		t.Error("opening must never create a key: a new one could not open the old values")
	}
}

func TestAReplacedKeyFileIsReportedWithItsFingerprint(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "file")
	sealed, _ := Seal("lab", creds)
	path, _ := KeyFilePath()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := createKeyFile(path); err != nil { // a different key in the same place
		t.Fatal(err)
	}

	_, _, err := Open("lab", sealed.Entries)
	if err == nil || !strings.Contains(err.Error(), "does not match") || !strings.Contains(err.Error(), "fingerprint") {
		t.Errorf("got %v", err)
	}
}

func TestAKeyFileOtherUsersCanReadIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not apply")
	}
	sandbox(t)
	t.Setenv(EnvStore, "file")
	sealed, _ := Seal("lab", creds)
	path, _ := KeyFilePath()
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Open("lab", sealed.Entries)
	if err == nil || !strings.Contains(err.Error(), "can be read by other users") || !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("got %v", err)
	}
}

func TestADamagedKeyFileIsReported(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvStore, "file")
	sealed, _ := Seal("lab", creds)
	path, _ := KeyFilePath()
	if err := os.WriteFile(path, []byte("not a key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open("lab", sealed.Entries); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("got %v", err)
	}
}

func TestCreatingAKeyNeverOverwritesOne(t *testing.T) {
	sandbox(t)
	path, _ := KeyFilePath()
	if err := createKeyFile(path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := createKeyFile(path); !errors.Is(err, os.ErrExist) {
		t.Errorf("second create: %v, want ErrExist", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("the existing key was overwritten")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
