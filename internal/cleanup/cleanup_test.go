package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPruneAfterSuccessNoRuntime(t *testing.T) {
	// GIVEN a workspace with no runtime directory
	tmpDir := t.TempDir()
	toolDir := filepath.Join(tmpDir, ".gst")
	err := os.MkdirAll(toolDir, 0755)
	assert.NoError(t, err)

	pruner := NewPruner(tmpDir, false)

	// WHEN pruning after success
	pruneErr := pruner.PruneAfterSuccess()

	// THEN no error should occur (nothing to prune is not an error)
	assert.Nil(t, pruneErr)
}

func TestPruneAfterSuccessRemovesRuntime(t *testing.T) {
	// GIVEN a workspace with runtime directory
	tmpDir := t.TempDir()
	toolDir := filepath.Join(tmpDir, ".gst")
	runtimePath := filepath.Join(toolDir, "runtime")
	err := os.MkdirAll(runtimePath, 0755)
	assert.NoError(t, err)

	// AND a marker file in runtime (to verify removal)
	markerFile := filepath.Join(runtimePath, "marker.txt")
	err = os.WriteFile(markerFile, []byte("test"), 0644)
	assert.NoError(t, err)

	pruner := NewPruner(tmpDir, false)

	// WHEN pruning after success
	pruneErr := pruner.PruneAfterSuccess()

	// THEN no error should occur
	assert.Nil(t, pruneErr)

	// AND runtime directory should be removed
	_, statErr := os.Stat(runtimePath)
	assert.Error(t, statErr, "runtime should be removed")
	assert.True(t, os.IsNotExist(statErr))
}

func TestPruneAfterSuccessKeepsRuntime(t *testing.T) {
	// GIVEN a workspace with runtime directory and KeepRuntime=true
	tmpDir := t.TempDir()
	toolDir := filepath.Join(tmpDir, ".gst")
	runtimePath := filepath.Join(toolDir, "runtime")
	err := os.MkdirAll(runtimePath, 0755)
	assert.NoError(t, err)

	markerFile := filepath.Join(runtimePath, "marker.txt")
	err = os.WriteFile(markerFile, []byte("test"), 0644)
	assert.NoError(t, err)

	pruner := NewPruner(tmpDir, true) // KeepRuntime=true

	// WHEN pruning after success
	pruneErr := pruner.PruneAfterSuccess()

	// THEN no error should occur
	assert.Nil(t, pruneErr)

	// AND runtime directory should still exist
	_, statErr := os.Stat(runtimePath)
	assert.NoError(t, statErr, "runtime should be preserved")
}

func TestPruneManualRemovesEntireToolDir(t *testing.T) {
	// GIVEN a complete .gst directory with various subdirs
	tmpDir := t.TempDir()
	toolDir := filepath.Join(tmpDir, ".gst")
	err := os.MkdirAll(filepath.Join(toolDir, "runtime"), 0755)
	assert.NoError(t, err)
	err = os.MkdirAll(filepath.Join(toolDir, "templates"), 0755)
	assert.NoError(t, err)

	// AND files in subdirectories
	err = os.WriteFile(filepath.Join(toolDir, "manifest.json"), []byte("{}"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(toolDir, "encryption.key"), []byte("abc123"), 0600)
	assert.NoError(t, err)

	pruner := NewPruner(tmpDir, false)

	// WHEN manually pruning
	pruneErr := pruner.PruneManual()

	// THEN no error should occur
	assert.Nil(t, pruneErr)

	// AND the entire .gst directory should be removed
	_, statErr := os.Stat(toolDir)
	assert.Error(t, statErr, ".gst should be removed")
	assert.True(t, os.IsNotExist(statErr))
}

func TestPruneManualNoDir(t *testing.T) {
	// GIVEN a workspace with no .gst directory
	tmpDir := t.TempDir()
	pruner := NewPruner(tmpDir, false)

	// WHEN manually pruning
	pruneErr := pruner.PruneManual()

	// THEN no error should occur (already clean is success)
	assert.Nil(t, pruneErr)
}

func TestPruneAfterSuccessWithOtherFilesInToolDir(t *testing.T) {
	// GIVEN a workspace with runtime and other preserved files (templates, keys)
	tmpDir := t.TempDir()
	toolDir := filepath.Join(tmpDir, ".gst")
	runtimePath := filepath.Join(toolDir, "runtime")
	templatesPath := filepath.Join(toolDir, "templates")

	err := os.MkdirAll(runtimePath, 0755)
	assert.NoError(t, err)
	err = os.MkdirAll(templatesPath, 0755)
	assert.NoError(t, err)

	// AND files that should be preserved
	err = os.WriteFile(filepath.Join(toolDir, "manifest.json"), []byte("{}"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(toolDir, "encryption.key"), []byte("abc123"), 0600)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(templatesPath, "template.so"), []byte("binary"), 0644)
	assert.NoError(t, err)

	pruner := NewPruner(tmpDir, false)

	// WHEN pruning after success
	pruneErr := pruner.PruneAfterSuccess()

	// THEN no error should occur
	assert.Nil(t, pruneErr)

	// AND runtime should be removed
	_, runtimeErr := os.Stat(runtimePath)
	assert.True(t, os.IsNotExist(runtimeErr))

	// AND preserved files should still exist
	_, manifestErr := os.Stat(filepath.Join(toolDir, "manifest.json"))
	assert.NoError(t, manifestErr, "manifest.json should be preserved")

	_, keyErr := os.Stat(filepath.Join(toolDir, "encryption.key"))
	assert.NoError(t, keyErr, "encryption.key should be preserved")

	_, templatesErr := os.Stat(filepath.Join(templatesPath, "template.so"))
	assert.NoError(t, templatesErr, "template files should be preserved")
}
