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
	sourceTemplateName := func(target BuildTarget) string {
		if target == BuildDebug {
			return "godot.windows.template_debug.x86_64.exe"
		}
		return "godot.windows.template_release.x86_64.exe"
	}
	destinationTemplateName := func(target BuildTarget) string {
		return fmt.Sprintf("windows_%s.exe", target)
	}

	// WHEN calling moveTemplate for release build
	moveErr := moveTemplate(ctx, tempDir, BuildRelease, sourceTemplateName, destinationTemplateName)

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
	sourceTemplateName := func(target BuildTarget) string {
		if target == BuildDebug {
			return "godot.windows.template_debug.x86_64.exe"
		}
		return "godot.windows.template_release.x86_64.exe"
	}
	destinationTemplateName := func(target BuildTarget) string {
		return fmt.Sprintf("windows_%s.exe", target)
	}

	// WHEN calling moveTemplate for debug build
	moveErr := moveTemplate(ctx, tempDir, BuildDebug, sourceTemplateName, destinationTemplateName)

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
	sourceTemplateName := func(target BuildTarget) string {
		if target == BuildDebug {
			return "godot.windows.template_debug.x86_64.exe"
		}
		return "godot.windows.template_release.x86_64.exe"
	}
	destinationTemplateName := func(target BuildTarget) string {
		return fmt.Sprintf("windows_%s.exe", target)
	}

	// WHEN calling moveTemplate with missing template
	moveErr := moveTemplate(ctx, tempDir, BuildRelease, sourceTemplateName, destinationTemplateName)

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
	sourceTemplateName := func(target BuildTarget) string {
		if target == BuildDebug {
			return "godot.windows.template_debug.x86_64.exe"
		}
		return "godot.windows.template_release.x86_64.exe"
	}
	destinationTemplateName := func(target BuildTarget) string {
		return fmt.Sprintf("windows_%s.exe", target)
	}

	// WHEN calling moveTemplate
	moveErr := moveTemplate(ctx, tempDir, BuildRelease, sourceTemplateName, destinationTemplateName)

	// THEN it should create the directory and copy file
	assert.Nil(t, moveErr, "Should succeed and create templates directory")
	assert.DirExists(t, templatesDir, "Templates directory should be created")

	dstPath := filepath.Join(templatesDir, "windows_template_release.exe")
	assert.FileExists(t, dstPath, "Template should be copied")
}
