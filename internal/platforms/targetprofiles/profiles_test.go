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

	// THEN windows, linux, and web targets should be declared for host-specific registration
	assert.Len(t, profiles, 3, "SConsHostTargetProfiles should include three target profiles")
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
	assert.Contains(t, profile.ExtraSConsArgs, "use_llvm=yes", "Windows profile should force LLVM mode in the MinGW compiler path")
	assert.Contains(t, profile.ExtraSConsArgs, "use_mingw=yes", "Windows profile should stay on the MinGW compiler path")
	assert.Contains(t, profile.ExtraSConsArgs, "d3d12=no", "Windows profile should disable D3D12 to avoid host Windows SDK dependency")
}

func TestWebTargetProfile(t *testing.T) {
	// GIVEN the supported SCons target profiles
	profiles := SConsHostTargetProfiles()

	// WHEN finding the Web profile
	var webProfile SConsTargetProfile
	for _, profile := range profiles {
		if profile.TargetTuple == "web/wasm32" {
			webProfile = profile
			break
		}
	}

	// THEN Web should use Godot's web platform and official template names
	assert.Equal(t, "web/wasm32", webProfile.TargetTuple, "Web profile should use the web/wasm32 target tuple")
	assert.Equal(t, "web", webProfile.SConsPlatform, "Web profile should use Godot's web SCons platform")
	assert.Equal(t, "godot.web.template_debug.wasm32.zip", webProfile.SourceDebug, "Web debug source should use Godot's wasm32 template name")
	assert.Equal(t, "godot.web.template_release.wasm32.zip", webProfile.SourceRelease, "Web release source should use Godot's wasm32 template name")
	assert.Equal(t, "web_%s.zip", webProfile.DestinationFmt, "Web profile should use concise output archive names")
}

func TestSConsLinuxProfileTemplateNames(t *testing.T) {
	// GIVEN a linux target profile
	profile := SConsHostTargetProfiles()[1]

	// WHEN resolving source and destination names
	debugSource := profile.SourceTemplateName(builder.BuildDebug)
	releaseSource := profile.SourceTemplateName(builder.BuildRelease)
	releaseDestination := profile.DestinationTemplateName(builder.BuildRelease)

	// THEN resolved names should match current Godot Linux profile configuration
	assert.Equal(t, "linuxbsd", profile.SConsPlatform, "Linux profile should use Godot's linuxbsd SCons platform name")
	assert.Equal(t, "godot.linuxbsd.template_debug.x86_64", debugSource, "SourceTemplateName should map debug target to linux debug source template")
	assert.Equal(t, "godot.linuxbsd.template_release.x86_64", releaseSource, "SourceTemplateName should map release target to linux release source template")
	assert.Equal(t, "linux_template_release.x86_64", releaseDestination, "DestinationTemplateName should apply destination format using BuildTarget values")
}
