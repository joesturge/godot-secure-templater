package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errInvalidManifest = errors.New("invalid manifest")

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
	entries, err := l.readEntries()
	if err != nil {
		return nil
	}
	return entries
}

func (l *Loader) readEntries() (Manifest, error) {
	data, err := os.ReadFile(l.ManifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return migrate(data)
}

// migrate reads raw manifest JSON and returns the entries, migrating from any
// prior schema version to the current v1 schema.
//
// Schema detection order:
//  1. v1 — JSON object with "version" == 1: parse directly.
//  2. v0 array — JSON array: wrap entries into v1.
//  3. v0 single-object — JSON object with no "version" key: wrap the single entry.
//
// An object with an unrecognised "version" (e.g. a future v2 on an older binary)
// is rejected and returns nil rather than silently producing bad data.
//
// Returns nil if the data cannot be parsed at all.
func migrate(data []byte) (Manifest, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errInvalidManifest
	}

	// JSON object: inspect the "version" key to distinguish v0-object from v1+.
	if data[0] == '{' {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(data, &probe); err != nil {
			return nil, errInvalidManifest
		}

		if rawVersion, ok := probe["version"]; ok {
			// Version key is present: this is v1 or newer.
			if bytes.Equal(bytes.TrimSpace(rawVersion), []byte("null")) {
				return nil, errInvalidManifest
			}

			var version int
			if err := json.Unmarshal(rawVersion, &version); err != nil {
				return nil, errInvalidManifest
			}
			if version != schemaVersion {
				// Unknown future version — fail safe rather than silently mangle.
				return nil, errInvalidManifest
			}

			rawPlatforms, ok := probe["platforms"]
			if !ok || bytes.Equal(bytes.TrimSpace(rawPlatforms), []byte("null")) {
				// v1 must have a "platforms" array.
				return nil, errInvalidManifest
			}

			var mf ManifestFile
			if err := json.Unmarshal(data, &mf); err != nil {
				return nil, errInvalidManifest
			}
			return mf.Platforms, nil
		}

		// No "version" key: treat as a v0 single ManifestEntry.
		var single ManifestEntry
		if err := json.Unmarshal(data, &single); err != nil {
			return nil, errInvalidManifest
		}
		if strings.TrimSpace(single.Platform) == "" {
			return nil, errInvalidManifest
		}
		return []ManifestEntry{single}, nil
	}

	// v0 array: bare JSON array of entries.
	if data[0] == '[' {
		var entries []ManifestEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			return entries, nil
		}
		return nil, errInvalidManifest
	}

	return nil, errInvalidManifest
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
	entries, err := l.readEntries()
	if err != nil {
		return err
	}
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
	entries, err := l.readEntries()
	if err != nil || entries == nil {
		return false
	}

	for _, e := range entries {
		if e.Platform != currentKey.Platform {
			continue
		}
		if !e.Success {
			// Skip failed entries; a later entry for the same platform may be successful.
			continue
		}
		entryKey := &CacheKey{
			GodotVersion:       e.GodotVersion,
			Platform:           e.Platform,
			ToolchainChecksums: e.ToolchainChecksums,
			ToolVersion:        e.ToolVersion,
		}
		if currentKey.Equals(entryKey) {
			return true
		}
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
