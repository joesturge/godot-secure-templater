package hostlinux

import (
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
