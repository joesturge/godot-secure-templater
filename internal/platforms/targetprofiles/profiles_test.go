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

	// THEN only the currently supported Windows target should be declared
	assert.Len(t, profiles, 1, "SConsHostTargetProfiles should include one target profile")
	assert.Equal(t, "windows/amd64", profiles[0].TargetTuple, "Profile should target windows/amd64")
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
