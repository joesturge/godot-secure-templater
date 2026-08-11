package sconsworkflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestAdapterForHostTuple(t *testing.T) {
	tests := []struct {
		name       string
		hostTuple  string
		expectsWin bool
	}{
		{
			name:       "windows host selects windows adapter",
			hostTuple:  "windows/amd64",
			expectsWin: true,
		},
		{
			name:       "linux host selects posix adapter",
			hostTuple:  "linux/amd64",
			expectsWin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a host tuple

			// WHEN resolving the host adapter
			adapter := AdapterForHostTuple(tt.hostTuple)

			// THEN adapter type should match host family
			_, isWindowsAdapter := adapter.(windowsHostAdapter)
			assert.Equal(t, tt.expectsWin, isWindowsAdapter, "AdapterForHostTuple should return expected adapter type for host tuple")
		})
	}
}

func TestPosixHostBuildEnv(t *testing.T) {
	// GIVEN a POSIX host adapter and workspace paths
	adapter := posixHostAdapter{}
	workspace := &internal.Workspace{Runtime: filepath.Join("/tmp", "runtime")}

	// WHEN building environment overrides
	env := adapter.BuildEnv(workspace, "test-key")

	// THEN PATH should use POSIX separator and key should be set
	assert.Contains(t, env["PATH"], ":", "POSIX PATH should use colon separator")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "BuildEnv should include encryption key override")
}

func TestResolvePythonExecutableFailsWithoutRuntimePython(t *testing.T) {
	// GIVEN no provisioned runtime python
	runtimeDir := t.TempDir()

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should fail fast instead of falling back to host binaries
	assert.Error(t, err, "resolvePythonExecutable should fail when runtime python is missing")
	assert.Equal(t, filepath.Join(runtimeDir, "python", "python"), resolved, "resolvePythonExecutable should return the expected runtime python path on POSIX hosts")
}

func TestResolvePythonExecutable_BinLayout(t *testing.T) {
	// GIVEN a python-build-standalone layout (python/bin/python)
	runtimeDir := t.TempDir()
	binDir := filepath.Join(runtimeDir, "python", "bin")
	err := os.MkdirAll(binDir, 0755)
	assert.NoError(t, err, "python/bin directory should be creatable")

	pythonPath := filepath.Join(binDir, "python")
	err = os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "python/bin/python executable should be creatable")

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should find the bin/python layout
	assert.NoError(t, err, "resolvePythonExecutable should succeed for python-build-standalone layout")
	assert.Equal(t, pythonPath, resolved, "resolvePythonExecutable should return python/bin/python for python-build-standalone layout")
}

func TestResolveZigExecutable_RuntimeRoot(t *testing.T) {
	// GIVEN a runtime directory with zig at the runtime root layout
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err, "Runtime zig directory should be creatable")

	zigPath := filepath.Join(zigDir, "zig")
	err = os.WriteFile(zigPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "Root zig executable should be creatable")

	// WHEN resolving the zig executable
	resolved, err := resolveZigExecutable(runtimeDir)

	// THEN it should return the runtime root zig executable
	assert.NoError(t, err, "resolveZigExecutable should resolve root runtime zig executable")
	assert.Equal(t, zigPath, resolved, "resolveZigExecutable should return runtime root zig path")
}

func TestResolveZigExecutable_NestedLayout(t *testing.T) {
	// GIVEN a runtime directory with zig in nested extracted layout
	runtimeDir := t.TempDir()
	nestedDir := filepath.Join(runtimeDir, "zig", "zig-x86_64-linux-0.16.0")
	err := os.MkdirAll(nestedDir, 0755)
	assert.NoError(t, err, "Nested zig directory should be creatable")

	zigPath := filepath.Join(nestedDir, "zig")
	err = os.WriteFile(zigPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "Nested zig executable should be creatable")

	// WHEN resolving the zig executable
	resolved, err := resolveZigExecutable(runtimeDir)

	// THEN it should return the nested zig executable
	assert.NoError(t, err, "resolveZigExecutable should resolve nested runtime zig executable")
	assert.Equal(t, zigPath, resolved, "resolveZigExecutable should return nested zig path")
}
