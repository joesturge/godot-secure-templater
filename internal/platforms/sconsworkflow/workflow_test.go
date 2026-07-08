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

func TestResolvePythonExecutableFallsBackToSystemPython(t *testing.T) {
	// GIVEN no provisioned runtime python and a fake system python3 on PATH
	runtimeDir := t.TempDir()
	binDir := t.TempDir()
	pythonPath := filepath.Join(binDir, "python3")
	err := os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "Fake python3 shim should be created")

	oldPath := os.Getenv("PATH")
	err = os.Setenv("PATH", binDir)
	assert.NoError(t, err, "PATH should be overrideable for the test")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", oldPath)
	})

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should fall back to the system python3
	assert.NoError(t, err, "resolvePythonExecutable should fall back to system python on POSIX hosts")
	assert.Equal(t, pythonPath, resolved, "resolvePythonExecutable should return the python3 found on PATH")
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
