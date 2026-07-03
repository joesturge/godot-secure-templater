package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/joemi/godot-secure-templater/internal"
)

// GenerateAES256Key generates a 256-bit (32-byte) AES key as a hex string.
func GenerateAES256Key() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// EnsureKey checks for an existing encryption key; if absent, generates and writes one.
func EnsureKey(keyPath string) (string, *internal.Error) {
	// Try to read existing key.
	if data, err := os.ReadFile(keyPath); err == nil {
		key := string(data)
		// Validate it looks like a hex string (64 chars for AES-256).
		if len(key) == 64 && isValidHex(key) {
			return key, nil
		}
		// File exists but is corrupt; treat as missing for now.
	}

	// Generate a new key.
	key, err := GenerateAES256Key()
	if err != nil {
		return "", &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to generate encryption key.",
			Details: err.Error(),
		}
	}

	// Write with restrictive permissions (owner-only on POSIX).
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return "", &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to create key directory.",
			Details: err.Error(),
		}
	}

	// Write atomically: temp + rename.
	tmpPath := keyPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(key), 0600); err != nil {
		return "", &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to write encryption key.",
			Details: err.Error(),
		}
	}

	if err := os.Rename(tmpPath, keyPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to finalize encryption key.",
			Details: err.Error(),
		}
	}

	return key, nil
}

// isValidHex checks if a string is valid hex.
func isValidHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}
