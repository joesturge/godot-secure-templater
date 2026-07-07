package targetprofiles

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal/builder"
)

func TestSConsHostTargetProfiles(t *testing.T) {
	// GIVEN current SCons target profile declarations

	// WHEN reading supported profiles
	profiles := SConsHostTargetProfiles()

	// THEN windows and linux targets should be declared
	assert.Len(t, profiles, 2, "SConsHostTargetProfiles should include two target profiles")
	assert.Equal(t, "windows/amd64", profiles[0].TargetTuple, "First profile should target windows/amd64")
	assert.Equal(t, "linux/amd64", profiles[1].TargetTuple, "Second profile should target linux/amd64")
}

func TestSConsTargetProfileTemplateNames(t *testing.T) {
	// GIVEN a windows target profile
	profile := SConsHostTargetProfiles()[0]

	// WHEN resolving source and destination names
	debugSource := profile.SourceTemplateName(builder.BuildDebug)
	releaseSource := profile.SourceTemplateName(builder.BuildRelease)
	debugDestination := profile.DestinationTemplateName(builder.BuildDebug)

	// THEN resolved names should match profile configuration
	assert.Equal(t, "godot.windows.template_debug.x86_64.exe", debugSource, "SourceTemplateName should map debug target to debug source template")
	assert.Equal(t, "godot.windows.template_release.x86_64.exe", releaseSource, "SourceTemplateName should map release target to release source template")
	assert.Equal(t, "windows_template_debug.exe", debugDestination, "DestinationTemplateName should apply destination format using BuildTarget values")
}
