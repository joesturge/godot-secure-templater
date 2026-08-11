package hostlinux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesAbsoluteZigCompiler(t *testing.T) {
	// GIVEN a provisioned zig binary in the runtime directory
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err)
	zigExe := filepath.Join(zigDir, "zig")
	err = os.WriteFile(zigExe, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err)

	workspace := &internal.Workspace{Runtime: runtimeDir}

	// WHEN building environment overrides
	env := buildEnv(workspace, "test-key")

	// THEN CC/CXX/AR must reference the absolute path to the provisioned zig
	assert.True(t, strings.HasPrefix(env["CC"], zigExe+" "), "CC should use absolute zig path, got: %s", env["CC"])
	assert.Equal(t, zigExe+" cc", env["CC"], "Linux host build env should use '<abszig> cc' as the C compiler")
	assert.Equal(t, zigExe+" c++", env["CXX"], "Linux host build env should use '<abszig> c++' as the C++ compiler")
	assert.Equal(t, zigExe+" ar", env["AR"], "Linux host build env should use '<abszig> ar' as the archiver")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Linux host build env should preserve the encryption key override")
}

func TestBuildEnvFallsBackGracefullyWhenZigMissing(t *testing.T) {
	// GIVEN a workspace with no provisioned zig
	runtimeDir := t.TempDir()
	workspace := &internal.Workspace{Runtime: runtimeDir}

	// WHEN building environment overrides
	env := buildEnv(workspace, "test-key")

	// THEN the base env should still contain the encryption key;
	// CC/CXX/AR will be absent because zig could not be resolved.
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "build env should preserve encryption key even when zig is missing")
	assert.Empty(t, env["CC"], "CC should be unset when zig is not provisioned")
}

func TestZigCompilerEnvReturnsAbsolutePaths(t *testing.T) {
	// GIVEN a provisioned zig binary
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err)
	zigExe := filepath.Join(zigDir, "zig")
	err = os.WriteFile(zigExe, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err)

	// WHEN resolving the compiler environment
	compilerEnv, gstErr := zigCompilerEnv(runtimeDir)

	// THEN it should return absolute paths
	assert.Nil(t, gstErr, "zigCompilerEnv should succeed with provisioned zig")
	assert.Equal(t, zigExe+" cc", compilerEnv["CC"])
	assert.Equal(t, zigExe+" c++", compilerEnv["CXX"])
	assert.Equal(t, zigExe+" ar", compilerEnv["AR"])
}

func TestZigCompilerEnvFailsWhenZigMissing(t *testing.T) {
	// GIVEN a runtime directory with no zig
	runtimeDir := t.TempDir()

	// WHEN resolving the compiler environment
	_, gstErr := zigCompilerEnv(runtimeDir)

	// THEN it should return a build-failed error
	assert.NotNil(t, gstErr, "zigCompilerEnv should fail when zig is not provisioned")
	assert.Equal(t, internal.ExitBuildFailed, gstErr.Code)
}

