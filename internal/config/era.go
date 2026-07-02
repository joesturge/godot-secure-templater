package config

import (
	"fmt"
	"strings"
)

// Era represents a Godot version era with different config requirements.
type Era string

const (
	Era43Plus  Era = "4.3+"    // 4.3 and later: export_credentials.cfg
	Era41To42  Era = "4.1-4.2" // 4.1 to 4.2: script_encryption_key in export_presets.cfg
	EraLegacy  Era = "<4.1"    // Pre-4.1: not supported
	EraUnknown Era = "unknown"
)

// VersionToEra determines which config era a Godot version belongs to.
// Returns Era and error if the version is unrecognized (fail closed).
func VersionToEra(version string) (Era, error) {
	if version == "" {
		return EraUnknown, fmt.Errorf("version is empty")
	}

	// Parse major.minor from version (e.g., "4.3.0" -> major=4, minor=3)
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return EraUnknown, fmt.Errorf("version format invalid: %s (expected X.Y.Z)", version)
	}

	var major, minor int
	_, err := fmt.Sscanf(version, "%d.%d", &major, &minor)
	if err != nil {
		return EraUnknown, fmt.Errorf("cannot parse version %s: %w", version, err)
	}

	// Version routing table (fail closed for unrecognized versions)
	if major == 4 {
		if minor >= 3 {
			return Era43Plus, nil
		}
		if minor == 1 || minor == 2 {
			return Era41To42, nil
		}
		if minor == 0 {
			return EraLegacy, fmt.Errorf("Godot 4.0 is not supported (template encryption requires 4.1+)")
		}
		// 4.x for x > 3: future minor version, fail closed
		return EraUnknown, fmt.Errorf("Godot 4.%d is not recognized; failing closed (update tool to support this version)", minor)
	}

	if major < 4 {
		return EraLegacy, fmt.Errorf("Godot %d.x is not supported (template encryption requires 4.1+)", major)
	}

	// Future major versions (e.g., 5.x): fail closed
	return EraUnknown, fmt.Errorf("Godot %d.%d is not recognized; failing closed (update tool to support this version)", major, minor)
}

// CredentialPath returns the file path where the encryption key should be written for the given era.
func CredentialPath(projectRoot string, era Era) (string, error) {
	switch era {
	case Era43Plus:
		// Godot 4.3+: dedicated credential file
		return joinPath(projectRoot, ".godot", "export_credentials.cfg"), nil

	case Era41To42:
		// Godot 4.1–4.2: embedded in presets file
		return joinPath(projectRoot, "export_presets.cfg"), nil

	case EraLegacy:
		return "", fmt.Errorf("encryption key storage not supported for Godot %s", era)

	default:
		return "", fmt.Errorf("unknown era: %s", era)
	}
}

// CredentialLineMarker returns the line marker or key name for the given era.
// This is used to identify where to inject or locate the key in the config.
func CredentialLineMarker(era Era) (string, error) {
	switch era {
	case Era43Plus:
		// .godot/export_credentials.cfg uses key=value format
		return "script_encryption_key=", nil

	case Era41To42:
		// export_presets.cfg uses [section] / key = value format
		return "script_encryption_key=", nil

	default:
		return "", fmt.Errorf("unknown era: %s", era)
	}
}

// Helper function for path joining
func joinPath(parts ...string) string {
	result := parts[0]
	for _, part := range parts[1:] {
		if result != "" && !strings.HasSuffix(result, "/") {
			result += "/"
		}
		result += part
	}
	return result
}
