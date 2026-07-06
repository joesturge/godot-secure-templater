package linux

import (
	"path/filepath"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/platform"
)

func init() {
	platform.Register(platform.Definition{
		ID:          "linux",
		TargetTuple: "linux/amd64",
		SupportedHostTuples: map[string]struct{}{
			"linux/amd64":   {},
			"windows/amd64": {},
		},
		Components: func(version string) ([]internal.Artifact, *internal.Error) {
			return nil, internal.ErrPlatformNotImplemented("linux")
		},
		Compile: func(ctx *internal.RunContext, key string) *internal.Error {
			return internal.ErrPlatformNotImplemented("linux")
		},
		ArtifactPaths: func(workspace *internal.Workspace) (releasePath string, debugPath string) {
			return filepath.Join(workspace.Templates, "linux_template_release"), filepath.Join(workspace.Templates, "linux_template_debug")
		},
		SuccessNextSteps: func() []string {
			return []string{
				"Linux target is registered but not implemented yet",
				"Implement linux compile/config hooks before running export workflow",
			}
		},
	})
}
