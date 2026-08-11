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
	assert.Equal(t, "zig cc", env["CC"], "Windows host build env should advertise Zig as the C compiler source")
	assert.Equal(t, "zig c++", env["CXX"], "Windows host build env should advertise Zig as the C++ compiler source")
	assert.Equal(t, "zig ar", env["AR"], "Windows host build env should advertise Zig as the archiver source")
	assert.Equal(t, filepath.Join(workspace.Runtime, "zig-shims"), env["MINGW_PREFIX"], "Windows host build env should point MINGW_PREFIX at the Zig shim root")
	assert.Equal(t, filepath.Join(workspace.Runtime, "zig-cache", "global"), env["ZIG_GLOBAL_CACHE_DIR"], "Windows host build env should pin Zig global cache to a workspace-local directory")
	assert.Equal(t, filepath.Join(workspace.Runtime, "zig-cache", "local"), env["ZIG_LOCAL_CACHE_DIR"], "Windows host build env should pin Zig local cache to a workspace-local directory")
	assert.Equal(t, filepath.Join(workspace.Runtime, "zig-tmp"), env["TEMP"], "Windows host build env should pin TEMP to a workspace-local directory")
	assert.Equal(t, filepath.Join(workspace.Runtime, "zig-tmp"), env["TMP"], "Windows host build env should pin TMP to a workspace-local directory")
	assert.Contains(t, env["PATH"], filepath.Join(workspace.Runtime, "zig"), "Windows host build env should include the bundled Zig bin path")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Windows host build env should preserve the encryption key override")
}
