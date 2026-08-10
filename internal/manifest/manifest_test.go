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
			name: "valid v1 manifest file",
			content: `{
	"version": 1,
	"platforms": [{
		"godot_version": "4.3.0",
		"version_resolution_method": "explicit",
		"platform": "windows",
		"tool_version": "0.1.0",
		"success": true,
		"toolchain_checksums": {"python": "abc123"}
	}]
}`,
			shouldExist:  true,
			wantNil:      false,
			wantGodot:    "4.3.0",
			wantPlatform: "windows",
			wantSuccess:  true,
		},
		{
			name: "legacy v0 array manifest is migrated to v1",
			content: `[{
	"godot_version": "4.3.0",
	"version_resolution_method": "explicit",
	"platform": "windows",
	"tool_version": "0.1.0",
	"success": true,
	"toolchain_checksums": {"python": "abc123"}
}]`,
			shouldExist:  true,
			wantNil:      false,
			wantGodot:    "4.3.0",
			wantPlatform: "windows",
			wantSuccess:  true,
		},
		{
			name: "legacy single-object manifest is migrated to v1",
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
			entries := loader.Read()

			// THEN the result should match expectations
			if tt.wantNil {
				assert.Nil(t, entries, "should return nil for missing/invalid manifest")
			} else {
				assert.NotNil(t, entries, "should return manifest entries")
				assert.Len(t, entries, 1, "should have one entry")
				assert.Equal(t, tt.wantGodot, entries[0].GodotVersion)
				assert.Equal(t, tt.wantPlatform, entries[0].Platform)
				assert.Equal(t, tt.wantSuccess, entries[0].Success)
			}
		})
	}
}

func TestLoaderWrite(t *testing.T) {
	// GIVEN a manifest to write
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	entries := Manifest{
		{
			GodotVersion:            "4.3.1",
			VersionResolutionMethod: "explicit",
			Platform:                "windows",
			ToolVersion:             "0.1.0",
			Success:                 true,
			ToolchainChecksums: map[string]string{
				"python": "abc123",
				"zig":    "def456",
			},
			TemplateRelease: "hash_release",
			TemplateDebug:   "hash_debug",
		},
	}

	// WHEN writing the manifest
	err := loader.Write(entries)

	// THEN no error should occur
	assert.Nil(t, err, "Write should not error")

	// AND the file should exist
	_, statErr := os.Stat(manifestPath)
	assert.NoError(t, statErr, "Manifest file should exist")

	// AND reading it back should restore the data
	readBack := loader.Read()
	assert.NotNil(t, readBack)
	assert.Len(t, readBack, 1, "should have one entry")
	assert.Equal(t, "4.3.1", readBack[0].GodotVersion)
	assert.Equal(t, "windows", readBack[0].Platform)
	assert.Equal(t, true, readBack[0].Success)
	assert.Equal(t, "abc123", readBack[0].ToolchainChecksums["python"])

	// AND the raw JSON should use the v1 schema
	raw, _ := os.ReadFile(manifestPath)
	var mf ManifestFile
	assert.NoError(t, json.Unmarshal(raw, &mf), "written file should be valid ManifestFile")
	assert.Equal(t, 1, mf.Version, "written manifest should carry version 1")
	assert.Len(t, mf.Platforms, 1, "platforms array should have one entry")

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

	entries := Manifest{
		{
			GodotVersion: "4.3.0",
			Platform:     "windows",
			Success:      true,
		},
	}

	// WHEN writing the manifest (atomic write: temp + rename)
	err := loader.Write(entries)

	// THEN no error should occur
	assert.Nil(t, err)

	// AND the main file should exist
	content, readErr := os.ReadFile(manifestPath)
	assert.NoError(t, readErr)

	// AND it should be valid v1 JSON object
	var m ManifestFile
	unmarshalErr := json.Unmarshal(content, &m)
	assert.NoError(t, unmarshalErr)
	assert.Equal(t, 1, m.Version, "written manifest should have version 1")

	// AND no temp file should remain
	tmpPath := manifestPath + ".tmp"
	_, tmpErr := os.Stat(tmpPath)
	assert.Error(t, tmpErr, "Temp file should be cleaned up")
}

