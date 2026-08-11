package hostwindows

import (
	"os"
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

	// THEN Windows builds should use the bundled Zig compiler toolchain
	assert.Equal(t, "zig cc", env["CC"], "Windows host build env should use zig cc as the C compiler")
	assert.Equal(t, "zig c++", env["CXX"], "Windows host build env should use zig c++ as the C++ compiler")
	assert.Equal(t, "zig ar", env["AR"], "Windows host build env should use zig ar as the archiver")
	assert.Equal(t, filepath.Join(workspace.Runtime, "zig-shims"), env["MINGW_PREFIX"], "Windows host build env should set MINGW_PREFIX to the generated runtime shim root")
	assert.Contains(t, env["PATH"], filepath.Join(workspace.Runtime, "zig-shims", "bin"), "Windows host build env should include shim bin path in PATH")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Windows host build env should preserve the encryption key override")
}

func TestEnsureWindowsZigShimsCreatesRequiredCompilerWrappers(t *testing.T) {
	// GIVEN a runtime directory for shim generation
	runtimeDir := t.TempDir()
	logger := internal.NewSimpleLogger(true)

	// WHEN preparing Windows Zig-backed MinGW-compatible shims
	shimErr := ensureWindowsZigShims(runtimeDir, logger)

	// THEN the shim setup should succeed
	assert.Nil(t, shimErr, "ensureWindowsZigShims should succeed when shim directory can be created")

	// AND expected wrapper commands should be generated under zig-shims/bin
	shimBin := filepath.Join(runtimeDir, "zig-shims", "bin")
	checkPaths := []string{
		filepath.Join(shimBin, "clang.cmd"),
		filepath.Join(shimBin, "clang++.cmd"),
		filepath.Join(shimBin, "ar.cmd"),
		filepath.Join(shimBin, "zig cc.cmd"),
		filepath.Join(shimBin, "zig c++.cmd"),
		filepath.Join(shimBin, "zig ar.cmd"),
		filepath.Join(shimBin, "windres.cmd"),
		filepath.Join(shimBin, "x86_64-w64-mingw32-clang.cmd"),
		filepath.Join(shimBin, "x86_64-w64-mingw32-ar.cmd"),
	}
	for _, p := range checkPaths {
		_, statErr := os.Stat(p)
		assert.NoError(t, statErr, "ensureWindowsZigShims should create required wrapper %s", p)
	}
}
