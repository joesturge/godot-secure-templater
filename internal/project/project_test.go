package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/stretchr/testify/assert"
)

func TestDetectGodotProject(t *testing.T) {
	// GIVEN a directory with a project.godot file
	tmpDir := t.TempDir()
	projectFile := filepath.Join(tmpDir, "project.godot")
	writeErr := os.WriteFile(projectFile, []byte("[application]\nname=\"Test\""), 0644)
	assert.NoError(t, writeErr, "Should write the project.godot fixture")

	// WHEN detecting a Godot project
	detected, err := Detect(tmpDir)

	// THEN no error should occur
	assert.Nil(t, err, "Detect should not error")
	// AND the project directory should be returned
	assert.Equal(t, tmpDir, detected, "Detect should return project directory")
}

func TestDetectMissingProject(t *testing.T) {
	// GIVEN a directory without a project.godot file
	tmpDir := t.TempDir()

	// WHEN trying to detect a Godot project
	detected, err := Detect(tmpDir)

	// THEN an error should occur
	assert.NotNil(t, err, "Detect should error on missing project")
	// AND it should be the correct error type
	assert.Equal(t, internal.ExitNotGodotProject, err.Code, "Should return ExitNotGodotProject")
	// AND no path should be returned
	assert.Empty(t, detected, "Should return empty path on error")
}

func TestReadVersion(t *testing.T) {
	// GIVEN various project.godot file contents with different version formats
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name: "standard 4.3 project",
			content: `config_version=5
[application]
config/name="Test"
config/features=PackedStringArray("4.3", "GL Rendering")
`,
			want:    "4.3",
			wantErr: false,
		},
		{
			name: "4.4 project",
			content: `config_version=5
config/features=PackedStringArray("4.4", "Mobile")
`,
			want:    "4.4",
			wantErr: false,
		},
		{
			name: "missing config/features",
			content: `config_version=5
[application]
config/name="Test"
`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "malformed features line",
			content: `config/features=Invalid`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a project file with specific content
			tmpDir := t.TempDir()
			projectFile := filepath.Join(tmpDir, "project.godot")
			writeErr := os.WriteFile(projectFile, []byte(tt.content), 0644)
			assert.NoError(t, writeErr, "Should write the project.godot fixture for the scenario")

			// WHEN reading the version
			got, err := ReadVersion(tmpDir)

			// THEN the error status should match expectations
			if tt.wantErr {
				assert.NotNil(t, err, "ReadVersion should error: %s", tt.name)
			} else {
				assert.Nil(t, err, "ReadVersion should not error: %s", tt.name)
				assert.Equal(t, tt.want, got, "ReadVersion should return correct version")
			}
		})
	}
}

func TestValidateMinorLine(t *testing.T) {
	// GIVEN various combinations of project minor version and supplied version values
	tests := []struct {
		name            string
		projectMinor    string
		suppliedVersion string
		wantErr         bool
		wantErrorCode   internal.ExitCode
		description     string
	}{
		{
			name:            "matching minor lines",
			projectMinor:    "4.3",
			suppliedVersion: "4.3.2",
			wantErr:         false,
			description:     "should accept matching minor",
		},
		{
			name:            "matching major.minor, different patch",
			projectMinor:    "4.3",
			suppliedVersion: "4.3.0",
			wantErr:         false,
			description:     "should accept same minor, different patch",
		},
		{
			name:            "different minor lines",
			projectMinor:    "4.3",
			suppliedVersion: "4.4.0",
			wantErr:         true,
			wantErrorCode:   internal.ExitVersionResolution,
			description:     "should reject different minor lines",
		},
		{
			name:            "invalid version format",
			projectMinor:    "4.3",
			suppliedVersion: "4",
			wantErr:         true,
			wantErrorCode:   internal.ExitVersionResolution,
			description:     "should reject malformed version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN project and supplied versions
			// WHEN validating the minor line
			err := ValidateMinorLine(tt.projectMinor, tt.suppliedVersion)

			// THEN the error status should match expectations
			if tt.wantErr {
				assert.NotNil(t, err, "ValidateMinorLine should error: %s", tt.description)
				if err != nil {
					assert.Equal(t, tt.wantErrorCode, err.Code, "Error code should match")
				}
			} else {
				assert.Nil(t, err, "ValidateMinorLine should not error: %s", tt.description)
			}
		})
	}
}

func TestEnsureGitignore(t *testing.T) {
	// GIVEN various .gitignore file states (empty, with other entries, with existing entry)
	tests := []struct {
		name           string
		initialContent string
		wantContains   string
	}{
		{
			name:           "empty gitignore",
			initialContent: "",
			wantContains:   ".gst/",
		},
		{
			name:           "existing gitignore without entry",
			initialContent: "*.o\n*.a\n",
			wantContains:   ".gst/",
		},
		{
			name:           "existing gitignore with entry",
			initialContent: ".gst/\n",
			wantContains:   ".gst/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a temporary directory with optional existing .gitignore
			tmpDir := t.TempDir()
			gitignorePath := filepath.Join(tmpDir, ".gitignore")

			if tt.initialContent != "" {
				writeErr := os.WriteFile(gitignorePath, []byte(tt.initialContent), 0644)
				assert.NoError(t, writeErr, "Should write the initial .gitignore content")
			}

			// WHEN ensuring .gitignore exists
			err := EnsureGitignore(tmpDir)

			// THEN no error should occur
			assert.Nil(t, err, "EnsureGitignore should not error")

			// AND the file should exist with the required entry
			content, readErr := os.ReadFile(gitignorePath)
			assert.NoError(t, readErr, "Should be able to read .gitignore")
			assert.Contains(t, string(content), tt.wantContains, "Gitignore should contain entry")
		})
	}
}

func TestInitWorkspace(t *testing.T) {
	// GIVEN a project directory
	tmpDir := t.TempDir()

	// WHEN initializing the workspace
	ws, err := InitWorkspace(tmpDir)

	// THEN no error should occur
	assert.Nil(t, err, "InitWorkspace should not error")

	// AND all required paths should be set and created
	expectedDirs := []string{
		ws.Root,
		ws.Runtime,
		ws.Templates,
		ws.Logs,
	}

	for _, dir := range expectedDirs {
		info, statErr := os.Stat(dir)
		assert.NoError(t, statErr, "Directory should exist: %s", dir)
		assert.True(t, info.IsDir(), "Path should be a directory: %s", dir)
	}

	// AND manifest, lock, and key paths should be set
	assert.NotEmpty(t, ws.Manifest, "Manifest path should be set")
	assert.NotEmpty(t, ws.Lock, "Lock path should be set")
	assert.NotEmpty(t, ws.KeyFile, "KeyFile path should be set")
}

func TestInitWorkspaceNestedCreation(t *testing.T) {
	// GIVEN a project directory
	tmpDir := t.TempDir()

	// WHEN initializing the workspace
	ws, err := InitWorkspace(tmpDir)

	// THEN no error should occur
	assert.Nil(t, err, "InitWorkspace should not error")

	// AND runtime component subdirectories should not be pre-created
	nestedDirs := []string{
		filepath.Join(ws.Runtime, "python"),
		filepath.Join(ws.Runtime, "zig"),
		filepath.Join(ws.Runtime, "scons"),
		filepath.Join(ws.Runtime, "godot_source"),
	}

	for _, dir := range nestedDirs {
		_, statErr := os.Stat(dir)
		assert.True(t, os.IsNotExist(statErr), "Runtime subdirectory should be created on-demand: %s", dir)
	}
}
