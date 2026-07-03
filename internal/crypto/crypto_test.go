package crypto

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAES256Key(t *testing.T) {
	// GIVEN multiple invocations of key generation
	tests := []struct {
		name string
	}{
		{"generates valid key"},
		{"generates different keys each time"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN generating an AES-256 key
			key, err := GenerateAES256Key()

			// THEN no error should occur
			assert.NoError(t, err, "GenerateAES256Key should not error")

			// AND it should be 64 characters (32 bytes as hex)
			assert.Len(t, key, 64, "Key should be 64 characters")

			// AND it should be valid hex
			decoded, err := hex.DecodeString(key)
			assert.NoError(t, err, "Key should be valid hex")

			// AND when decoded it should be 32 bytes
			assert.Len(t, decoded, 32, "Decoded key should be 32 bytes")
		})
	}
}

func TestGenerateAES256KeyUniqueness(t *testing.T) {
	// GIVEN two separate key generation calls
	key1, _ := GenerateAES256Key()
	key2, _ := GenerateAES256Key()

	// THEN the keys should be unique
	assert.NotEqual(t, key1, key2, "Generated keys should be unique")
}

func TestEnsureKeyGeneratesNewKey(t *testing.T) {
	// GIVEN a non-existent key file path
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "encryption.key")

	// WHEN ensuring a key exists
	key1, err := EnsureKey(keyPath)

	// THEN no error should occur
	assert.Nil(t, err, "EnsureKey should not error")
	// AND a key should be generated
	assert.Len(t, key1, 64, "Key should be 64 characters")

	// AND the file should exist with correct permissions
	info, statErr := os.Stat(keyPath)
	assert.NoError(t, statErr, "Key file should exist")
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "Key file should have 0600 permissions")

	// AND the file content should match the returned key
	data, readErr := os.ReadFile(keyPath)
	assert.NoError(t, readErr, "Should read key file")
	assert.Equal(t, key1, string(data), "Key file content should match returned key")
}

func TestEnsureKeyReusesExistingKey(t *testing.T) {
	// GIVEN an existing key file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "encryption.key")
	key1, _ := EnsureKey(keyPath)

	// WHEN ensuring the key exists again
	key2, err := EnsureKey(keyPath)

	// THEN no error should occur
	assert.Nil(t, err, "Second EnsureKey call should not error")
	// AND the same key should be returned
	assert.Equal(t, key1, key2, "EnsureKey should return the same key on reuse")
}

func TestEnsureKeyWithNonexistentDirectory(t *testing.T) {
	// GIVEN a key path with non-existent parent directories
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "subdir", "encryption.key")

	// WHEN ensuring the key exists
	key, err := EnsureKey(keyPath)

	// THEN no error should occur
	assert.Nil(t, err, "EnsureKey should create parent directory")
	// AND a key should be generated
	assert.Len(t, key, 64, "Key should be 64 characters")

	// AND the file should exist
	_, statErr := os.Stat(keyPath)
	assert.NoError(t, statErr, "Key file should exist")
}

func TestEnsureKeyInvalidExistingKey(t *testing.T) {
	// GIVEN an invalid key file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "encryption.key")
	writeErr := os.WriteFile(keyPath, []byte("not-a-valid-hex-key"), 0644)
	assert.NoError(t, writeErr, "Should write the invalid key fixture")

	// WHEN ensuring the key exists
	key, err := EnsureKey(keyPath)

	// THEN no error should occur
	assert.Nil(t, err, "EnsureKey should generate new key for invalid existing key")
	// AND a new valid key should be generated
	assert.Len(t, key, 64, "Key should be 64 characters")

	// AND it should be valid hex
	_, decodeErr := hex.DecodeString(key)
	assert.NoError(t, decodeErr, "Generated key should be valid hex")
}
