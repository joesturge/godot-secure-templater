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

func verifyCompileReadiness(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile) *internal.Error {
	compilerEnv, err := zigCompilerEnv(ctx.Workspace.Runtime)
	if err != nil {
		return err
	}
	return sconsworkflow.VerifyCompileReadinessWithEnv(ctx, hostTuple, profile, compilerEnv)
}

func buildCommandForProfile(profile targetprofiles.SConsTargetProfile) func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
	return func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
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

		env, envErr := buildEnv(ctx.Workspace, key)
		if envErr != nil {
			return nil, envErr
		}

		cmd := sconsworkflow.BuildCommand(pythonExe, sconsExe, sconsArgs, ctx.Logger)
		cmd.Dir = godotSrc
		cmd.Env = makeEnv(env)
		return cmd, nil
	}
}

func buildEnv(workspace *internal.Workspace, key string) (map[string]string, *internal.Error) {
	hostAdapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	env := hostAdapter.BuildEnv(workspace, key)
	compilerEnv, err := zigCompilerEnv(workspace.Runtime)
	if err != nil {
		return nil, err
	}
	for k, v := range compilerEnv {
		env[k] = v
	}
	return env, nil
}

// zigCompilerEnv resolves the absolute path to the provisioned zig binary and returns
// CC, CXX, and AR overrides that use it directly, so no PATH lookup is needed.
// The path is double-quoted so that workspace paths containing spaces are handled
// correctly by SCons when it parses the compiler command string.
func zigCompilerEnv(runtimeDir string) (map[string]string, *internal.Error) {
	zigExe, err := sconsworkflow.ResolveZigExecutable(runtimeDir)
	if err != nil {
		return nil, &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Provisioned zig compiler not found",
			Details: err.Error(),
		}
	}
	quoted := fmt.Sprintf("%q", zigExe)
	return map[string]string{
		"CC":  quoted + " cc",
		"CXX": quoted + " c++",
		"AR":  quoted + " ar",
	}, nil
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

