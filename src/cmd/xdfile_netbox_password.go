package cmd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const (
	xdfileNetBoxKeyFileName                = "xdfile-netbox.key"
	xdfileNetBoxEncryptedPasswordPrefix    = "enc:v1:"
	xdfileNetBoxEncryptionKeySize          = 32
	xdfileNetBoxEncryptionNonceSize        = 12
	xdfileNetBoxEncryptionAssociatedString = "xdfile-netbox-password-v1"
)

func xdfileNetBoxMasterKeyPath() string {
	return filepath.Join(variable.XdfileMainDir, xdfileNetBoxKeyFileName)
}

func xdfileNetBoxLoadMasterKey() ([]byte, error) {
	key, err := xdfileNetBoxReadMasterKey(xdfileNetBoxMasterKeyPath())
	if err != nil {
		return nil, fmt.Errorf("read SSH password encryption key: %w", err)
	}
	return key, nil
}

func xdfileNetBoxLoadOrCreateMasterKey() ([]byte, error) {
	path := xdfileNetBoxMasterKeyPath()
	key, err := xdfileNetBoxReadMasterKey(path)
	if err == nil {
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read SSH password encryption key: %w", err)
	}

	key = make([]byte, xdfileNetBoxEncryptionKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate SSH password encryption key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return nil, fmt.Errorf("create SSH password key directory: %w", err)
	}
	encoded := base64.RawStdEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(encoded+"\n"), utils.ConfigFilePerm); err != nil {
		return nil, fmt.Errorf("write SSH password encryption key: %w", err)
	}
	return key, nil
}

func xdfileNetBoxReadMasterKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	keyText := strings.TrimSpace(string(data))
	key, err := base64.RawStdEncoding.DecodeString(keyText)
	if err != nil || len(key) != xdfileNetBoxEncryptionKeySize {
		return nil, fmt.Errorf("invalid SSH password encryption key")
	}
	return key, nil
}

func xdfileNetBoxEncryptPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	if xdfileNetBoxPasswordIsEncrypted(password) {
		return password, nil
	}
	key, err := xdfileNetBoxLoadOrCreateMasterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("initialize SSH password encryption: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("initialize SSH password encryption mode: %w", err)
	}

	nonce := make([]byte, xdfileNetBoxEncryptionNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate SSH password encryption nonce: %w", err)
	}
	payload := append([]byte(nil), nonce...)
	payload = gcm.Seal(payload, nonce, []byte(password), []byte(xdfileNetBoxEncryptionAssociatedString))
	return xdfileNetBoxEncryptedPasswordPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func xdfileNetBoxPasswordPlaintext(stored string) (string, bool, error) {
	if stored == "" {
		return "", false, nil
	}
	if !xdfileNetBoxPasswordIsEncrypted(stored) {
		return stored, false, nil
	}
	key, err := xdfileNetBoxLoadMasterKey()
	if err != nil {
		return "", true, err
	}
	payloadText := strings.TrimPrefix(stored, xdfileNetBoxEncryptedPasswordPrefix)
	payload, err := base64.RawURLEncoding.DecodeString(payloadText)
	if err != nil {
		return "", true, fmt.Errorf("decode encrypted SSH password: %w", err)
	}
	if len(payload) <= xdfileNetBoxEncryptionNonceSize {
		return "", true, fmt.Errorf("encrypted SSH password payload is too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", true, fmt.Errorf("initialize SSH password decryption: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", true, fmt.Errorf("initialize SSH password decryption mode: %w", err)
	}
	nonce := payload[:xdfileNetBoxEncryptionNonceSize]
	ciphertext := payload[xdfileNetBoxEncryptionNonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(xdfileNetBoxEncryptionAssociatedString))
	if err != nil {
		return "", true, fmt.Errorf("decrypt SSH password: %w", err)
	}
	return string(plain), true, nil
}

func xdfileNetBoxPasswordIsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), xdfileNetBoxEncryptedPasswordPrefix)
}

func xdfilePrepareNetBoxConnectionsForStorage(connections []xdfileNetBoxConnection) ([]xdfileNetBoxConnection, error) {
	normalized := xdfileNormalizeNetBoxConnections(connections)
	for i := range normalized {
		if normalized[i].Password == "" {
			continue
		}
		encrypted, err := xdfileNetBoxEncryptPassword(normalized[i].Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt SSH password for %s: %w", normalized[i].Name, err)
		}
		normalized[i].Password = encrypted
	}
	return normalized, nil
}

func xdfileMigrateNetBoxConnectionPasswords(connections []xdfileNetBoxConnection) ([]xdfileNetBoxConnection, bool, error) {
	migrated := false
	for i := range connections {
		stored := connections[i].Password
		if stored == "" {
			continue
		}
		password, encrypted, err := xdfileNetBoxPasswordPlaintext(stored)
		if err != nil {
			return nil, false, fmt.Errorf("decrypt SSH password for %s: %w", connections[i].Name, err)
		}
		if password != "" {
			xdfileSetNetBoxPassword(connections[i].Name, password)
		}
		if !encrypted {
			encryptedPassword, err := xdfileNetBoxEncryptPassword(password)
			if err != nil {
				return nil, false, fmt.Errorf("encrypt legacy SSH password for %s: %w", connections[i].Name, err)
			}
			connections[i].Password = encryptedPassword
			migrated = true
		}
	}
	return connections, migrated, nil
}
