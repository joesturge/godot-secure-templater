package internal

import (
	"fmt"
)

// ExitCode represents the stable exit code contract for CI [Slice 2].
type ExitCode int

const (
	ExitSuccess ExitCode = iota
	ExitGenericFailure
	ExitUsageError
	ExitNotGodotProject
	ExitVersionResolution
	ExitChecksumMismatch
	ExitInsufficientDisk
	ExitBuildFailed
	ExitConfigInjectionFailed
	ExitUnsupportedGodot
	ExitLockHeld
)

// Error is the base error type for all typed errors in the tool.
type Error struct {
	Code    ExitCode
	Message string
	Details string // optional details for logging
}

func (e *Error) Error() string {
	if e == nil {
		return "unknown error"
	}
	if e.Details != "" {
		return fmt.Sprintf("%s\n%s", e.Message, e.Details)
	}
	return e.Message
}

// Errors by concern.

var (
	ErrNotGodotProject = &Error{
		Code:    ExitNotGodotProject,
		Message: "No `project.godot` found. Run this from your Godot project root.",
	}

	ErrProjectVersionUnreadable = &Error{
		Code:    ExitNotGodotProject,
		Message: "Could not read Godot version from `project.godot`.",
		Details: "Ensure `config/features` contains a version string like `PackedStringArray(\"4.3\", ...)`.",
	}

	ErrMinorMismatch = func(projectMinor, resolvedMinor string) *Error {
		return &Error{
			Code:    ExitVersionResolution,
			Message: fmt.Sprintf("Version mismatch: project targets Godot %s but --godot-version=%s was supplied (different minor line).", projectMinor, resolvedMinor),
			Details: "Compatibility guarantees end at the minor line. Patch differences are tolerated.",
		}
	}

	ErrVersionUnresolved = &Error{
		Code:    ExitVersionResolution,
		Message: "Could not determine the Godot patch version.",
		Details: "Pass --godot-version=X.Y.Z to specify it explicitly.",
	}

	ErrChecksumMismatch = func(artifact string, expected, got string) *Error {
		return &Error{
			Code:    ExitChecksumMismatch,
			Message: fmt.Sprintf("Integrity check failed for `%s`.", artifact),
			Details: fmt.Sprintf("Expected SHA-256: %s\nGot:              %s\nToolchain may be tampered. Aborting.", expected, got),
		}
	}

	ErrInsufficientDisk = func(needed, available uint64, volume string) *Error {
		return &Error{
			Code:    ExitInsufficientDisk,
			Message: fmt.Sprintf("Insufficient disk space on %s.", volume),
			Details: fmt.Sprintf("Need ~%d GB, found %d GB available.", needed/(1024*1024*1024), available/(1024*1024*1024)),
		}
	}

	ErrBuildFailed = func(stage string, logPath string) *Error {
		return &Error{
			Code:    ExitBuildFailed,
			Message: fmt.Sprintf("SCons build failed at stage: %s.", stage),
			Details: fmt.Sprintf("See full log: %s", logPath),
		}
	}

	ErrConfigStructureUnrecognized = &Error{
		Code:    ExitConfigInjectionFailed,
		Message: "Could not locate expected sections in export configuration.",
		Details: "Refusing to edit to avoid corruption. Check your export_presets.cfg file.",
	}

	ErrUnsupportedGodot = func(version string) *Error {
		return &Error{
			Code:    ExitUnsupportedGodot,
			Message: fmt.Sprintf("Godot %s is not supported by this tool.", version),
			Details: "Slice 0 supports Godot 4.3+ only.",
		}
	}

	ErrLockHeld = func(pid, host string) *Error {
		return &Error{
			Code:    ExitLockHeld,
			Message: "Another run holds the lock.",
			Details: fmt.Sprintf("PID: %s, Host: %s. Wait for it to finish or remove .gst/.lock if stale.", pid, host),
		}
	}

	ErrUnsupportedPlatformTuple = func(tuple string) *Error {
		return &Error{
			Code:    ExitUsageError,
			Message: fmt.Sprintf("Unsupported platform tuple: %s", tuple),
			Details: "Expected tuple format is os/arch (for example, windows/amd64 or linux/amd64). This value defaults to detected host tuple (GOOS/GOARCH).",
		}
	}

	ErrUnknownPlatform = func(platformID string) *Error {
		return &Error{
			Code:    ExitUsageError,
			Message: fmt.Sprintf("Unknown platform: %s", platformID),
			Details: "No platform plugin is registered for the requested target.",
		}
	}

	ErrHostTargetUnsupported = func(hostTuple, targetTuple string) *Error {
		return &Error{
			Code:    ExitUsageError,
			Message: fmt.Sprintf("Host tuple %s cannot build target %s", hostTuple, targetTuple),
			Details: "Select a compatible host/target pair or add support for this tuple in the platform registry.",
		}
	}

	ErrPlatformNotImplemented = func(platformID string) *Error {
		return &Error{
			Code:    ExitUsageError,
			Message: fmt.Sprintf("Platform %s is registered but not yet implemented", platformID),
			Details: "The platform exists in the registry, but build/provision logic has not been implemented yet.",
		}
	}
)
