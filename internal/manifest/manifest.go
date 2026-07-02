package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Loader handles reading and writing manifest files.
type Loader struct {
	// ManifestPath is the full path to manifest.json.
	ManifestPath string
}

// NewLoader creates a Loader for the given workspace root.
func NewLoader(workspaceRoot string) *Loader {
	return &Loader{
		ManifestPath: filepath.Join(workspaceRoot, "manifest.json"),
	}
}

// Read loads the manifest from disk. Returns nil if the file doesn't exist or is invalid.
// Does not fail; caller decides how to handle a missing or corrupted manifest.
func (l *Loader) Read() *Manifest {
	data, err := os.ReadFile(l.ManifestPath)
	if err != nil {
		return nil
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	return &m
}

// Write persists the manifest atomically (temp + rename).
// Returns an error if write fails (e.g., permission denied).
func (l *Loader) Write(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Atomic write: temp + rename
	tmpPath := l.ManifestPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp manifest: %w", err)
	}

	if err := os.Rename(tmpPath, l.ManifestPath); err != nil {
		// Best-effort cleanup
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename manifest: %w", err)
	}

	return nil
}

// CanSkipBuild checks if the current build inputs match the manifest's cache key
// and the build was successful. Returns true if rebuild can be skipped.
func (l *Loader) CanSkipBuild(currentKey *CacheKey) bool {
	m := l.Read()
	if m == nil || !m.Success {
		return false
	}

	manifestKey := &CacheKey{
		GodotVersion:       m.GodotVersion,
		Platform:           m.Platform,
		ToolchainChecksums: m.ToolchainChecksums,
		ToolVersion:        m.ToolVersion,
	}

	return currentKey.Equals(manifestKey)
}

// ComputeFileHash computes the SHA-256 hash of a file.
// Used to verify template binaries and toolchain artifacts.
func ComputeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
