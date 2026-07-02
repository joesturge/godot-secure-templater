package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoaderRead(t *testing.T) {
	// GIVEN various manifest file states
	tests := []struct {
		name         string
		content      string
		shouldExist  bool
		wantNil      bool
		wantGodot    string
		wantPlatform string
		wantSuccess  bool
	}{
		{
			name: "valid manifest file",
			content: `{
	"godot_version": "4.3.0",
	"version_resolution_method": "explicit",
	"platform": "windows",
	"tool_version": "0.1.0",
	"success": true,
	"toolchain_checksums": {"python": "abc123"}
}`,
			shouldExist:  true,
			wantNil:      false,
			wantGodot:    "4.3.0",
			wantPlatform: "windows",
			wantSuccess:  true,
		},
		{
			name:        "missing manifest file",
			shouldExist: false,
			wantNil:     true,
		},
		{
			name:        "malformed JSON",
			content:     `{invalid json}`,
			shouldExist: true,
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a manifest file
			tmpDir := t.TempDir()
			manifestPath := filepath.Join(tmpDir, "manifest.json")

			if tt.shouldExist {
				err := os.WriteFile(manifestPath, []byte(tt.content), 0644)
				assert.NoError(t, err, "should write test manifest")
			}

			// WHEN reading the manifest
			loader := &Loader{ManifestPath: manifestPath}
			m := loader.Read()

			// THEN the result should match expectations
			if tt.wantNil {
				assert.Nil(t, m, "should return nil for missing/invalid manifest")
			} else {
				assert.NotNil(t, m, "should return manifest")
				assert.Equal(t, tt.wantGodot, m.GodotVersion)
				assert.Equal(t, tt.wantPlatform, m.Platform)
				assert.Equal(t, tt.wantSuccess, m.Success)
			}
		})
	}
}

func TestLoaderWrite(t *testing.T) {
	// GIVEN a manifest to write
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	manifest := &Manifest{
		GodotVersion:            "4.3.1",
		VersionResolutionMethod: "explicit",
		Platform:                "windows",
		ToolVersion:             "0.1.0",
		Success:                 true,
		ToolchainChecksums: map[string]string{
			"python": "abc123",
			"mingw":  "def456",
		},
		TemplateRelease: "hash_release",
		TemplateDebug:   "hash_debug",
	}

	// WHEN writing the manifest
	err := loader.Write(manifest)

	// THEN no error should occur
	assert.Nil(t, err, "Write should not error")

	// AND the file should exist
	_, statErr := os.Stat(manifestPath)
	assert.NoError(t, statErr, "Manifest file should exist")

	// AND reading it back should restore the data
	readBack := loader.Read()
	assert.NotNil(t, readBack)
	assert.Equal(t, "4.3.1", readBack.GodotVersion)
	assert.Equal(t, "windows", readBack.Platform)
	assert.Equal(t, true, readBack.Success)
	assert.Equal(t, "abc123", readBack.ToolchainChecksums["python"])

	// AND no temp file should be left behind
	tmpPath := manifestPath + ".tmp"
	_, tmpStatErr := os.Stat(tmpPath)
	assert.Error(t, tmpStatErr, "Temp file should be cleaned up")
}

func TestLoaderWriteNilManifest(t *testing.T) {
	// GIVEN a nil manifest
	tmpDir := t.TempDir()
	loader := &Loader{ManifestPath: filepath.Join(tmpDir, "manifest.json")}

	// WHEN writing nil
	err := loader.Write(nil)

	// THEN an error should occur
	assert.NotNil(t, err, "Write(nil) should error")
}

func TestLoaderAtomicWrite(t *testing.T) {
	// GIVEN a manifest to write
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	manifest := &Manifest{
		GodotVersion: "4.3.0",
		Platform:     "windows",
		Success:      true,
	}

	// WHEN writing the manifest (atomic write: temp + rename)
	err := loader.Write(manifest)

	// THEN no error should occur
	assert.Nil(t, err)

	// AND the main file should exist
	content, readErr := os.ReadFile(manifestPath)
	assert.NoError(t, readErr)

	// AND it should be valid JSON
	var m Manifest
	unmarshalErr := json.Unmarshal(content, &m)
	assert.NoError(t, unmarshalErr)

	// AND no temp file should remain
	tmpPath := manifestPath + ".tmp"
	_, tmpErr := os.Stat(tmpPath)
	assert.Error(t, tmpErr, "Temp file should be cleaned up")
}

