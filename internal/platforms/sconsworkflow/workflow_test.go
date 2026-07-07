package sconsworkflow

import (
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
