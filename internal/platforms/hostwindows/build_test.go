package hostwindows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesProvisionedMingwCompiler(t *testing.T) {
	// GIVEN a provisioned MinGW layout in the runtime directory
	runtimeDir := t.TempDir()
	mingwBin := filepath.Join(runtimeDir, "mingw", "bin")
	err := os.MkdirAll(mingwBin, 0755)
	assert.NoError(t, err, "mingw bin directory should be creatable")
	err = os.WriteFile(filepath.Join(mingwBin, "x86_64-w64-mingw32-gcc.exe"), []byte(""), 0644)
	assert.NoError(t, err, "mingw compiler marker should be writable")

	workspace := &internal.Workspace{Runtime: runtimeDir}

	// WHEN building environment overrides
	env := buildEnv(workspace, "test-key")

	// THEN Windows builds should use the provisioned MinGW compiler toolchain
	assert.Equal(t, "gcc", env["CC"], "Windows host build env should use MinGW gcc as the C compiler")
	assert.Equal(t, "g++", env["CXX"], "Windows host build env should use MinGW g++ as the C++ compiler")
	assert.Equal(t, "ar", env["AR"], "Windows host build env should use MinGW ar as the archiver")
	assert.Equal(t, filepath.Join(runtimeDir, "mingw"), env["MINGW_PREFIX"], "Windows host build env should point MINGW_PREFIX at the MinGW root")
	assert.Contains(t, env["PATH"], filepath.Join(runtimeDir, "mingw", "bin"), "Windows host build env should include MinGW bin path")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Windows host build env should preserve the encryption key override")
}

func TestEnsureWindowsMingwPrefixFailsWhenMissing(t *testing.T) {
	// GIVEN a workspace with no provisioned mingw
	runtimeDir := t.TempDir()

	// WHEN validating the provisioned MinGW prefix
	buildErr := ensureWindowsMingwPrefix(runtimeDir)

	// THEN an error should be returned because mingw is required
	assert.NotNil(t, buildErr, "ensureWindowsMingwPrefix should fail when mingw is not provisioned")
	assert.Equal(t, internal.ExitBuildFailed, buildErr.Code, "build env error should have ExitBuildFailed code")
}
