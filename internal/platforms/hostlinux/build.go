package hostlinux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/platforms/sconsworkflow"
	"github.com/joemi/godot-secure-templater/internal/platforms/targetprofiles"
)

var lookPathFn = exec.LookPath

func ensureHostDependencies() *internal.Error {
	if _, err := lookPathFn("pkg-config"); err == nil {
		return nil
	}

	return &internal.Error{
		Code:    internal.ExitBuildFailed,
		Message: "Missing required host dependency: pkg-config",
		Details: "Linux template builds require pkg-config for Godot's linuxbsd SCons checks. Install pkg-config (or pkgconf) on the host and rerun.\nUbuntu/Debian: sudo apt-get install -y pkg-config\nFedora: sudo dnf install -y pkgconf-pkg-config\nArch: sudo pacman -S pkgconf",
	}
}

func verifyCompileReadiness(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile) *internal.Error {
	if err := ensureHostDependencies(); err != nil {
		return err
	}
	return sconsworkflow.VerifyCompileReadiness(ctx, hostTuple, profile)
}

func buildCommandForProfile(profile targetprofiles.SConsTargetProfile) func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
	return func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
		if err := ensureHostDependencies(); err != nil {
			return nil, err
		}

		tools, err := sconsworkflow.ResolveRuntimeTools(ctx.Workspace, ctx.Logger)
		if err != nil {
			return nil, err
		}
		hostAdapter := sconsworkflow.AdapterForHostTuple(hostTuple)
		hostAdapter.NormalizeRuntimeTools(tools)

		pythonExe := tools.PythonExe
		sconsExe := tools.SConsExe
		godotSrc := tools.GodotSource

		ctx.Logger.Debug("Using Python: %s", pythonExe)
		ctx.Logger.Debug("Using SCons: %s", sconsExe)
		ctx.Logger.Debug("Godot source: %s", godotSrc)

		sconsArgs := []string{
			fmt.Sprintf("platform=%s", profile.SConsPlatform),
			fmt.Sprintf("target=%s", target),
			"dev_build=no",
			"optimize=speed",
		}
		if len(profile.ExtraSConsArgs) > 0 {
			sconsArgs = append(sconsArgs, profile.ExtraSConsArgs...)
		}

		cmd := sconsworkflow.BuildCommand(pythonExe, sconsExe, sconsArgs, ctx.Logger)
		cmd.Dir = godotSrc
		cmd.Env = makeEnv(buildEnv(ctx.Workspace, key))
		return cmd, nil
	}
}

func buildEnv(workspace *internal.Workspace, key string) map[string]string {
	hostAdapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	env := hostAdapter.BuildEnv(workspace, key)
	env["CC"] = "zig cc"
	env["CXX"] = "zig c++"
	env["AR"] = "zig ar"
	return env
}

func makeEnv(overrides map[string]string) []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		if _, ok := overrides[key]; !ok {
			filtered = append(filtered, e)
		}
	}
	for k, v := range overrides {
		filtered = append(filtered, fmt.Sprintf("%s=%s", k, v))
	}
	return filtered
}