func TestUpsertEntryNewPlatform(t *testing.T) {
	// GIVEN a manifest with a windows entry
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	initial := Manifest{
		{GodotVersion: "4.3.0", Platform: "windows", ToolVersion: "0.1.0", Success: true},
	}
	err := loader.Write(initial)
	assert.Nil(t, err, "initial write should not error")

	// WHEN upserting a new web entry
	webEntry := ManifestEntry{
		GodotVersion: "4.3.0",
		Platform:     "web",
		ToolVersion:  "0.1.0",
		Success:      true,
	}
	upsertErr := loader.UpsertEntry(webEntry)

	// THEN no error should occur
	assert.Nil(t, upsertErr, "UpsertEntry should not error")

	// AND the manifest should contain both entries
	readBack := loader.Read()
	assert.Len(t, readBack, 2, "should have two entries")

	platforms := map[string]bool{}
	for _, e := range readBack {
		platforms[e.Platform] = true
	}
	assert.True(t, platforms["windows"], "windows entry should be present")
	assert.True(t, platforms["web"], "web entry should be present")
}

func TestUpsertEntryReplacesPlatform(t *testing.T) {
	// GIVEN a manifest with a windows entry with Success=false
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	initial := Manifest{
		{GodotVersion: "4.3.0", Platform: "windows", ToolVersion: "0.1.0", Success: false},
	}
	err := loader.Write(initial)
	assert.Nil(t, err, "initial write should not error")

	// WHEN upserting a new windows entry with Success=true
	updated := ManifestEntry{
		GodotVersion: "4.3.0",
		Platform:     "windows",
		ToolVersion:  "0.1.0",
		Success:      true,
	}
	upsertErr := loader.UpsertEntry(updated)

	// THEN no error should occur
	assert.Nil(t, upsertErr, "UpsertEntry should not error")

	// AND the manifest should still have only one entry
	readBack := loader.Read()
	assert.Len(t, readBack, 1, "should still have one entry")

	// AND the entry should be updated
	assert.True(t, readBack[0].Success, "entry should be updated to Success=true")
}

