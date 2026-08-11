package manifest

import (
	"strings"
	"time"
)

// schemaVersion is the current manifest file format version.
const schemaVersion = 1

// ManifestFile is the versioned on-disk representation of manifest.json.
// Version 1 introduced an explicit schema version and renamed the top-level
// array to "platforms".
type ManifestFile struct {
	// Version identifies the manifest schema. The current version is 1.
	Version int `json:"version"`

	// Platforms holds one entry per target platform build.
	Platforms []ManifestEntry `json:"platforms"`
}

// ManifestEntry records the inputs and outputs of a single successful build run
// for one platform target. The manifest file stores an array of these entries.
type ManifestEntry struct {
	// GodotVersion is the resolved Godot version (e.g., "4.3.0").
	GodotVersion string `json:"godot_version"`

	// VersionResolutionMethod is how the version was determined:
	// "explicit", "local-editor", "github-api", "interactive".
	VersionResolutionMethod string `json:"version_resolution_method"`

	// Platform is the target platform (e.g., "windows").
	Platform string `json:"platform"`

	// ToolchainChecksums maps toolchain component names to their SHA-256 hashes
	// for integrity verification (e.g., "python" -> "abc123...").
	ToolchainChecksums map[string]string `json:"toolchain_checksums"`

	// ToolVersion identifies this tool's own version for cache invalidation.
	ToolVersion string `json:"tool_version"`

	// Timestamp of the successful build.
	Timestamp time.Time `json:"timestamp"`

	// Success indicates whether the build completed successfully.
	Success bool `json:"success"`

	// TemplateRelease is the SHA-256 hash of the compiled release template.
	TemplateRelease string `json:"template_release_hash"`

	// TemplateDebug is the SHA-256 hash of the compiled debug template.
	TemplateDebug string `json:"template_debug_hash"`

	// Note: config state (version-era, preset structure) is NOT recorded.
	// Config corruption is caught at write time; manifest focuses on build inputs/outputs.
}

// Manifest is the in-memory slice of per-platform build entries.
// Callers work with this type; ManifestFile is only used for serialisation.
type Manifest = []ManifestEntry

// CacheKey represents the set of inputs that determine build cache validity for one
// platform target. If a matching entry is found in the manifest and Success=true,
// the build can be skipped (unless --force-rebuild).
type CacheKey struct {
	GodotVersion       string
	Platform           string
	ToolchainChecksums map[string]string
	ToolVersion        string
}

// Equals returns true if this CacheKey matches another.
func (k *CacheKey) Equals(other *CacheKey) bool {
	if other == nil {
		return false
	}
	if k.GodotVersion != other.GodotVersion ||
		k.Platform != other.Platform ||
		k.ToolVersion != other.ToolVersion {
		return false
	}
	if len(k.ToolchainChecksums) != len(other.ToolchainChecksums) {
		return false
	}
	for name, hash := range k.ToolchainChecksums {
		if normalizeChecksum(name, other.ToolchainChecksums[name]) != normalizeChecksum(name, hash) {
			return false
		}
	}
	return true
}

func normalizeChecksum(name, value string) string {
	if name == "godot_source" {
		return ""
	}
	if strings.HasPrefix(value, "placeholder_godot_") {
		return ""
	}
	return value
}
