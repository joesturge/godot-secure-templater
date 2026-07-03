package main

import (
	"bytes"
	"os"
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
			name:    "missing required godot-version flag",
			args:    []string{"create"},
			wantErr: "required flag(s) \"godot-version\" not set",
		},
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

	// THEN the create subcommand should be discoverable
	assert.NoError(t, err, "create subcommand should be registered on root command")
	assert.Equal(t, "create", cmd.Name(), "Resolved command should be create")

	// AND the godot-version flag should be required
	assert.NotNil(t, flag, "create command should define the godot-version flag")
	assert.Contains(t, flag.Annotations, "cobra_annotation_bash_completion_one_required_flag", "Required-flag annotation should be present")
	assert.Equal(t, []string{"true"}, flag.Annotations["cobra_annotation_bash_completion_one_required_flag"], "godot-version should be marked as required")
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
	t.Cleanup(func() {
		flagGodotVersion = oldVersion
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
