// Package crypto provides encryption and decryption utilities for sensitive data.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// KeySize is the required size for AES-256 keys (32 bytes)
	KeySize = 32
	// NonceSize is the size of GCM nonce (12 bytes)
	NonceSize = 12
	// VersionPrefix is the prefix for encrypted data
	VersionPrefix = "ENC[v%d]:"
)

var (
	ErrInvalidKey        = errors.New("invalid encryption key: must be 32 bytes")
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	ErrDecryptionFailed  = errors.New("decryption failed")
)

// Encryptor handles AES-256-GCM encryption and decryption.
type Encryptor struct {
	key     []byte
	version int
}

// NewEncryptor creates a new Encryptor with the given key.
// Key must be 32 bytes for AES-256.
func NewEncryptor(key []byte, version int) (*Encryptor, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}
	return &Encryptor{
		key:     key,
		version: version,
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns base64-encoded ciphertext with version prefix: ENC[v1]:base64(nonce+ciphertext)
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt: nonce + ciphertext (includes auth tag)
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Format: ENC[v1]:base64data
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return fmt.Sprintf(VersionPrefix, e.version) + encoded, nil
}

// Decrypt decrypts ciphertext encrypted by Encrypt.
// Expects format: ENC[vN]:base64data
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	// Parse version prefix
	if !strings.HasPrefix(ciphertext, "ENC[v") {
		return "", ErrInvalidCiphertext
	}

	// Find the colon separator
	colonIdx := strings.Index(ciphertext, "]:")
	if colonIdx == -1 {
		return "", ErrInvalidCiphertext
	}

	// Extract base64 data
	encoded := ciphertext[colonIdx+2:]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	if len(data) < NonceSize {
		return "", ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// Extract nonce and ciphertext
	nonce := data[:NonceSize]
	ciphertextBytes := data[NonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// GetVersion returns the key version used by this encryptor.
func (e *Encryptor) GetVersion() int {
	return e.version
}

// ParseVersion extracts the version number from an encrypted string.
// Returns 0 if the format is invalid.
func ParseVersion(ciphertext string) int {
	if !strings.HasPrefix(ciphertext, "ENC[v") {
		return 0
	}
	var version int
	_, err := fmt.Sscanf(ciphertext, "ENC[v%d]:", &version)
	if err != nil {
		return 0
	}
	return version
}
