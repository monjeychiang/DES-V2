package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	ErrKeyNotFound    = errors.New("encryption key not found")
	ErrKeyNotLoaded   = errors.New("key manager not initialized")
	ErrVersionMissing = errors.New("key version not configured")
)

// KeyManager manages encryption keys for multiple versions.
// Supports key rotation by maintaining multiple key versions.
type KeyManager struct {
	mu           sync.RWMutex
	currentVer   int
	encryptors   map[int]*Encryptor
	envKeyPrefix string
}

// NewKeyManager creates a new KeyManager and loads keys from environment variables.
// Environment variables should follow the pattern:
//   - MASTER_ENCRYPTION_KEY (version 1)
//   - MASTER_ENCRYPTION_KEY_V2 (version 2)
//   - etc.
func NewKeyManager() (*KeyManager, error) {
	km := &KeyManager{
		encryptors:   make(map[int]*Encryptor),
		envKeyPrefix: "MASTER_ENCRYPTION_KEY",
	}

	// Load version 1 (required)
	if err := km.loadKey(1, km.envKeyPrefix); err != nil {
		return nil, fmt.Errorf("load primary key: %w", err)
	}
	km.currentVer = 1

	// Load additional versions (optional)
	for v := 2; v <= 10; v++ {
		envName := fmt.Sprintf("%s_V%d", km.envKeyPrefix, v)
		if err := km.loadKey(v, envName); err == nil {
			km.currentVer = v // Use latest available version
		}
	}

	return km, nil
}

// loadKey loads a single key from environment variable.
func (km *KeyManager) loadKey(version int, envName string) error {
	keyBase64 := os.Getenv(envName)
	if keyBase64 == "" {
		return ErrKeyNotFound
	}

	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return fmt.Errorf("decode key %s: %w", envName, err)
	}

	enc, err := NewEncryptor(key, version)
	if err != nil {
		return fmt.Errorf("create encryptor v%d: %w", version, err)
	}

	km.encryptors[version] = enc
	return nil
}

// Encrypt encrypts plaintext using the current (latest) key version.
func (km *KeyManager) Encrypt(plaintext string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	enc, ok := km.encryptors[km.currentVer]
	if !ok {
		return "", ErrKeyNotLoaded
	}

	return enc.Encrypt(plaintext)
}

// Decrypt decrypts ciphertext, automatically selecting the correct key version.
func (km *KeyManager) Decrypt(ciphertext string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	// Parse version from ciphertext
	version := ParseVersion(ciphertext)
	if version == 0 {
		return "", ErrInvalidCiphertext
	}

	enc, ok := km.encryptors[version]
	if !ok {
		return "", fmt.Errorf("key version %d not available", version)
	}

	return enc.Decrypt(ciphertext)
}

// ReEncrypt re-encrypts a ciphertext with the current key version.
// Useful for key rotation - decrypt with old key, encrypt with new key.
func (km *KeyManager) ReEncrypt(ciphertext string) (string, error) {
	plaintext, err := km.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt for re-encryption: %w", err)
	}
	return km.Encrypt(plaintext)
}

// CurrentVersion returns the current (latest) key version being used.
func (km *KeyManager) CurrentVersion() int {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentVer
}

// HasVersion checks if a specific key version is loaded.
func (km *KeyManager) HasVersion(version int) bool {
	km.mu.RLock()
	defer km.mu.RUnlock()
	_, ok := km.encryptors[version]
	return ok
}

// GenerateKey generates a new random 32-byte key suitable for AES-256.
// Returns the key as a base64-encoded string for easy storage.
func GenerateKey() (string, error) {
	key := make([]byte, KeySize)
	if _, err := cryptoRandRead(key); err != nil {
		return "", fmt.Errorf("generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// cryptoRandRead is a variable for testing purposes
var cryptoRandRead = func(b []byte) (int, error) {
	return rand.Reader.Read(b)
}
