package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionToEra(t *testing.T) {
	// GIVEN various Godot versions
	tests := []struct {
		name      string
		version   string
		wantEra   Era
		wantError bool
	}{
		{
			name:      "4.3.0",
			version:   "4.3.0",
			wantEra:   Era43Plus,
			wantError: false,
		},
		{
			name:      "4.3.1",
			version:   "4.3.1",
			wantEra:   Era43Plus,
			wantError: false,
		},
		{
			name:      "4.4.0",
			version:   "4.4.0",
			wantEra:   Era43Plus,
			wantError: false,
		},
		{
			name:      "5.0.0 (future)",
			version:   "5.0.0",
			wantEra:   EraUnknown,
			wantError: true,
		},
		{
			name:      "4.2.0",
			version:   "4.2.0",
			wantEra:   Era41To42,
			wantError: false,
		},
		{
			name:      "4.1.0",
			version:   "4.1.0",
			wantEra:   Era41To42,
			wantError: false,
		},
		{
			name:      "4.0.0 (too old)",
			version:   "4.0.0",
			wantEra:   EraLegacy,
			wantError: true,
		},
		{
			name:      "3.x.x (too old)",
			version:   "3.5.3",
			wantEra:   EraLegacy,
			wantError: true,
		},
		{
			name:      "empty version",
			version:   "",
			wantEra:   EraUnknown,
			wantError: true,
		},
		{
			name:      "malformed version",
			version:   "latest",
			wantEra:   EraUnknown,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN converting version to era
			got, err := VersionToEra(tt.version)

			// THEN result should match expectations
			if tt.wantError {
				assert.NotNil(t, err, "VersionToEra should error")
			} else {
				assert.Nil(t, err, "VersionToEra should not error")
				assert.Equal(t, tt.wantEra, got)
			}
		})
	}
}

func TestCredentialPath(t *testing.T) {
	// GIVEN various eras
	tests := []struct {
		name        string
		era         Era
		projectRoot string
		wantPath    string
		wantError   bool
	}{
		{
			name:        "4.3+: export_credentials.cfg in .godot/",
			era:         Era43Plus,
			projectRoot: "/home/user/MyGame",
			wantPath:    "/home/user/MyGame/.godot/export_credentials.cfg",
			wantError:   false,
		},
		{
			name:        "4.1-4.2: export_presets.cfg in root",
			era:         Era41To42,
			projectRoot: "/home/user/MyGame",
			wantPath:    "/home/user/MyGame/export_presets.cfg",
			wantError:   false,
		},
		{
			name:        "legacy: not supported",
			era:         EraLegacy,
			projectRoot: "/home/user/MyGame",
			wantPath:    "",
			wantError:   true,
		},
		{
			name:        "unknown: error",
			era:         EraUnknown,
			projectRoot: "/home/user/MyGame",
			wantPath:    "",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN getting credential path
			got, err := CredentialPath(tt.projectRoot, tt.era)

			// THEN result should match expectations
			if tt.wantError {
				assert.NotNil(t, err, "CredentialPath should error")
			} else {
				assert.Nil(t, err, "CredentialPath should not error")
				assert.Equal(t, tt.wantPath, got)
			}
		})
	}
}

func TestCredentialLineMarker(t *testing.T) {
	// GIVEN various eras
	tests := []struct {
		name       string
		era        Era
		wantMarker string
		wantError  bool
	}{
		{
			name:       "4.3+: script_encryption_key=",
			era:        Era43Plus,
			wantMarker: "script_encryption_key=",
			wantError:  false,
		},
		{
			name:       "4.1-4.2: script_encryption_key=",
			era:        Era41To42,
			wantMarker: "script_encryption_key=",
			wantError:  false,
		},
		{
			name:       "legacy: not supported",
			era:        EraLegacy,
			wantMarker: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN getting credential line marker
			got, err := CredentialLineMarker(tt.era)

			// THEN result should match expectations
			if tt.wantError {
				assert.NotNil(t, err, "CredentialLineMarker should error")
			} else {
				assert.Nil(t, err, "CredentialLineMarker should not error")
				assert.Equal(t, tt.wantMarker, got)
			}
		})
	}
}

func TestVersionToEraRanges(t *testing.T) {
	// GIVEN boundary versions
	tests := []struct {
		name    string
		version string
		wantEra Era
	}{
		{
			name:    "4.3.0 boundary (first of 4.3+)",
			version: "4.3.0",
			wantEra: Era43Plus,
		},
		{
			name:    "4.2.9 boundary (last of 4.1-4.2)",
			version: "4.2.9",
			wantEra: Era41To42,
		},
		{
			name:    "4.1.0 boundary (first of 4.1-4.2)",
			version: "4.1.0",
			wantEra: Era41To42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN checking version era
			got, err := VersionToEra(tt.version)

			// THEN should match expected era
			assert.Nil(t, err)
			assert.Equal(t, tt.wantEra, got)
		})
	}
}
