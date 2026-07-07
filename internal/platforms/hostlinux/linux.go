package hostlinux

import (
	"path/filepath"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/platform"
	"github.com/joemi/godot-secure-templater/internal/platforms/targetprofiles"
)

const hostTuple = "linux/amd64"

func init() {
	for _, profile := range targetprofiles.SConsHostTargetProfiles() {
		if profile.TargetTuple != "linux/amd64" {
			continue
		}
		platform.Register(platform.Definition{
			HostTuple:   hostTuple,
			TargetTuple: profile.TargetTuple,
			Components: func(version string) ([]internal.Artifact, *internal.Error) {
				return Components(version), nil
			},
			Compile: func(ctx *internal.RunContext, key string) *internal.Error {
				return builder.CompileTemplates(
					ctx,
					key,
					buildCommandForProfile(profile),
					profile.SourceTemplateName,
					profile.DestinationTemplateName,
				)
			},
			ArtifactPaths: func(workspace *internal.Workspace) (releasePath string, debugPath string) {
				return filepath.Join(workspace.Templates, profile.DestinationTemplateName(builder.BuildRelease)), filepath.Join(workspace.Templates, profile.DestinationTemplateName(builder.BuildDebug))
			},
			SuccessNextSteps: func() []string {
				return []string{
					"Open your project in the Godot Editor",
					"Create or edit the " + profile.PresetLabel + " export preset",
					"Set the release template to " + profile.ReleaseSetting,
					"Set the debug template to " + profile.DebugSetting,
					"Copy the encryption key from .gst/encryption.key into the preset or credentials file as appropriate for your Godot version",
				}
			},
		})
	}
}
