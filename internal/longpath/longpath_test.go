package longpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxPathLengthWindows(t *testing.T) {
	// GIVEN a Windows checker
	checker := NewChecker("windows")

	// WHEN getting max path length
	max := checker.MaxPathLength()

	// THEN should return Windows MAX_PATH (260)
	assert.Equal(t, 260, max)
}

func TestMaxPathLengthPosix(t *testing.T) {
	// GIVEN a POSIX checker
	checker := NewChecker("linux")

	// WHEN getting max path length
	max := checker.MaxPathLength()

	// THEN should return POSIX typical (4096)
	assert.Equal(t, 4096, max)
}

func TestCheckPathWithinLimit(t *testing.T) {
	// GIVEN a Windows checker and a short path
	checker := NewChecker("windows")
	path := "C:\\Users\\dev\\project"

	// WHEN checking the path
	warning, err := checker.CheckPath(path)

	// THEN no error or warning should occur
	assert.Nil(t, err)
	assert.Empty(t, warning)
}

func TestCheckPathApproachingLimit(t *testing.T) {
	// GIVEN a Windows checker and a long path (approaching 260)
	checker := NewChecker("windows")
	// Create a path that's 90%+ of 260 (234+ chars)
	path := "C:\\" + string(make([]byte, 235)) // Total 238 chars (> 234)

	// WHEN checking the path
	warning, err := checker.CheckPath(path)

	// THEN a warning should be returned
	assert.Nil(t, err)
	assert.NotEmpty(t, warning, "should warn when approaching limit")
	assert.Contains(t, warning, "nearing Windows MAX_PATH limit")
}

func TestCheckPathExceedsLimit(t *testing.T) {
	// GIVEN a Windows checker and a path exceeding 260 chars
	checker := NewChecker("windows")
	path := "C:\\" + string(make([]byte, 260)) // Total 265 chars

	// WHEN checking the path
	warning, err := checker.CheckPath(path)

	// THEN an error should be returned
	assert.NotNil(t, err, "should error when exceeding limit")
	assert.Empty(t, warning)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestExtendedLengthPathWindows(t *testing.T) {
	// GIVEN a Windows checker and a local path
	checker := NewChecker("windows")
	path := "C:\\Users\\dev\\project\\file.txt"

	// WHEN extending the path
	extended := checker.ExtendedLengthPath(path)

	// THEN it should have the \\?\ prefix
	assert.Equal(t, "\\\\?\\C:\\Users\\dev\\project\\file.txt", extended)
}

func TestExtendedLengthPathPosix(t *testing.T) {
	// GIVEN a POSIX checker and a path
	checker := NewChecker("linux")
	path := "/home/dev/project/file.txt"

	// WHEN extending the path (no-op on POSIX)
	extended := checker.ExtendedLengthPath(path)

	// THEN it should remain unchanged
	assert.Equal(t, path, extended)
}

func TestNeedsPrefixingWindows(t *testing.T) {
	// GIVEN a Windows checker
	checker := NewChecker("windows")

	tests := []struct {
		name     string
		path     string
		wantNeed bool
	}{
		{
			name:     "short path",
			path:     "C:\\Users\\dev\\file.txt",
			wantNeed: false,
		},
		{
			name:     "long path (200+ chars)",
			path:     "C:\\" + string(make([]byte, 200)),
			wantNeed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN checking if prefixing is needed
			got := checker.NeedsPrefixing(tt.path)

			// THEN result should match expectation
			assert.Equal(t, tt.wantNeed, got)
		})
	}
}

func TestNeedsPrefixingPosix(t *testing.T) {
	// GIVEN a POSIX checker
	checker := NewChecker("linux")

	// WHEN checking a long path (should never need prefixing on POSIX)
	path := "/home/dev/" + string(make([]byte, 200))
	got := checker.NeedsPrefixing(path)

	// THEN should return false
	assert.False(t, got, "POSIX should never need long-path prefixing")
}

func TestDiagnosticMessageWindows(t *testing.T) {
	// GIVEN a Windows checker
	checker := NewChecker("windows")

	// WHEN getting diagnostic message
	msg := checker.DiagnosticMessage()

	// THEN should contain Windows-specific guidance
	assert.NotEmpty(t, msg)
	assert.Contains(t, msg, "260")
	assert.Contains(t, msg, "Long Path support")
}

func TestDiagnosticMessagePosix(t *testing.T) {
	// GIVEN a POSIX checker
	checker := NewChecker("linux")

	// WHEN getting diagnostic message
	msg := checker.DiagnosticMessage()

	// THEN should return empty (no guidance needed on POSIX)
	assert.Empty(t, msg)
}

func TestCheckPathPosixNoLimit(t *testing.T) {
	// GIVEN a POSIX checker and a very long path
	checker := NewChecker("linux")
	path := "/home/dev/" + string(make([]byte, 4000)) // Well within POSIX limit

	// WHEN checking
	warning, err := checker.CheckPath(path)

	// THEN no error should occur (POSIX is more lenient)
	assert.Nil(t, err)
	assert.Empty(t, warning)
}

func TestIsLongPathsEnabledNonWindowsPlatform(t *testing.T) {
	// GIVEN a non-Windows checker
	checker := NewChecker("linux")

	// WHEN checking registry status
	enabled, err := checker.IsLongPathsEnabled()

	// THEN it should return false without error
	assert.NoError(t, err, "Non-Windows platform checks should not error")
	assert.False(t, enabled, "Long path registry state should be false on non-Windows platforms")
}

func TestIsLongPathsEnabledWindowsCheckerOnNonWindowsRuntime(t *testing.T) {
	// GIVEN a Windows checker running on a non-Windows runtime in tests
	checker := NewChecker("windows")

	// WHEN checking registry status
	enabled, err := checker.IsLongPathsEnabled()

	// THEN it should degrade gracefully without error
	assert.NoError(t, err, "Windows registry probe should degrade gracefully on non-Windows runtime")
	assert.False(t, enabled, "LongPathsEnabled should be false when runtime is not Windows")
}
