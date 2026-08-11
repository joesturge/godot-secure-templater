package hostwindows

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
	// Windows builds use Zig directly and do not depend on MinGW wrappers.
	env["CC"] = "zig cc -target x86_64-windows-msvc"
	env["CXX"] = "zig c++ -target x86_64-windows-msvc"
	env["AR"] = "zig ar"
	// Strip host Visual Studio/Windows SDK toolchain variables so zig does not
	// inherit MSVC headers or libs from the runner.
	env["CL"] = ""
	env["INCLUDE"] = ""
	env["LIB"] = ""
	env["LIBPATH"] = ""
	env["VCINSTALLDIR"] = ""
	env["VCToolsInstallDir"] = ""
	env["WindowsSdkDir"] = ""
	env["UniversalCRTSdkDir"] = ""

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
		if v == "" {
			continue
		}
		filtered = append(filtered, fmt.Sprintf("%s=%s", k, v))
	}
	return filtered
}
