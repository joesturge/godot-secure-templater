package hostlinux

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestBuildEnvUsesAbsoluteZigCompiler(t *testing.T) {
	// GIVEN a provisioned zig binary in the runtime directory
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err, "zig directory should be creatable")
	zigExe := filepath.Join(zigDir, "zig")
	err = os.WriteFile(zigExe, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "zig executable should be writable")

	workspace := &internal.Workspace{Runtime: runtimeDir}

	// WHEN building environment overrides
	env, buildErr := buildEnv(workspace, "test-key")

	// THEN CC/CXX/AR must reference the absolute path to the provisioned zig
	assert.Nil(t, buildErr, "buildEnv should succeed with provisioned zig")
	quotedZig := fmt.Sprintf("%q", zigExe)
	assert.Equal(t, quotedZig+" cc", env["CC"], "Linux host build env should use quoted '<abszig> cc' as the C compiler")
	assert.Equal(t, quotedZig+" c++", env["CXX"], "Linux host build env should use quoted '<abszig> c++' as the C++ compiler")
	assert.Equal(t, quotedZig+" ar", env["AR"], "Linux host build env should use quoted '<abszig> ar' as the archiver")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Linux host build env should preserve the encryption key override")
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

func TestZigCompilerEnvReturnsAbsolutePaths(t *testing.T) {
	// GIVEN a provisioned zig binary
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err, "zig directory should be creatable")
	zigExe := filepath.Join(zigDir, "zig")
	err = os.WriteFile(zigExe, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "zig executable should be writable")

	// WHEN resolving the compiler environment
	compilerEnv, gstErr := zigCompilerEnv(runtimeDir)

	// THEN it should return quoted absolute paths
	assert.Nil(t, gstErr, "zigCompilerEnv should succeed with provisioned zig")
	quotedZig := fmt.Sprintf("%q", zigExe)
	assert.Equal(t, quotedZig+" cc", compilerEnv["CC"], "CC should use quoted absolute zig path")
	assert.Equal(t, quotedZig+" c++", compilerEnv["CXX"], "CXX should use quoted absolute zig path")
	assert.Equal(t, quotedZig+" ar", compilerEnv["AR"], "AR should use quoted absolute zig path")
}

func TestZigCompilerEnvFailsWhenZigMissing(t *testing.T) {
	// GIVEN a runtime directory with no zig
	runtimeDir := t.TempDir()

	// WHEN resolving the compiler environment
	_, gstErr := zigCompilerEnv(runtimeDir)

	// THEN it should return a build-failed error
	assert.NotNil(t, gstErr, "zigCompilerEnv should fail when zig is not provisioned")
	assert.Equal(t, internal.ExitBuildFailed, gstErr.Code, "zigCompilerEnv should return ExitBuildFailed when zig is missing")
}

