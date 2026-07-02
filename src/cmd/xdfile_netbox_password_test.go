package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	variable "github.com/s0x401/xdfile-manager/src/config"
)

func TestXdfileNetBoxSavedPasswordIsEncrypted(t *testing.T) {
	restore := isolateNetBoxPasswordTest(t)
	defer restore()

	path := filepath.Join(variable.XdfileMainDir, xdfileNetBoxFileName)
	connections := []xdfileNetBoxConnection{{
		Name:     "prod",
		Host:     "example.com",
		User:     "deploy",
		Port:     22,
		Password: "super-secret",
	}}

	if err := xdfileSaveNetBoxPrefs(path, connections); err != nil {
		t.Fatalf("save netbox prefs: %v", err)
	}

	data := mustReadFileForTest(t, path)
	if strings.Contains(string(data), "super-secret") {
		t.Fatalf("saved netbox prefs must not contain plaintext password:\n%s", string(data))
	}
	if !strings.Contains(string(data), xdfileNetBoxEncryptedPasswordPrefix) {
		t.Fatalf("saved netbox prefs should contain encrypted password token:\n%s", string(data))
	}
	if _, err := os.Stat(xdfileNetBoxMasterKeyPath()); err != nil {
		t.Fatalf("expected netbox encryption key file: %v", err)
	}

	xdfileNetBoxPasswordCache = map[string]string{}
	loaded, err := xdfileLoadNetBoxPrefs(path)
	if err != nil {
		t.Fatalf("load encrypted netbox prefs: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded connection count = %d, want 1", len(loaded))
	}
	if !xdfileNetBoxPasswordIsEncrypted(loaded[0].Password) {
		t.Fatalf("loaded stored password should remain encrypted, got %q", loaded[0].Password)
	}
	if got := loaded[0].passwordForAuth(); got != "super-secret" {
		t.Fatalf("decrypted password = %q, want super-secret", got)
	}
}

func TestXdfileNetBoxLegacyPlaintextPasswordMigratesToEncrypted(t *testing.T) {
	restore := isolateNetBoxPasswordTest(t)
	defer restore()

	path := filepath.Join(variable.XdfileMainDir, xdfileNetBoxFileName)
	legacy := xdfileNetBoxPrefs{
		Connections: []xdfileNetBoxConnection{{
			Name:     "legacy",
			Host:     "legacy.example.com",
			User:     "root",
			Port:     22,
			Password: "legacy-secret",
		}},
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("encode legacy prefs: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir legacy prefs: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy prefs: %v", err)
	}

	loaded, err := xdfileLoadNetBoxPrefs(path)
	if err != nil {
		t.Fatalf("load legacy netbox prefs: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded connection count = %d, want 1", len(loaded))
	}
	if got := loaded[0].passwordForAuth(); got != "legacy-secret" {
		t.Fatalf("legacy password auth = %q, want legacy-secret", got)
	}

	migrated := mustReadFileForTest(t, path)
	if strings.Contains(string(migrated), "legacy-secret") {
		t.Fatalf("migrated netbox prefs must not contain plaintext password:\n%s", string(migrated))
	}
	if !strings.Contains(string(migrated), xdfileNetBoxEncryptedPasswordPrefix) {
		t.Fatalf("migrated netbox prefs should contain encrypted password token:\n%s", string(migrated))
	}
}

func isolateNetBoxPasswordTest(t *testing.T) func() {
	t.Helper()
	originalMainDir := variable.XdfileMainDir
	originalPasswordCache := xdfileNetBoxPasswordCache
	variable.XdfileMainDir = filepath.Join(t.TempDir(), "xdfile-data")
	xdfileNetBoxPasswordCache = map[string]string{}
	return func() {
		variable.XdfileMainDir = originalMainDir
		xdfileNetBoxPasswordCache = originalPasswordCache
	}
}

func mustReadFileForTest(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
