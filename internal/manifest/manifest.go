package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
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
// Automatically migrates v0 manifests (bare array or single object) to the v1 schema.
// Does not fail; caller decides how to handle a missing or corrupted manifest.
func (l *Loader) Read() Manifest {
	data, err := os.ReadFile(l.ManifestPath)
	if err != nil {
		return nil
	}
	return migrate(data)
}

// migrate reads raw manifest JSON and returns the entries, migrating from any
// prior schema version to the current v1 schema.
//
// Schema detection order:
//  1. v1 — JSON object with a "version" field ≥ 1: parse directly.
//  2. v0 array — JSON array: wrap entries into v1.
//  3. v0 single-object — JSON object without "version": wrap the single entry.
//
// Returns nil if the data cannot be parsed at all.
func migrate(data []byte) Manifest {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}

	// v1: JSON object with explicit "version" field.
	if data[0] == '{' {
		var probe struct {
			Version int `json:"version"`
		}
		if json.Unmarshal(data, &probe) == nil && probe.Version >= 1 {
			var mf ManifestFile
			if err := json.Unmarshal(data, &mf); err != nil {
				return nil
			}
			return mf.Platforms
		}
	}

	// v0 array: bare JSON array of entries.
	if data[0] == '[' {
		var entries []ManifestEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			return entries
		}
		return nil
	}

	// v0 single object: a single ManifestEntry without version wrapper.
	if data[0] == '{' {
		var single ManifestEntry
		if err := json.Unmarshal(data, &single); err != nil {
			return nil
		}
		return []ManifestEntry{single}
	}

	return nil
}

// Write persists the manifest atomically (temp + rename) using the v1 schema.
// Returns an error if write fails (e.g., permission denied).
func (l *Loader) Write(m Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	mf := ManifestFile{
		Version:   schemaVersion,
		Platforms: m,
	}

	data, err := json.MarshalIndent(mf, "", "  ")
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

// UpsertEntry adds or replaces the entry for the given platform in the manifest.
// If an entry for the platform already exists it is replaced; otherwise a new entry
// is appended. The updated manifest is then written atomically.
func (l *Loader) UpsertEntry(entry ManifestEntry) error {
	entries := l.Read()
	if entries == nil {
		entries = []ManifestEntry{}
	}

	found := false
	for i, e := range entries {
		if e.Platform == entry.Platform {
			entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}

	return l.Write(entries)
}

// CanSkipBuild checks whether an entry matching the current platform's cache key
// already exists in the manifest and was successful. Returns true if rebuild can
// be skipped.
func (l *Loader) CanSkipBuild(currentKey *CacheKey) bool {
	entries := l.Read()
	if entries == nil {
		return false
	}

	for _, e := range entries {
		if e.Platform != currentKey.Platform {
			continue
		}
		if !e.Success {
			return false
		}
		entryKey := &CacheKey{
			GodotVersion:       e.GodotVersion,
			Platform:           e.Platform,
			ToolchainChecksums: e.ToolchainChecksums,
			ToolVersion:        e.ToolVersion,
		}
		return currentKey.Equals(entryKey)
	}

	return false
}

// BuildEntry constructs a ManifestEntry from the provided build outputs.
func BuildEntry(
	godotVersion string,
	versionResolutionMethod string,
	platform string,
	toolchainChecksums map[string]string,
	toolVersion string,
	success bool,
	templateReleaseHash string,
	templateDebugHash string,
) ManifestEntry {
	return ManifestEntry{
		GodotVersion:            godotVersion,
		VersionResolutionMethod: versionResolutionMethod,
		Platform:                platform,
		ToolchainChecksums:      toolchainChecksums,
		ToolVersion:             toolVersion,
		Timestamp:               time.Now().UTC(),
		Success:                 success,
		TemplateRelease:         templateReleaseHash,
		TemplateDebug:           templateDebugHash,
	}
}

// ComputeFileHash computes the SHA-256 hash of a file.
// Used to verify template binaries and toolchain artifacts.
func ComputeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
