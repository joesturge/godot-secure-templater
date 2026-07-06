package windows

import (
	"path/filepath"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/config"
	"github.com/joemi/godot-secure-templater/internal/platform"
	"github.com/joemi/godot-secure-templater/internal/toolchain"
)

func init() {
	platform.Register(platform.Definition{
		ID:          "windows",
		TargetTuple: "windows/amd64",
		SupportedHostTuples: map[string]struct{}{
			"windows/amd64": {},
		},
		Components: func(version string) ([]internal.Artifact, *internal.Error) {
			return toolchain.WindowsComponents(version), nil
		},
		Compile: func(ctx *internal.RunContext, key string) *internal.Error {
			return builder.CompileTemplates(ctx, key)
		},
		ConfigureProject: func(projectRoot string, workspace *internal.Workspace, version string, key string, logger internal.Logger) *internal.Error {
			era, eraErr := config.VersionToEra(version)
			if eraErr != nil {
				return &internal.Error{Code: internal.ExitUnsupportedGodot, Message: "Could not determine credential era for this Godot version.", Details: eraErr.Error()}
			}

			presetsPath := filepath.Join(projectRoot, "export_presets.cfg")
			releasePath, debugPath := filepath.Join(workspace.Templates, "windows_template_release.exe"), filepath.Join(workspace.Templates, "windows_template_debug.exe")
			if err := config.InjectWindowsTemplate(presetsPath, releasePath, debugPath); err != nil {
				return err
			}

			credsPath, credPathErr := config.CredentialPath(projectRoot, era)
			if credPathErr != nil {
				return &internal.Error{Code: internal.ExitUnsupportedGodot, Message: "Could not determine credential path for this Godot version.", Details: credPathErr.Error()}
			}
			if err := config.InjectEncryptionKey(credsPath, key); err != nil {
				logger.Error("Credential injection failed: %v", err)
				return err
			}

			return nil
		},
		ArtifactPaths: func(workspace *internal.Workspace) (releasePath string, debugPath string) {
			return filepath.Join(workspace.Templates, "windows_template_release.exe"), filepath.Join(workspace.Templates, "windows_template_debug.exe")
		},
		SuccessNextSteps: func() []string {
			return []string{
				"Open your project in the Godot Editor",
				"Go to Project -> Export",
				"Export your game using the Windows preset",
			}
		},
	})
}