func TestCacheKeyEquals(t *testing.T) {
	// GIVEN various cache key pairs
	tests := []struct {
		name   string
		key1   *CacheKey
		key2   *CacheKey
		wantEq bool
	}{
		{
			name: "identical keys",
			key1: &CacheKey{
				GodotVersion: "4.3.0",
				Platform:     "windows",
				ToolVersion:  "0.1.0",
				ToolchainChecksums: map[string]string{
					"python": "abc123",
					"mingw":  "def456",
				},
			},
			key2: &CacheKey{
				GodotVersion: "4.3.0",
				Platform:     "windows",
				ToolVersion:  "0.1.0",
				ToolchainChecksums: map[string]string{
					"python": "abc123",
					"mingw":  "def456",
				},
			},
			wantEq: true,
		},
		{
			name:   "different Godot version",
			key1:   &CacheKey{GodotVersion: "4.3.0", Platform: "windows", ToolVersion: "0.1.0"},
			key2:   &CacheKey{GodotVersion: "4.3.1", Platform: "windows", ToolVersion: "0.1.0"},
			wantEq: false,
		},
		{
			name:   "different platform",
			key1:   &CacheKey{GodotVersion: "4.3.0", Platform: "windows", ToolVersion: "0.1.0"},
			key2:   &CacheKey{GodotVersion: "4.3.0", Platform: "linux", ToolVersion: "0.1.0"},
			wantEq: false,
		},
		{
			name: "different toolchain checksums",
			key1: &CacheKey{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			key2: &CacheKey{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"python": "different"},
			},
			wantEq: false,
		},
		{
			name:   "one key is nil",
			key1:   &CacheKey{GodotVersion: "4.3.0"},
			key2:   nil,
			wantEq: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN comparing keys
			got := tt.key1.Equals(tt.key2)

			// THEN result should match expectation
			assert.Equal(t, tt.wantEq, got, "Equals() result mismatch")
		})
	}
}

func TestCanSkipBuild(t *testing.T) {
	// GIVEN various manifest states and cache keys
	tests := []struct {
		name         string
		manifestData *Manifest
		currentKey   *CacheKey
		wantSkip     bool
	}{
		{
			name: "matching cache key, successful build",
			manifestData: &Manifest{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				Success:            true,
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			currentKey: &CacheKey{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			wantSkip: true,
		},
		{
			name: "matching cache key, but build failed",
			manifestData: &Manifest{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				Success:            false,
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			currentKey: &CacheKey{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			wantSkip: false,
		},
		{
			name: "different Godot version",
			manifestData: &Manifest{
				GodotVersion:       "4.3.0",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				Success:            true,
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			currentKey: &CacheKey{
				GodotVersion:       "4.3.1",
				Platform:           "windows",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			wantSkip: false,
		},
		{
			name:         "no manifest file",
			manifestData: nil,
			currentKey: &CacheKey{
				GodotVersion: "4.3.0",
				Platform:     "windows",
			},
			wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a manifest file (or absence)
			tmpDir := t.TempDir()
			manifestPath := filepath.Join(tmpDir, "manifest.json")
			loader := &Loader{ManifestPath: manifestPath}

			if tt.manifestData != nil {
				data, _ := json.Marshal(tt.manifestData)
				_ = os.WriteFile(manifestPath, data, 0644)
			}

			// WHEN checking if build can be skipped
			got := loader.CanSkipBuild(tt.currentKey)

			// THEN result should match expectation
			assert.Equal(t, tt.wantSkip, got, "CanSkipBuild() result mismatch")
		})
	}
}

func TestComputeFileHash(t *testing.T) {
	// GIVEN a file with known content
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	testContent := []byte("hello world")

	err := os.WriteFile(filePath, testContent, 0644)
	assert.NoError(t, err, "should write test file")

	// WHEN computing its hash
	hash, hashErr := ComputeFileHash(filePath)

	// THEN no error should occur
	assert.Nil(t, hashErr, "ComputeFileHash should not error")

	// AND the hash should be a valid SHA-256 hex string
	assert.Len(t, hash, 64, "SHA-256 hash should be 64 hex chars")

	// AND the same file should always produce the same hash
	hash2, _ := ComputeFileHash(filePath)
	assert.Equal(t, hash, hash2, "Same file should have same hash")

	// AND the hash should match expected (known SHA-256 of "hello world")
	// SHA-256("hello world") = b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	assert.Equal(t, expected, hash, "Hash should match known value")
}

func TestComputeFileHashNonexistent(t *testing.T) {
	// GIVEN a nonexistent file
	// WHEN computing its hash
	hash, err := ComputeFileHash("/nonexistent/file.bin")

	// THEN an error should occur
	assert.NotNil(t, err, "Should error for nonexistent file")
	assert.Empty(t, hash, "Hash should be empty on error")
}

func TestManifestTimestamp(t *testing.T) {
	// GIVEN a manifest with a timestamp
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	now := time.Now()
	manifest := &Manifest{
		GodotVersion: "4.3.0",
		Platform:     "windows",
		Timestamp:    now,
		Success:      true,
	}

	// WHEN writing and reading the manifest
	err := loader.Write(manifest)
	assert.Nil(t, err)

	readBack := loader.Read()

	// THEN the timestamp should be preserved
	assert.NotNil(t, readBack)
	// Compare only up to seconds since JSON serialization loses nanosecond precision
	assert.True(t, readBack.Timestamp.Unix() == now.Unix(),
		"Timestamp should be preserved (comparing Unix timestamps)")
}
