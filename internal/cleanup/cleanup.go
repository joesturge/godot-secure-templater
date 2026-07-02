package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
)

// Pruner handles cleanup of temporary build artifacts.
type Pruner struct {
	// WorkspaceRoot is the project root (where .gst/ lives)
	WorkspaceRoot string

	// KeepRuntime if true, preserves .gst/runtime/ (e.g., for debugging or CI caching)
	KeepRuntime bool
}

// NewPruner creates a Pruner for the given workspace.
func NewPruner(workspaceRoot string, keepRuntime bool) *Pruner {
	return &Pruner{
		WorkspaceRoot: workspaceRoot,
		KeepRuntime:   keepRuntime,
	}
}

// PruneAfterSuccess removes temporary artifacts after a successful build.
// Preserves: templates/, encryption.key, manifest.json (these are needed for exports)
// Removes (unless KeepRuntime=true): runtime/ (toolchain, Godot source)
// Returns nil if nothing was removed; error only if an actionable failure occurs.
func (p *Pruner) PruneAfterSuccess() error {
	if p.KeepRuntime {
		return nil // User requested to keep runtime for caching or debugging
	}

	runtimePath := filepath.Join(p.WorkspaceRoot, ".gst", "runtime")

	// Check if runtime exists before attempting removal
	_, err := os.Stat(runtimePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to prune
		}
		return fmt.Errorf("failed to stat runtime directory: %w", err)
	}

	// Remove runtime
	if err := os.RemoveAll(runtimePath); err != nil {
		return fmt.Errorf("failed to prune runtime: %w", err)
	}

	return nil
}

// PruneManual explicitly removes .gst/ directory on demand (via 'clean' command).
// This is the nuclear option — removes everything, including templates and keys.
// Only called via explicit command; not after build success.
func (p *Pruner) PruneManual() error {
	toolDir := filepath.Join(p.WorkspaceRoot, ".gst")

	_, err := os.Stat(toolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already cleaned
		}
		return fmt.Errorf("failed to stat .gst: %w", err)
	}

	if err := os.RemoveAll(toolDir); err != nil {
		return fmt.Errorf("failed to remove .gst: %w", err)
	}

	return nil
}
