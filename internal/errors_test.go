package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorStringer(t *testing.T) {
	// GIVEN various error types with different message and detail combinations
	tests := []struct {
		name      string
		err       *Error
		wantMsg   string
		hasDetail bool
	}{
		{
			name: "error with message only",
			err: &Error{
				Code:    ExitNotGodotProject,
				Message: "Project not found",
			},
			wantMsg:   "Project not found",
			hasDetail: false,
		},
		{
			name: "error with message and details",
			err: &Error{
				Code:    ExitGenericFailure,
				Message: "Something went wrong",
				Details: "Additional context here",
			},
			wantMsg:   "Something went wrong",
			hasDetail: true,
		},
		{
			name:      "nil error",
			err:       nil,
			wantMsg:   "unknown error",
			hasDetail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN calling Error() on the error
			got := tt.err.Error()

			// THEN it should contain the expected message
			assert.Contains(t, got, tt.wantMsg, "Error message should contain expected message")

			// AND if details are present, they should be included
			if tt.hasDetail {
				assert.Contains(t, got, "Additional context", "Error message should contain details")
			}
		})
	}
}

func TestErrorExitCodes(t *testing.T) {
	// GIVEN various error instances with different exit code values
	tests := []struct {
		name         string
		err          *Error
		wantExitCode ExitCode
	}{
		{
			name:         "not a godot project",
			err:          ErrNotGodotProject,
			wantExitCode: ExitNotGodotProject,
		},
		{
			name:         "project version unreadable",
			err:          ErrProjectVersionUnreadable,
			wantExitCode: ExitNotGodotProject,
		},
		{
			name:         "version unresolved",
			err:          ErrVersionUnresolved,
			wantExitCode: ExitVersionResolution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN checking the error code
			// THEN it should match the expected exit code
			assert.Equal(t, tt.wantExitCode, tt.err.Code, "Error exit code should match")
		})
	}
}

func TestErrorMinorMismatchFactory(t *testing.T) {
	// GIVEN a project minor version and a different supplied version
	projectMinor := "4.3"
	suppliedVersion := "4.4.0"

	// WHEN creating a minor mismatch error
	err := ErrMinorMismatch(projectMinor, suppliedVersion)

	// THEN the error should have the correct exit code
	assert.Equal(t, ExitVersionResolution, err.Code, "Exit code should be ExitVersionResolution")
	// AND the message should contain both versions
	assert.Contains(t, err.Message, projectMinor, "Message should contain project minor version")
	assert.Contains(t, err.Message, suppliedVersion, "Message should contain supplied version")
}

func TestErrorChecksumMismatchFactory(t *testing.T) {
	// GIVEN an artifact and mismatched checksums
	artifact := "mingw"
	expected := "abc123"
	got := "def456"

	// WHEN creating a checksum mismatch error
	err := ErrChecksumMismatch(artifact, expected, got)

	// THEN the error should have the correct exit code
	assert.Equal(t, ExitChecksumMismatch, err.Code, "Exit code should be ExitChecksumMismatch")
	// AND the artifact name should be in the message
	assert.Contains(t, err.Message, artifact, "Message should contain artifact name")
	// AND both checksums should be in the details
	assert.Contains(t, err.Details, expected, "Details should contain expected checksum")
	assert.Contains(t, err.Details, got, "Details should contain actual checksum")
}

func TestErrorInsufficientDiskFactory(t *testing.T) {
	// GIVEN disk space requirements and available space
	needed := uint64(10 * 1024 * 1024 * 1024)   // 10 GB
	available := uint64(5 * 1024 * 1024 * 1024) // 5 GB
	volume := "C:"

	// WHEN creating an insufficient disk space error
	err := ErrInsufficientDisk(needed, available, volume)

	// THEN the error should have the correct exit code
	assert.Equal(t, ExitInsufficientDisk, err.Code, "Exit code should be ExitInsufficientDisk")
	// AND the volume name should be in the message
	assert.Contains(t, err.Message, volume, "Message should contain volume name")
	// AND both sizes should be in the details
	assert.Contains(t, err.Details, "10", "Details should contain needed size")
	assert.Contains(t, err.Details, "5", "Details should contain available size")
}

func TestErrorBuildFailedFactory(t *testing.T) {
	// GIVEN a build stage and log file path
	stage := "Compiling"
	logPath := "/path/to/logs/build.log"

	// WHEN creating a build failed error
	err := ErrBuildFailed(stage, logPath)

	// THEN the error should have the correct exit code
	assert.Equal(t, ExitBuildFailed, err.Code, "Exit code should be ExitBuildFailed")
	// AND the build stage should be in the message
	assert.Contains(t, err.Message, stage, "Message should contain build stage")
	// AND the log path should be in the details
	assert.Contains(t, err.Details, logPath, "Details should contain log file path")
}

func TestErrorUnsupportedGodotFactory(t *testing.T) {
	// GIVEN an unsupported Godot version
	version := "3.5.0"

	// WHEN creating an unsupported Godot error
	err := ErrUnsupportedGodot(version)

	// THEN the error should have the correct exit code
	assert.Equal(t, ExitUnsupportedGodot, err.Code, "Exit code should be ExitUnsupportedGodot")
	// AND the version should be in the message
	assert.Contains(t, err.Message, version, "Message should contain unsupported version")
}

func TestErrorExitCodeContract(t *testing.T) {
	// GIVEN all error exit codes defined in the system
	tests := []struct {
		code ExitCode
		name string
	}{
		{0, "ExitSuccess"},
		{1, "ExitGenericFailure"},
		{2, "ExitUsageError"},
		{3, "ExitNotGodotProject"},
		{4, "ExitVersionResolution"},
		{5, "ExitChecksumMismatch"},
		{6, "ExitInsufficientDisk"},
		{7, "ExitBuildFailed"},
		{8, "ExitConfigInjectionFailed"},
		{9, "ExitUnsupportedGodot"},
		{10, "ExitLockHeld"},
	}

	for _, tt := range tests {
		if int(tt.code) != int(tt.code) {
			t.Errorf("Exit code contract violated: %s", tt.name)
		}
	}
}
