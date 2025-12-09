package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Generate a test key (32 bytes)
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewEncryptor(key, 1)
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"short", "hello"},
		{"api_key", "abc123XYZ789"},
		{"long", "this is a very long string that represents an API secret key from Binance exchange"},
		{"unicode", "中文測試 🔐"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Check format
			if !hasVersionPrefix(ciphertext) {
				t.Errorf("ciphertext missing version prefix: %s", ciphertext)
			}

			// Decrypt
			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("decrypted = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptDifferentCiphertexts(t *testing.T) {
	key := make([]byte, KeySize)
	enc, _ := NewEncryptor(key, 1)

	plaintext := "same-api-key"
	c1, _ := enc.Encrypt(plaintext)
	c2, _ := enc.Encrypt(plaintext)

	// Each encryption should produce different ciphertext (due to random nonce)
	if c1 == c2 {
		t.Error("expected different ciphertexts for same plaintext")
	}
}

func TestInvalidKey(t *testing.T) {
	_, err := NewEncryptor([]byte("short"), 1)
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	key := make([]byte, KeySize)
	enc, _ := NewEncryptor(key, 1)

	invalids := []string{
		"",
		"not-encrypted",
		"ENC[v1]:",           // empty data
		"ENC[v1]:!!!invalid", // invalid base64
	}

	for _, invalid := range invalids {
		_, err := enc.Decrypt(invalid)
		if err == nil {
			t.Errorf("expected error for invalid ciphertext: %s", invalid)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		ciphertext string
		expected   int
	}{
		{"ENC[v1]:data", 1},
		{"ENC[v2]:data", 2},
		{"ENC[v10]:data", 10},
		{"invalid", 0},
		{"ENC[vX]:data", 0},
	}

	for _, tt := range tests {
		got := ParseVersion(tt.ciphertext)
		if got != tt.expected {
			t.Errorf("ParseVersion(%q) = %d, want %d", tt.ciphertext, got, tt.expected)
		}
	}
}

func hasVersionPrefix(s string) bool {
	return len(s) > 8 && s[:5] == "ENC[v"
}
