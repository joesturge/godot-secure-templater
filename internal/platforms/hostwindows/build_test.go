package hostwindows

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesBundledZigCompiler(t *testing.T) {
	// GIVEN a Windows host workspace
	workspace := &internal.Workspace{Runtime: filepath.Join("C:", "tmp", "runtime")}

	// WHEN building environment overrides
	env := buildEnv(workspace, "test-key")

	// THEN Windows builds should invoke Zig directly and stay off MinGW wrappers
	assert.Equal(t, "zig cc", env["CC"], "Windows host build env should use zig cc as the C compiler")
	assert.Equal(t, "zig c++", env["CXX"], "Windows host build env should use zig c++ as the C++ compiler")
	assert.Equal(t, "zig ar", env["AR"], "Windows host build env should use zig ar as the archiver")
	assert.NotContains(t, env, "MINGW_PREFIX", "Windows host build env should not set MINGW_PREFIX")
	assert.Contains(t, env["PATH"], filepath.Join(workspace.Runtime, "zig"), "Windows host build env should include the bundled Zig bin path")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Windows host build env should preserve the encryption key override")
}
