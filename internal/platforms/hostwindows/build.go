package hostwindows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/platforms/sconsworkflow"
	"github.com/joemi/godot-secure-templater/internal/platforms/targetprofiles"
)

func buildCommandForProfile(profile targetprofiles.SConsTargetProfile) func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
	return func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
		tools, err := sconsworkflow.ResolveRuntimeTools(ctx.Workspace, ctx.Logger)
		if err != nil {
			return nil, err
		}
		hostAdapter := sconsworkflow.WindowsHostAdapter()
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

func verifyCompileReadiness(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile) *internal.Error {
	return sconsworkflow.VerifyCompileReadinessWithEnv(ctx, hostTuple, profile, buildEnv(ctx.Workspace, "verify-only"))
}

func buildEnv(workspace *internal.Workspace, key string) map[string]string {
	hostAdapter := sconsworkflow.WindowsHostAdapter()
	env := hostAdapter.BuildEnv(workspace, key)
	shimRoot := filepath.Join(workspace.Runtime, "zig-shims")
	shimBin := filepath.Join(shimRoot, "bin")
	env["PATH"] = prependWindowsPath(shimBin, env["PATH"])
	env["MINGW_PREFIX"] = shimRoot
	// Zig-backed MinGW-compatible shims keep the build off external MinGW.
	env["CC"] = "clang"
	env["CXX"] = "clang++"
	env["AR"] = "ar"

	return env
}

func prependWindowsPath(entry string, existing string) string {
	if existing == "" {
		return entry
	}
	if strings.HasPrefix(existing, entry+";") || existing == entry {
		return existing
	}
	return entry + ";" + existing
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
