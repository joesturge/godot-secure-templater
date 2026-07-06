package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestRootCommand_CreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown command",
			args:    []string{"does-not-exist"},
			wantErr: "unknown command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a root command configured with invalid CLI args
			bufOut := &bytes.Buffer{}
			bufErr := &bytes.Buffer{}
			rootCmd.SetOut(bufOut)
			rootCmd.SetErr(bufErr)
			rootCmd.SetArgs(tt.args)

			// WHEN executing the command
			err := rootCmd.Execute()

			// THEN a validation error should be returned
			assert.Error(t, err, "Command execution should fail for invalid CLI input")
			assert.Contains(t, err.Error(), tt.wantErr, "Error should explain the specific CLI validation issue")

			// AND stderr should include the same validation detail
			assert.Contains(t, bufErr.String(), tt.wantErr, "stderr should surface the same validation message to users")
		})
	}
}

func TestRootCommand_HasCreateSubcommandAndRequiredFlag(t *testing.T) {
	// GIVEN the root command from package initialisation
	cmd, _, err := rootCmd.Find([]string{"create"})

	// WHEN inspecting the command metadata
	flag := createCmd.Flag("godot-version")
	platformFlag := createCmd.Flag("platform")
	editorPathFlag := createCmd.Flag("godot-editor-path")

	// THEN the create subcommand should be discoverable
	assert.NoError(t, err, "create subcommand should be registered on root command")
	assert.Equal(t, "create", cmd.Name(), "Resolved command should be create")

	// AND the godot-version flag should be present and optional (slice 1 resolver fallback)
	assert.NotNil(t, flag, "create command should define the godot-version flag")
	assert.NotContains(t, flag.Annotations, "cobra_annotation_bash_completion_one_required_flag", "godot-version should not be required in slice 1")

	// AND the create command should expose editor path override for local-editor strategy
	assert.NotNil(t, editorPathFlag, "create command should define the godot-editor-path flag")

	// AND the platform flag should default to the detected host tuple
	assert.NotNil(t, platformFlag, "create command should define the platform flag")
	assert.Equal(t, detectedHostTuple(), platformFlag.DefValue, "platform flag default should match detected GOOS/GOARCH host tuple")
}

func TestDetectedHostTuple(t *testing.T) {
	// GIVEN runtime host information
	// WHEN deriving the tuple
	tuple := detectedHostTuple()

	// THEN the tuple should contain OS and arch separated by a slash
	assert.Contains(t, tuple, "/", "Detected host tuple should use os/arch format")
	parts := bytes.Split([]byte(tuple), []byte("/"))
	assert.Len(t, parts, 2, "Detected host tuple should have exactly two components")
}

func TestResolveTargetPlatform(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTuple    string
		wantPlatform string
		wantErr      bool
	}{
		{
			name:         "windows tuple explicit",
			input:        "windows/amd64",
			wantTuple:    "windows/amd64",
			wantPlatform: "windows",
			wantErr:      false,
		},
		{
			name:         "windows alias normalised",
			input:        "windows",
			wantTuple:    "windows/amd64",
			wantPlatform: "windows",
			wantErr:      false,
		},
		{
			name:         "linux tuple is unknown when plugin is not registered",
			input:        "linux/amd64",
			wantTuple:    "linux/amd64",
			wantPlatform: "",
			wantErr:      true,
		},
		{
			name:         "unknown tuple unsupported",
			input:        "darwin/arm64",
			wantTuple:    "darwin/arm64",
			wantPlatform: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a platform tuple input

			// WHEN resolving target platform details
			gotTuple, gotPlatform, err := resolveTargetPlatform(tt.input)

			// THEN resolved tuple and platform should match expectations
			assert.Equal(t, tt.wantTuple, gotTuple, "Resolved tuple should match expected normalised tuple")
			assert.Equal(t, tt.wantPlatform, gotPlatform, "Resolved platform should match expected platform key")

			// AND error presence should match support status
			if tt.wantErr {
				assert.NotNil(t, err, "Unsupported tuple should return a typed error")
			} else {
				assert.Nil(t, err, "Supported tuple should resolve without error")
			}
		})
	}
}

func TestRootCommand_HasCleanSubcommand(t *testing.T) {
	// GIVEN the root command from package initialisation
	cmd, _, err := rootCmd.Find([]string{"clean"})

	// WHEN resolving subcommand metadata
	// THEN the clean subcommand should be discoverable
	assert.NoError(t, err, "clean subcommand should be registered on root command")
	assert.Equal(t, "clean", cmd.Name(), "Resolved command should be clean")
}

