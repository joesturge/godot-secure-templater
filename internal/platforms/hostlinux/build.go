package hostlinux

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
	"github.com/joemi/godot-secure-templater/internal/platforms/webworkflow"
)

func verifyCompileReadiness(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile) *internal.Error {
	if profile.TargetTuple == "web/wasm32" {
		return webworkflow.VerifyCompileReadiness(ctx, profile, hostTuple)
	}
	compilerEnv, err := zigCompilerEnvForTarget(ctx.Workspace.Runtime, profile.TargetTuple)
	if err != nil {
		return err
	}
	return sconsworkflow.VerifyCompileReadinessWithEnv(ctx, hostTuple, profile, compilerEnv)
}

func buildCommandForProfile(profile targetprofiles.SConsTargetProfile) func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
	return func(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
		if profile.TargetTuple == "web/wasm32" {
			return webworkflow.BuildCommandForProfile(ctx, profile, hostTuple, target, key)
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

		env, envErr := buildEnvForProfile(ctx.Workspace, key, profile.TargetTuple)
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
	return buildEnvForProfile(workspace, key, "linux/amd64")
}

func buildEnvForProfile(workspace *internal.Workspace, key string, targetTuple string) (map[string]string, *internal.Error) {
	hostAdapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	env := hostAdapter.BuildEnv(workspace, key)
	if strings.EqualFold(strings.TrimSpace(targetTuple), "windows/amd64") {
		mingwPrefix, err := resolveBundledMingwPrefix(workspace.Runtime)
		if err != nil {
			return nil, &internal.Error{
				Code:    internal.ExitBuildFailed,
				Message: "Provisioned MinGW compiler not found",
				Details: err.Error(),
			}
		}
		mingwBin := filepath.Join(mingwPrefix, "bin")
		env["MINGW_PREFIX"] = mingwPrefix
		env["PATH"] = prependPosixPath(mingwBin, env["PATH"])
		env["CC"] = "x86_64-w64-mingw32-clang"
		env["CXX"] = "x86_64-w64-mingw32-clang++"
		env["AR"] = "x86_64-w64-mingw32-ar"
		return env, nil
	}

	compilerEnv, err := zigCompilerEnvForTarget(workspace.Runtime, targetTuple)
	if err != nil {
		return nil, err
	}
	for k, v := range compilerEnv {
		env[k] = v
	}
	return env, nil
}

func prependPosixPath(entry string, existing string) string {
	if existing == "" {
		return entry
	}
	if strings.HasPrefix(existing, entry+string(os.PathListSeparator)) || existing == entry {
		return existing
	}
	return entry + string(os.PathListSeparator) + existing
}

func resolveBundledMingwPrefix(runtimeDir string) (string, error) {
	base := filepath.Join(runtimeDir, "mingw")
	if hasBinDir(base) {
		return base, nil
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("mingw directory not found under %s: %w", base, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(base, entry.Name())
		if hasBinDir(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no MinGW toolchain with a bin directory found under %s", base)
}

func hasBinDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "bin"))
	return err == nil && info.IsDir()
}

func zigCompilerEnvForTarget(runtimeDir string, targetTuple string) (map[string]string, *internal.Error) {
	zigExe, err := sconsworkflow.ResolveZigExecutable(runtimeDir)
	if err != nil {
		return nil, &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Provisioned zig compiler not found",
			Details: err.Error(),
		}
	}
	quoted := fmt.Sprintf("%q", zigExe)
	compilerPrefix := quoted + " cc"
	compilerCXXPrefix := quoted + " c++"
	if strings.EqualFold(strings.TrimSpace(targetTuple), "windows/amd64") {
		compilerPrefix += " -target x86_64-windows-gnu"
		compilerCXXPrefix += " -target x86_64-windows-gnu"
	}
	return map[string]string{
		"CC":  compilerPrefix,
		"CXX": compilerCXXPrefix,
		"AR":  quoted + " ar",
	}, nil
}

// zigCompilerEnv resolves the absolute path to the provisioned zig binary and returns
// CC, CXX, and AR overrides that use it directly, so no PATH lookup is needed.
// The path is double-quoted so that workspace paths containing spaces are handled
// correctly by SCons when it parses the compiler command string.
func zigCompilerEnv(runtimeDir string) (map[string]string, *internal.Error) {
	return zigCompilerEnvForTarget(runtimeDir, "linux/amd64")
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
