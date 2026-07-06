package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestDetectHostTuple(t *testing.T) {
	// GIVEN deterministic runtime tuple stubs
	oldOS := runtimeGOOS
	oldArch := runtimeGOARCH
	runtimeGOOS = "windows"
	runtimeGOARCH = "amd64"
	t.Cleanup(func() {
		runtimeGOOS = oldOS
		runtimeGOARCH = oldArch
	})

	// WHEN detecting the host tuple
	tuple := DetectHostTuple()

	// THEN the tuple should be goos/goarch
	assert.Equal(t, "windows/amd64", tuple, "DetectHostTuple should return normalized goos/goarch tuple")
}

func TestResolveTargetTuple(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		hostTuple string
		wantTuple string
		wantErr   bool
	}{
		{
			name:      "blank input defaults to host tuple",
			raw:       "",
			hostTuple: "windows/amd64",
			wantTuple: "windows/amd64",
			wantErr:   false,
		},
		{
			name:      "alias windows normalizes",
			raw:       "windows",
			hostTuple: "linux/amd64",
			wantTuple: "windows/amd64",
			wantErr:   false,
		},
		{
			name:      "invalid tuple returns typed error",
			raw:       "windows-amd64",
			hostTuple: "windows/amd64",
			wantTuple: "windows-amd64",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN tuple and host input

			// WHEN resolving a target tuple
			got, err := ResolveTargetTuple(tt.raw, tt.hostTuple)

			// THEN tuple normalization should match expected output
			assert.Equal(t, tt.wantTuple, got, "ResolveTargetTuple should return expected canonical tuple")

			// AND error state should match expectation
			if tt.wantErr {
				assert.NotNil(t, err, "ResolveTargetTuple should return typed error for invalid tuple format")
			} else {
				assert.Nil(t, err, "ResolveTargetTuple should succeed for supported tuple formats")
			}
		})
	}
}

func TestValidateHostSupport(t *testing.T) {
	def := Definition{
		ID:          "windows",
		TargetTuple: "windows/amd64",
		SupportedHostTuples: map[string]struct{}{
			"windows/amd64": {},
		},
		Components: func(version string) ([]internal.Artifact, *internal.Error) {
			return []internal.Artifact{}, nil
		},
		Compile: func(ctx *internal.RunContext, key string) *internal.Error {
			return nil
		},
		ArtifactPaths: func(workspace *internal.Workspace) (releasePath string, debugPath string) {
			return "", ""
		},
		SuccessNextSteps: func() []string {
			return []string{"dummy step"}
		},
	}

	// GIVEN an allowed host tuple
	allowedErr := ValidateHostSupport(def, "windows/amd64")
	// WHEN validating allowed tuple
	// THEN no error should be returned
	assert.Nil(t, allowedErr, "ValidateHostSupport should allow explicitly listed host tuple")

	// GIVEN a disallowed host tuple
	disallowedErr := ValidateHostSupport(def, "linux/amd64")
	// WHEN validating disallowed tuple
	// THEN a typed compatibility error should be returned
	assert.NotNil(t, disallowedErr, "ValidateHostSupport should fail for unsupported host-target combination")
	assert.Equal(t, internal.ExitUsageError, disallowedErr.Code, "Unsupported host-target tuple should map to usage error exit code")
}
