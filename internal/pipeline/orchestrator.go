package pipeline

import (
	"fmt"
	"time"

	"github.com/joemi/godot-secure-templater/internal/cleanup"
	"github.com/joemi/godot-secure-templater/internal/config"
	"github.com/joemi/godot-secure-templater/internal/longpath"
	"github.com/joemi/godot-secure-templater/internal/manifest"
	"github.com/joemi/godot-secure-templater/internal/version"
)

// Options captures all pipeline configuration for a build run.
type Options struct {
	// ProjectRoot is the detected Godot project directory
	ProjectRoot string

	// GodotVersion is the explicit Godot version (if supplied)
	GodotVersion string

	// GodotEditorPath points to a specific editor binary for local version detection.
	GodotEditorPath string

	// ProjectMinor is the project's declared minor line (e.g., "4.3")
	ProjectMinor string

	// Platform is the target platform (e.g., "windows")
	Platform string

	// KeepRuntime preserves toolchain after successful build
	KeepRuntime bool

	// ForceRebuild skips idempotency check (ignores cache)
	ForceRebuild bool

	// RegenerateKey requests new encryption key (requires confirmation)
	RegenerateKey bool

	// Force skips all confirmations (automation/CI mode)
	Force bool

	// Verbose enables verbose logging
	Verbose bool
}

// BuildResult captures the outcome of a build run.
type BuildResult struct {
	// Success indicates the build compiled and wired successfully
	Success bool

	// VersionResolution is how the version was determined
	VersionResolution *version.Resolution

	// CacheHit indicates idempotency cache matched
	CacheHit bool

	// Era is the config era for this Godot version
	Era config.Era

	// ManifestPath is where the build manifest was written
	ManifestPath string

	// Message is a user-friendly summary
	Message string

	// Warnings are non-fatal notices (e.g., long path, key regeneration)
	Warnings []string
}

// Orchestrator manages the build pipeline end-to-end.
type Orchestrator struct {
	opts *Options

	pruner         *cleanup.Pruner
	pathChecker    *longpath.Checker
	encryptionPath string
	manifestPath   string
}

// NewOrchestrator creates a pipeline orchestrator.
func NewOrchestrator(opts *Options) *Orchestrator {
	manifestPath := fmt.Sprintf("%s/.gst/manifest.json", opts.ProjectRoot)
	encryptionPath := fmt.Sprintf("%s/.gst/encryption.key", opts.ProjectRoot)

	return &Orchestrator{
		opts:           opts,
		manifestPath:   manifestPath,
		encryptionPath: encryptionPath,
		pruner:         cleanup.NewPruner(opts.ProjectRoot, opts.KeepRuntime),
		pathChecker:    longpath.NewChecker(opts.Platform),
	}
}

// CheckLongPaths validates that project paths are within acceptable lengths.
// Returns warnings (non-fatal) or error if paths definitely exceed limits.
func (o *Orchestrator) CheckLongPaths() ([]string, error) {
	var warnings []string

	if warning, err := o.pathChecker.CheckPath(o.opts.ProjectRoot); err != nil {
		return nil, fmt.Errorf("project path exceeds limits: %w", err)
	} else if warning != "" {
		warnings = append(warnings, warning)
	}

	if o.opts.Platform == "windows" {
		enabled, err := o.pathChecker.IsLongPathsEnabled()
		if err != nil {
			warnings = append(warnings, "warning: could not query Windows LongPathsEnabled registry state; path-length handling may be limited")
		} else if !enabled {
			warnings = append(warnings, "warning: Windows LongPathsEnabled is not enabled; very long paths may fail")
		}
	}

	return warnings, nil
}

// ResolveVersion determines Godot version via the configured strategy chain.
func (o *Orchestrator) ResolveVersion() (*version.Resolution, error) {
	// Create resolver with strategy chain (priority order)
	resolver := version.NewResolver(
		&version.ExplicitStrategy{Version: o.opts.GodotVersion},
		&version.LocalEditorStrategy{EditorPath: o.opts.GodotEditorPath},
		&version.GitHubAPIStrategy{MinorVersion: o.opts.ProjectMinor},
		&version.InteractiveStrategy{},
	)

	resolution, err := resolver.Resolve()
	if err != nil {
		return nil, fmt.Errorf("version resolution failed: %w", err)
	}

	return resolution, nil
}

// DetermineConfigEra maps the resolved version to a config era.
func (o *Orchestrator) DetermineConfigEra(version string) (config.Era, error) {
	era, err := config.VersionToEra(version)
	if err != nil {
		return "", fmt.Errorf("unsupported Godot version: %w", err)
	}
	return era, nil
}

// CheckIdempotency determines if the build can be skipped based on the cache.
func (o *Orchestrator) CheckIdempotency(resolution *version.Resolution, toolchainChecksums map[string]string, toolVersion string) bool {
	if o.opts.ForceRebuild {
		return false // User requested rebuild
	}

	loader := &manifest.Loader{ManifestPath: o.manifestPath}

	// Construct the current cache key
	currentKey := &manifest.CacheKey{
		GodotVersion:       resolution.Version,
		Platform:           o.opts.Platform,
		ToolVersion:        toolVersion,
		ToolchainChecksums: toolchainChecksums,
	}

	// Check if cache hit (manifest exists, matches, and last build succeeded)
	return loader.CanSkipBuild(currentKey)
}

// WriteManifest records the build result for future idempotency checks.
func (o *Orchestrator) WriteManifest(
	resolution *version.Resolution,
	platform string,
	toolchainChecksums map[string]string,
	toolVersion string,
	success bool,
	templateReleaseHash string,
	templateDebugHash string,
) error {
	m := &manifest.Manifest{
		GodotVersion:            resolution.Version,
		VersionResolutionMethod: string(resolution.Method),
		Platform:                platform,
		ToolVersion:             toolVersion,
		ToolchainChecksums:      toolchainChecksums,
		Timestamp:               time.Now().UTC(),
		Success:                 success,
		TemplateRelease:         templateReleaseHash,
		TemplateDebug:           templateDebugHash,
	}

	loader := &manifest.Loader{ManifestPath: o.manifestPath}
	if err := loader.Write(m); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// CleanupAfterSuccess prunes temporary build artifacts (unless --keep-runtime).
func (o *Orchestrator) CleanupAfterSuccess() error {
	if err := o.pruner.PruneAfterSuccess(); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	return nil
}

// GetTeammateMessage returns guidance for teammates who need to re-run the tool locally.
func (o *Orchestrator) GetTeammateMessage() string {
	return `
📋 Note for teammates:
	Treat the encryption key as a shared project secret.
	Distribute it securely to team members and CI via a secrets manager.
	Do not commit key material to source control.`
}
