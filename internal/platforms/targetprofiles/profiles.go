package targetprofiles

import (
	"fmt"

	"github.com/joemi/godot-secure-templater/internal/builder"
)

// SConsTargetProfile describes target-specific build and output settings.
type SConsTargetProfile struct {
	TargetTuple    string
	SConsPlatform  string
	SourceDebug    string
	SourceRelease  string
	DestinationFmt string
	PresetLabel    string
	ReleaseSetting string
	DebugSetting   string
	ExtraSConsArgs []string
}

func (p SConsTargetProfile) SourceTemplateName(target builder.BuildTarget) string {
	if target == builder.BuildDebug {
		return p.SourceDebug
	}
	return p.SourceRelease
}

func (p SConsTargetProfile) DestinationTemplateName(target builder.BuildTarget) string {
	return fmt.Sprintf(p.DestinationFmt, target)
}

// SConsHostTargetProfiles returns the currently supported SCons targets.
func SConsHostTargetProfiles() []SConsTargetProfile {
	return []SConsTargetProfile{
		{
			TargetTuple:    "windows/amd64",
			SConsPlatform:  "windows",
			SourceDebug:    "godot.windows.template_debug.x86_64.exe",
			SourceRelease:  "godot.windows.template_release.x86_64.exe",
			DestinationFmt: "windows_%s.exe",
			PresetLabel:    "Windows",
			ReleaseSetting: ".gst/templates/windows_template_release.exe",
			DebugSetting:   ".gst/templates/windows_template_debug.exe",
			ExtraSConsArgs: []string{"d3d12=no"},
		},
		{
			TargetTuple:    "linux/amd64",
			SConsPlatform:  "linuxbsd",
			SourceDebug:    "godot.linuxbsd.template_debug.x86_64",
			SourceRelease:  "godot.linuxbsd.template_release.x86_64",
			DestinationFmt: "linux_%s.x86_64",
			PresetLabel:    "Linux",
			ReleaseSetting: ".gst/templates/linux_template_release.x86_64",
			DebugSetting:   ".gst/templates/linux_template_debug.x86_64",
		},
	}
}
