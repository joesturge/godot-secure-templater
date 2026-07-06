package windows

import (
	"path/filepath"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/platform"
)

func init() {
	platform.Register(platform.Definition{
		ID:          "windows",
		TargetTuple: "windows/amd64",
		SupportedHostTuples: map[string]struct{}{
			"windows/amd64": {},
		},
		Components: func(version string) ([]internal.Artifact, *internal.Error) {
			return Components(version), nil
		},
		Compile: func(ctx *internal.RunContext, key string) *internal.Error {
			return builder.CompileTemplates(ctx, key, buildCommand, sourceTemplateName, destinationTemplateName)
		},
		ArtifactPaths: func(workspace *internal.Workspace) (releasePath string, debugPath string) {
			return filepath.Join(workspace.Templates, "windows_template_release.exe"), filepath.Join(workspace.Templates, "windows_template_debug.exe")
		},
		SuccessNextSteps: func() []string {
			return []string{
				"Open your project in the Godot Editor",
				"Create or edit the Windows export preset",
				"Set the release template to .gst/templates/windows_template_release.exe",
				"Set the debug template to .gst/templates/windows_template_debug.exe",
				"Copy the encryption key from .gst/encryption.key into the preset or credentials file as appropriate for your Godot version",
			}
		},
	})
}
