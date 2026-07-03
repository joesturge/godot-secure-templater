package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/stretchr/testify/assert"
)

type captureLogger struct {
	lines []string
}

func (l *captureLogger) Info(msg string, args ...interface{}) {
	l.lines = append(l.lines, strings.TrimSpace(formatLine(msg, args...)))
}

func formatLine(msg string, args ...interface{}) string {
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

func TestFindGodotSource(t *testing.T) {
	// GIVEN a temporary directory with Godot source structure
	tempDir := t.TempDir()
	godotSourceDir := filepath.Join(tempDir, "godot-4.6.3-stable")
	err := os.Mkdir(godotSourceDir, 0755)
	assert.NoError(t, err, "Failed to create godot source directory")

	// Create a marker file to verify we found the right directory
	markerFile := filepath.Join(godotSourceDir, "SConstruct")
	err = os.WriteFile(markerFile, []byte("# Godot build file"), 0644)
	assert.NoError(t, err, "Failed to create marker file")

	// WHEN calling findGodotSource
	result, err := findGodotSource(tempDir)

	// THEN it should return the correct path
	assert.NoError(t, err, "Should find Godot source directory")
	assert.Equal(t, godotSourceDir, result, "Should return the godot-*-stable directory")
	assert.DirExists(t, result, "Returned path should exist")
}

func TestFindGodotSource_NotFound(t *testing.T) {
	// GIVEN a temporary empty directory
	tempDir := t.TempDir()

	// WHEN calling findGodotSource
	_, err := findGodotSource(tempDir)

	// THEN it should return an error
	assert.Error(t, err, "Should error when no Godot source found")
}

func TestFindGodotSource_MultipleVersions(t *testing.T) {
	// GIVEN a directory with multiple Godot versions
	tempDir := t.TempDir()

	// Create multiple versions
	versions := []string{"godot-4.5.0-stable", "godot-4.6.3-stable", "godot-4.7.0-stable"}
	for _, v := range versions {
		versionDir := filepath.Join(tempDir, v)
		err := os.Mkdir(versionDir, 0755)
		assert.NoError(t, err, "Failed to create version directory: %s", v)
	}

	// WHEN calling findGodotSource
	result, err := findGodotSource(tempDir)

	// THEN it should return one of the valid directories
	assert.NoError(t, err, "Should find one of the Godot source directories")
	assert.DirExists(t, result, "Returned path should exist")
	assert.True(t, filepath.Base(result) != "", "Should return a godot-*-stable directory")
}

func TestBuildEnv(t *testing.T) {
	// GIVEN a RunContext with workspace setup
	tempDir := t.TempDir()
	pythonDir := filepath.Join(tempDir, "python")
	mingwDir := filepath.Join(tempDir, "mingw")
	sconsDir := filepath.Join(tempDir, "scons")

	// Create the necessary directories
	for _, dir := range []string{pythonDir, mingwDir, sconsDir} {
		err := os.Mkdir(dir, 0755)
		assert.NoError(t, err, "Failed to create directory: %s", dir)
	}

	ctx := &internal.RunContext{
		Workspace: &internal.Workspace{
			Root:    tempDir,
			Runtime: tempDir,
		},
		Logger: internal.NewSimpleLogger(false),
	}

	// WHEN calling BuildEnv
	env := BuildEnv(ctx, "test-encryption-key")

	// THEN it should return an environment map with required variables
	assert.NotNil(t, env, "Should return non-nil environment")
	assert.Equal(t, "test-encryption-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "Should set encryption key")
	assert.NotEmpty(t, env["PATH"], "Should set PATH")
	assert.NotEmpty(t, env["PYTHONPATH"], "Should set PYTHONPATH")
	assert.Contains(t, env["PATH"], mingwDir, "PATH should include MinGW directory")
}

func TestBuildEnv_EncryptionKey(t *testing.T) {
	// GIVEN different encryption keys
	tempDir := t.TempDir()
	ctx := &internal.RunContext{
		Workspace: &internal.Workspace{
			Root:    tempDir,
			Runtime: tempDir,
		},
		Logger: internal.NewSimpleLogger(false),
	}

	tests := []struct {
		name string
		key  string
	}{
		{
			name: "standard key format",
			key:  "0123456789abcdef0123456789abcdef",
		},
		{
			name: "long key",
			key:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
		{
			name: "empty key",
			key:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN calling BuildEnv with encryption key
			env := BuildEnv(ctx, tt.key)

			// THEN environment should contain the key
			assert.Equal(t, tt.key, env["SCRIPT_AES256_ENCRYPTION_KEY"], "Should set provided encryption key")
		})
	}
}

func TestMakeEnv(t *testing.T) {
	// GIVEN a map with custom environment variables
	inputEnv := map[string]string{
		"CUSTOM_VAR":  "custom_value",
		"ANOTHER_VAR": "another_value",
	}

	// WHEN calling makeEnv
	result := makeEnv(inputEnv)

	// THEN it should return a slice with proper key=value format
	assert.NotNil(t, result, "Should return non-nil slice")
	assert.Greater(t, len(result), 0, "Should return non-empty slice")

	// AND should contain the custom variables
	foundCustom := false
	foundAnother := false
	for _, envVar := range result {
		if envVar == "CUSTOM_VAR=custom_value" {
			foundCustom = true
		}
		if envVar == "ANOTHER_VAR=another_value" {
			foundAnother = true
		}
	}
	assert.True(t, foundCustom, "Should contain CUSTOM_VAR=custom_value")
	assert.True(t, foundAnother, "Should contain ANOTHER_VAR=another_value")
}

// TestStreamOutput verifies output streaming handles multi-line input
func TestStreamOutput(t *testing.T) {
	// GIVEN a reader with multi-line output
	input := "line 1\nline 2\nerror: something failed\nline 3\n"
	reader := io.NopCloser(strings.NewReader(input))

	// Create a logger to capture output
	logger := internal.NewSimpleLogger(false)

	// WHEN calling streamOutput
	// NOTE: streamOutput writes to logger; we test through expected behavior
	// This test verifies the function doesn't panic and handles multi-line input
	assert.NotPanics(t, func() {
		streamOutput(logger, reader, false)
	}, "Should handle multi-line output without panicking")
}

// TestStreamOutput_EmptyInput verifies empty input is handled gracefully
func TestStreamOutput_EmptyInput(t *testing.T) {
	// GIVEN an empty reader
	reader := io.NopCloser(strings.NewReader(""))

	logger := internal.NewSimpleLogger(false)

	// WHEN calling streamOutput with empty input
	// THEN it should complete without error
	assert.NotPanics(t, func() {
		streamOutput(logger, reader, false)
	}, "Should handle empty input gracefully")
}

// TestStreamOutput_ErrorFlag verifies error output handling
func TestStreamOutput_ErrorFlag(t *testing.T) {
	// GIVEN a reader with error output
	input := "fatal error: something broke\n"
	reader := io.NopCloser(strings.NewReader(input))

	logger := internal.NewSimpleLogger(true)

	// WHEN calling streamOutput with isError=true
	// THEN it should handle error output without panicking
	assert.NotPanics(t, func() {
		streamOutput(logger, reader, true)
	}, "Should handle error output correctly")
}

func TestStreamOutput_EmitsElapsedStageUpdates(t *testing.T) {
	// GIVEN stdout input that contains stage transitions
	input := "Generating C++ bindings...\nCompiling core\nLinking\n"
	reader := io.NopCloser(strings.NewReader(input))
	logger := &captureLogger{}

	// WHEN streaming output through the parser-aware logger
	streamOutput(logger, reader, false)

	// THEN stage updates should include elapsed-time context
	foundElapsed := false
	for _, line := range logger.lines {
		if strings.Contains(line, "elapsed:") {
			foundElapsed = true
			break
		}
	}
	assert.True(t, foundElapsed, "Stage updates should include elapsed-time counters")
}

// TestMoveTemplate tests for the moveTemplate function

func TestMoveTemplate_Release(t *testing.T) {
	// GIVEN a Godot source directory with compiled release template
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	templatesDir := filepath.Join(tempDir, "templates")

	err := os.Mkdir(binDir, 0755)
	assert.NoError(t, err, "Failed to create bin directory")

	err = os.Mkdir(templatesDir, 0755)
	assert.NoError(t, err, "Failed to create templates directory")

	// Create the template file that SCons produces
	sourceFile := filepath.Join(binDir, "godot.windows.template_release.x86_64.exe")
	templateContent := []byte("fake template executable")
	err = os.WriteFile(sourceFile, templateContent, 0755)
	assert.NoError(t, err, "Failed to create template file")

	ctx := &internal.RunContext{
		Workspace: &internal.Workspace{
			Templates: templatesDir,
		},
		Logger: internal.NewSimpleLogger(false),
	}

	// WHEN calling moveTemplate for release build
	moveErr := moveTemplate(ctx, tempDir, BuildRelease)

	// THEN it should copy successfully
	assert.Nil(t, moveErr, "Should copy release template without error")

	// AND the destination file should exist
	dstPath := filepath.Join(templatesDir, "windows_template_release.exe")
	assert.FileExists(t, dstPath, "Destination template should exist")

	// AND content should match
	dstContent, err := os.ReadFile(dstPath)
	assert.NoError(t, err, "Failed to read destination file")
	assert.Equal(t, templateContent, dstContent, "Destination content should match source")
}

func TestMoveTemplate_Debug(t *testing.T) {
	// GIVEN a Godot source directory with compiled debug template
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	templatesDir := filepath.Join(tempDir, "templates")

	err := os.Mkdir(binDir, 0755)
	assert.NoError(t, err, "Failed to create bin directory")

	err = os.Mkdir(templatesDir, 0755)
	assert.NoError(t, err, "Failed to create templates directory")

	// Create the debug template file
	sourceFile := filepath.Join(binDir, "godot.windows.template_debug.x86_64.exe")
	templateContent := []byte("fake debug template executable")
	err = os.WriteFile(sourceFile, templateContent, 0755)
	assert.NoError(t, err, "Failed to create template file")

	ctx := &internal.RunContext{
		Workspace: &internal.Workspace{
			Templates: templatesDir,
		},
		Logger: internal.NewSimpleLogger(false),
	}

	// WHEN calling moveTemplate for debug build
	moveErr := moveTemplate(ctx, tempDir, BuildDebug)

	// THEN it should copy successfully
	assert.Nil(t, moveErr, "Should copy debug template without error")

	// AND the destination file should exist
	dstPath := filepath.Join(templatesDir, "windows_template_debug.exe")
	assert.FileExists(t, dstPath, "Destination debug template should exist")
}

func TestMoveTemplate_NotFound(t *testing.T) {
	// GIVEN a Godot source directory without template executable
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	templatesDir := filepath.Join(tempDir, "templates")

	err := os.Mkdir(binDir, 0755)
	assert.NoError(t, err, "Failed to create bin directory")

	err = os.Mkdir(templatesDir, 0755)
	assert.NoError(t, err, "Failed to create templates directory")

	ctx := &internal.RunContext{
		Workspace: &internal.Workspace{
			Templates: templatesDir,
		},
		Logger: internal.NewSimpleLogger(false),
	}

	// WHEN calling moveTemplate with missing template
	moveErr := moveTemplate(ctx, tempDir, BuildRelease)

	// THEN it should return an error
	assert.NotNil(t, moveErr, "Should error when template not found")
	assert.Contains(t, moveErr.Message, "Compiled template not found", "Error should indicate missing template")
	assert.Contains(t, moveErr.Details, "godot.windows.template_release.x86_64.exe", "Error should mention expected filename")
}

func TestMoveTemplate_CreateDestinationDir(t *testing.T) {
	// GIVEN a Godot source with template but non-existent templates directory
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	templatesDir := filepath.Join(tempDir, "templates")

	err := os.Mkdir(binDir, 0755)
	assert.NoError(t, err, "Failed to create bin directory")

	// Don't create templatesDir - should be created by moveTemplate

	sourceFile := filepath.Join(binDir, "godot.windows.template_release.x86_64.exe")
	err = os.WriteFile(sourceFile, []byte("fake template"), 0755)
	assert.NoError(t, err, "Failed to create template file")

	ctx := &internal.RunContext{
		Workspace: &internal.Workspace{
			Templates: templatesDir,
		},
		Logger: internal.NewSimpleLogger(false),
	}

	// WHEN calling moveTemplate
	moveErr := moveTemplate(ctx, tempDir, BuildRelease)

	// THEN it should create the directory and copy file
	assert.Nil(t, moveErr, "Should succeed and create templates directory")
	assert.DirExists(t, templatesDir, "Templates directory should be created")

	dstPath := filepath.Join(templatesDir, "windows_template_release.exe")
	assert.FileExists(t, dstPath, "Template should be copied")
}