func TestRunCreate_ReturnsNotGodotProjectError(t *testing.T) {
	// GIVEN an empty temporary directory that is not a Godot project
	tmp := t.TempDir()
	oldWD, wdErr := os.Getwd()
	assert.NoError(t, wdErr, "Current working directory should be readable for test setup")

	chdirErr := os.Chdir(tmp)
	assert.NoError(t, chdirErr, "Changing to temporary test directory should succeed")
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	oldVersion := flagGodotVersion
	flagGodotVersion = "4.3.2"
	oldPlatform := flagPlatform
	flagPlatform = "windows/amd64"
	t.Cleanup(func() {
		flagGodotVersion = oldVersion
		flagPlatform = oldPlatform
	})

	// WHEN running create directly
	err := runCreate(createCmd, nil)

	// THEN a typed not-godot-project error should be returned
	assert.Error(t, err, "runCreate should fail when project.godot is missing")

	var toolErr *internal.Error
	assert.ErrorAs(t, err, &toolErr, "runCreate should return the tool's typed error type")
	assert.Equal(t, internal.ExitNotGodotProject, toolErr.Code, "Error code should indicate non-Godot project context")
	assert.Contains(t, toolErr.Message, "project.godot", "Error message should tell user what file is missing")
}

func TestRunClean_ReturnsNotGodotProjectError(t *testing.T) {
	// GIVEN an empty temporary directory that is not a Godot project
	tmp := t.TempDir()
	oldWD, wdErr := os.Getwd()
	assert.NoError(t, wdErr, "Current working directory should be readable for test setup")

	chdirErr := os.Chdir(tmp)
	assert.NoError(t, chdirErr, "Changing to temporary test directory should succeed")
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	// WHEN running clean directly
	err := runClean(cleanCmd, nil)

	// THEN a typed not-godot-project error should be returned
	assert.Error(t, err, "runClean should fail when project.godot is missing")

	var toolErr *internal.Error
	assert.ErrorAs(t, err, &toolErr, "runClean should return the tool's typed error type")
	assert.Equal(t, internal.ExitNotGodotProject, toolErr.Code, "Error code should indicate non-Godot project context")
}

func TestAcquireRunLock(t *testing.T) {
	tests := []struct {
		name        string
		seedLock    bool
		seedContent string
		wantErrCode internal.ExitCode
	}{
		{
			name:        "acquires and releases lock when no lock exists",
			seedLock:    false,
			seedContent: "",
		},
		{
			name:        "returns lock held when lock already exists",
			seedLock:    true,
			seedContent: "pid=1234\nhost=test-host\n",
			wantErrCode: internal.ExitLockHeld,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a lock path with optional pre-existing lock metadata
			tmpDir := t.TempDir()
			lockPath := filepath.Join(tmpDir, ".lock")

			if tt.seedLock {
				writeErr := os.WriteFile(lockPath, []byte(tt.seedContent), 0600)
				assert.NoError(t, writeErr, "Should seed lock file for lock-held scenario")
			}

			// WHEN acquiring the run lock
			release, err := acquireRunLock(lockPath)

			if tt.seedLock {
				// THEN a typed lock-held error should be returned
				assert.NotNil(t, err, "Expected lock acquisition to fail when lock exists")
				assert.Nil(t, release, "Release callback should be nil when lock acquisition fails")
				assert.Equal(t, tt.wantErrCode, err.Code, "Error code should indicate lock held")
				assert.Contains(t, err.Details, "test-host", "Error details should include lock owner host when metadata exists")
				return
			}

			// THEN lock acquisition should succeed
			assert.Nil(t, err, "Expected lock acquisition to succeed when lock file does not exist")
			assert.NotNil(t, release, "Release callback should be returned on successful lock acquisition")

			_, statErr := os.Stat(lockPath)
			assert.NoError(t, statErr, "Lock file should exist after acquisition")

			// AND releasing should remove the lock file
			release()
			_, postReleaseErr := os.Stat(lockPath)
			assert.True(t, os.IsNotExist(postReleaseErr), "Lock file should be removed after release")
		})
	}
}

func TestBuildToolchainChecksums(t *testing.T) {
	// GIVEN toolchain component metadata with pinned checksums
	components := []internal.Artifact{
		{Name: "python", SHA256: "abc"},
		{Name: "mingw", SHA256: "def"},
		{Name: "godot_source", SHA256: "placeholder_godot_4.6.3"},
	}

	// WHEN building the checksum map
	checksums := buildToolchainChecksums(components)

	// THEN all component checksums should be indexed by component name
	assert.Equal(t, "abc", checksums["python"], "Python checksum should be preserved in map")
	assert.Equal(t, "def", checksums["mingw"], "MinGW checksum should be preserved in map")
	assert.Equal(t, "", checksums["godot_source"], "Legacy placeholder checksums should be normalized to empty for cache compatibility")
	assert.Len(t, checksums, 3, "Checksum map should include all components")
}
