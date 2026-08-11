package hostwindows

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesProvisionedZigCompiler(t *testing.T) {
	// GIVEN a provisioned zig.exe binary in the runtime directory
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err, "zig directory should be creatable")
	zigExe := filepath.Join(zigDir, "zig.exe")
	err = os.WriteFile(zigExe, []byte("@echo off\r\nexit 0\r\n"), 0755)
	assert.NoError(t, err, "zig.exe should be writable")

	workspace := &internal.Workspace{Runtime: runtimeDir}

	// WHEN building environment overrides
	env, buildErr := buildEnv(workspace, "test-key")

	// THEN Windows builds should use the provisioned Zig compiler toolchain
	assert.Nil(t, buildErr, "buildEnv should succeed with provisioned zig")
	quotedZig := fmt.Sprintf("%q", zigExe)
	assert.Equal(t, quotedZig+" cc", env["CC"], "Windows host build env should use provisioned zig cc as the C compiler")
	assert.Equal(t, quotedZig+" c++", env["CXX"], "Windows host build env should use provisioned zig c++ as the C++ compiler")
	assert.Equal(t, quotedZig+" ar", env["AR"], "Windows host build env should use provisioned zig ar as the archiver")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Windows host build env should preserve the encryption key override")
}

func TestBuildEnvFailsWhenZigMissing(t *testing.T) {
	// GIVEN a workspace with no provisioned zig
	runtimeDir := t.TempDir()
	workspace := &internal.Workspace{Runtime: runtimeDir}

	// WHEN building environment overrides
	_, buildErr := buildEnv(workspace, "test-key")

	// THEN an error should be returned because zig is required
	assert.NotNil(t, buildErr, "buildEnv should fail when zig is not provisioned")
	assert.Equal(t, internal.ExitBuildFailed, buildErr.Code, "build env error should have ExitBuildFailed code")
}

