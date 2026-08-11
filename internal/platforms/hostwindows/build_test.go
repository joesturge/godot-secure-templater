package hostwindows

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesBundledMingwCompiler(t *testing.T) {
	// GIVEN a Windows host workspace
	workspace := &internal.Workspace{Runtime: filepath.Join("C:", "tmp", "runtime")}

	// WHEN building environment overrides
	env := buildEnv(workspace, "test-key")

	// THEN Windows builds should use the bundled MinGW compiler toolchain
	assert.Equal(t, "gcc", env["CC"], "Windows host build env should use MinGW gcc as the C compiler")
	assert.Equal(t, "g++", env["CXX"], "Windows host build env should use MinGW g++ as the C++ compiler")
	assert.Equal(t, "ar", env["AR"], "Windows host build env should use MinGW ar as the archiver")
	assert.Equal(t, filepath.Join(workspace.Runtime, "mingw"), env["MINGW_PREFIX"], "Windows host build env should point MINGW_PREFIX at the MinGW root")
	assert.Contains(t, env["PATH"], filepath.Join(workspace.Runtime, "mingw", "bin"), "Windows host build env should include MinGW bin path")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Windows host build env should preserve the encryption key override")
}
