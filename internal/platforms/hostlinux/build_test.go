package hostlinux

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesZigCompiler(t *testing.T) {
	// GIVEN a linux host workspace
	workspace := &internal.Workspace{Runtime: filepath.Join("/tmp", "runtime")}

	// WHEN building environment overrides
	env := buildEnv(workspace, "test-key")

	// THEN the build should prefer the provisioned Zig compiler toolchain
	assert.Equal(t, "zig cc", env["CC"], "Linux host build env should use zig cc as the C compiler")
	assert.Equal(t, "zig c++", env["CXX"], "Linux host build env should use zig c++ as the C++ compiler")
	assert.Equal(t, "zig ar", env["AR"], "Linux host build env should use zig ar as the archiver")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Linux host build env should preserve the encryption key override")
}

func TestEnsureHostDependencies(t *testing.T) {
	tests := []struct {
		name      string
		lookupErr error
		wantErr   bool
	}{
		{
			name:      "pkg-config available succeeds",
			lookupErr: nil,
			wantErr:   false,
		},
		{
			name:      "missing pkg-config fails with build error",
			lookupErr: fmt.Errorf("not found"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN deterministic host path lookup behaviour
			oldLookPath := lookPathFn
			lookPathFn = func(file string) (string, error) {
				if tt.lookupErr != nil {
					return "", tt.lookupErr
				}
				return "/usr/bin/pkg-config", nil
			}
			t.Cleanup(func() {
				lookPathFn = oldLookPath
			})

			// WHEN validating host dependencies for linux compilation
			err := ensureHostDependencies()

			// THEN result should match expected dependency availability
			if tt.wantErr {
				assert.NotNil(t, err, "ensureHostDependencies should return typed error when pkg-config is missing")
				assert.Equal(t, internal.ExitBuildFailed, err.Code, "Missing pkg-config should map to build-failed exit code")
				assert.Contains(t, err.Message, "pkg-config", "Missing dependency message should mention pkg-config")
				return
			}

			assert.Nil(t, err, "ensureHostDependencies should succeed when pkg-config is available")
		})
	}
}