func TestUpsertEntryCreatesManifestFromScratch(t *testing.T) {
	// GIVEN no manifest file exists
	tmpDir := t.TempDir()
	loader := &Loader{ManifestPath: filepath.Join(tmpDir, "manifest.json")}

	// WHEN upserting an entry
	entry := ManifestEntry{GodotVersion: "4.3.0", Platform: "windows", Success: true}
	err := loader.UpsertEntry(entry)

	// THEN no error should occur
	assert.Nil(t, err, "UpsertEntry should not error")

	// AND the manifest should contain the entry
	readBack := loader.Read()
	assert.Len(t, readBack, 1, "should have one entry")
	assert.Equal(t, "windows", readBack[0].Platform)
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
					"zig":    "def456",
				},
			},
			key2: &CacheKey{
				GodotVersion: "4.3.0",
				Platform:     "windows",
				ToolVersion:  "0.1.0",
				ToolchainChecksums: map[string]string{
					"python": "abc123",
					"zig":    "def456",
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
		{
			name: "legacy placeholder godot checksum equals empty checksum",
			key1: &CacheKey{
				GodotVersion:       "4.6.3",
				Platform:           "windows",
				ToolVersion:        "dev",
				ToolchainChecksums: map[string]string{"godot_source": "placeholder_godot_4.6.3", "python": "abc"},
			},
			key2: &CacheKey{
				GodotVersion:       "4.6.3",
				Platform:           "windows",
				ToolVersion:        "dev",
				ToolchainChecksums: map[string]string{"godot_source": "", "python": "abc"},
			},
			wantEq: true,
		},
		{
			name: "godot source hash equals empty checksum",
			key1: &CacheKey{
				GodotVersion:       "4.6.3",
				Platform:           "windows",
				ToolVersion:        "dev",
				ToolchainChecksums: map[string]string{"godot_source": "fa22b5f974125057087c9ef725eae582dbc5e39385dc377e8d5dbc295b367e1c", "python": "abc"},
			},
			key2: &CacheKey{
				GodotVersion:       "4.6.3",
				Platform:           "windows",
				ToolVersion:        "dev",
				ToolchainChecksums: map[string]string{"godot_source": "", "python": "abc"},
			},
			wantEq: true,
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
		manifestData Manifest
		currentKey   *CacheKey
		wantSkip     bool
	}{
		{
			name: "matching cache key, successful build",
			manifestData: Manifest{
				{
					GodotVersion:       "4.3.0",
					Platform:           "windows",
					ToolVersion:        "0.1.0",
					Success:            true,
					ToolchainChecksums: map[string]string{"python": "abc123"},
				},
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
			manifestData: Manifest{
				{
					GodotVersion:       "4.3.0",
					Platform:           "windows",
					ToolVersion:        "0.1.0",
					Success:            false,
					ToolchainChecksums: map[string]string{"python": "abc123"},
				},
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
			manifestData: Manifest{
				{
					GodotVersion:       "4.3.0",
					Platform:           "windows",
					ToolVersion:        "0.1.0",
					Success:            true,
					ToolchainChecksums: map[string]string{"python": "abc123"},
				},
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
		{
			name: "platform not present in manifest",
			manifestData: Manifest{
				{
					GodotVersion:       "4.3.0",
					Platform:           "windows",
					ToolVersion:        "0.1.0",
					Success:            true,
					ToolchainChecksums: map[string]string{"python": "abc123"},
				},
			},
			currentKey: &CacheKey{
				GodotVersion:       "4.3.0",
				Platform:           "web",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"python": "abc123"},
			},
			wantSkip: false,
		},
		{
			name: "multi-platform manifest: target platform matches",
			manifestData: Manifest{
				{
					GodotVersion:       "4.3.0",
					Platform:           "windows",
					ToolVersion:        "0.1.0",
					Success:            true,
					ToolchainChecksums: map[string]string{"python": "abc123"},
				},
				{
					GodotVersion:       "4.3.0",
					Platform:           "web",
					ToolVersion:        "0.1.0",
					Success:            true,
					ToolchainChecksums: map[string]string{"emcc": "def456"},
				},
			},
			currentKey: &CacheKey{
				GodotVersion:       "4.3.0",
				Platform:           "web",
				ToolVersion:        "0.1.0",
				ToolchainChecksums: map[string]string{"emcc": "def456"},
			},
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a manifest file (or absence)
			tmpDir := t.TempDir()
			manifestPath := filepath.Join(tmpDir, "manifest.json")
			loader := &Loader{ManifestPath: manifestPath}

			if tt.manifestData != nil {
				mf := ManifestFile{Version: 1, Platforms: tt.manifestData}
				data, _ := json.Marshal(mf)
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
	// GIVEN a manifest entry with a timestamp
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	loader := &Loader{ManifestPath: manifestPath}

	now := time.Now()
	entries := Manifest{
		{
			GodotVersion: "4.3.0",
			Platform:     "windows",
			Timestamp:    now,
			Success:      true,
		},
	}

	// WHEN writing and reading the manifest
	err := loader.Write(entries)
	assert.Nil(t, err)

	readBack := loader.Read()

	// THEN the timestamp should be preserved
	assert.NotNil(t, readBack)
	assert.Len(t, readBack, 1, "should have one entry")
	// Compare only up to seconds since JSON serialization loses nanosecond precision
	assert.True(t, readBack[0].Timestamp.Unix() == now.Unix(),
		"Timestamp should be preserved (comparing Unix timestamps)")
}
