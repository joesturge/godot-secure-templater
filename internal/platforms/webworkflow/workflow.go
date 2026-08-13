package webworkflow

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

const emscriptenSDK = "sdk-releases-3ebc04a3dab24522a5bf8ced3ce3caea816558f6-64bit"

func BuildCommandForProfile(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile, hostTuple string, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
	if err := ensureSDK(ctx, hostTuple); err != nil {
		return nil, err
	}

	tools, err := sconsworkflow.ResolveRuntimeTools(ctx.Workspace, ctx.Logger)
	if err != nil {
		return nil, err
	}
	adapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	adapter.NormalizeRuntimeTools(tools)
	sconsArgs := []string{
		fmt.Sprintf("platform=%s", profile.SConsPlatform),
		fmt.Sprintf("target=%s", target),
		"dev_build=no",
		"optimize=speed",
	}
	sconsArgs = append(sconsArgs, profile.ExtraSConsArgs...)
	cmd := sconsworkflow.BuildCommand(tools.PythonExe, tools.SConsExe, sconsArgs, ctx.Logger)
	cmd.Dir = tools.GodotSource
	env := buildEnv(ctx.Workspace, hostTuple, key)
	cmd.Env = mergedEnv(env)
	return cmd, nil
}

func VerifyCompileReadiness(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile, hostTuple string) *internal.Error {
	if err := ensureSDK(ctx, hostTuple); err != nil {
		return err
	}
	return sconsworkflow.VerifyCompileReadinessWithEnv(ctx, hostTuple, profile, buildEnv(ctx.Workspace, hostTuple, "verify-only"))
}

func ensureSDK(ctx *internal.RunContext, hostTuple string) *internal.Error {
	sdkRoot := resolveSDKRoot(ctx.Workspace.Runtime)
	emsdkPy := filepath.Join(sdkRoot, "emsdk.py")
	configPath := filepath.Join(sdkRoot, ".emscripten")
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	tools, err := sconsworkflow.ResolveRuntimeTools(ctx.Workspace, ctx.Logger)
	if err != nil {
		return err
	}
	adapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	adapter.NormalizeRuntimeTools(tools)
	baseEnv := adapter.BuildEnv(ctx.Workspace, "")
	for _, args := range [][]string{{"install", emscriptenSDK}, {"activate", emscriptenSDK}} {
		cmd := exec.Command(tools.PythonExe, append([]string{emsdkPy}, args...)...)
		cmd.Dir = sdkRoot
		cmd.Env = mergedEnv(baseEnv)
		output, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return &internal.Error{
				Code:    internal.ExitBuildFailed,
				Message: "Emscripten SDK setup failed",
				Details: fmt.Sprintf("%v\n%s", runErr, strings.TrimSpace(string(output))),
			}
		}
	}
	return nil
}

func buildEnv(workspace *internal.Workspace, hostTuple string, key string) map[string]string {
	adapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	env := adapter.BuildEnv(workspace, key)
	sdkRoot := resolveSDKRoot(workspace.Runtime)
	separator := string(os.PathListSeparator)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(hostTuple)), "windows/") {
		separator = ";"
	}
	emscriptenDir := filepath.Join(sdkRoot, "upstream", "emscripten")
	paths := []string{
		emscriptenDir,
		filepath.Join(sdkRoot, "upstream", "bin"),
		filepath.Join(sdkRoot, "node", "20.18.0_64bit", "bin"),
	}
	env["PATH"] = strings.Join(append(paths, env["PATH"]), separator)
	env["EMSDK"] = sdkRoot
	env["EMSDK_ROOT"] = sdkRoot
	env["EM_CONFIG"] = filepath.Join(sdkRoot, ".emscripten")
	// em++.py (and emcc.py) do `import emcc` — the emscripten directory must be on
	// PYTHONPATH so Python can find the sibling modules regardless of how the subprocess
	// is launched.
	existingPythonPath := env["PYTHONPATH"]
	if existingPythonPath != "" {
		env["PYTHONPATH"] = emscriptenDir + separator + existingPythonPath
	} else {
		env["PYTHONPATH"] = emscriptenDir
	}
	return env
}

func resolveSDKRoot(runtimeDir string) string {
	base := filepath.Join(runtimeDir, "emsdk")
	if _, err := os.Stat(filepath.Join(base, "emsdk.py")); err == nil {
		return base
	}
	entries, err := os.ReadDir(base)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidate := filepath.Join(base, entry.Name())
				if _, err := os.Stat(filepath.Join(candidate, "emsdk.py")); err == nil {
					return candidate
				}
			}
		}
	}
	return base
}

func mergedEnv(overrides map[string]string) []string {
	return internal.SanitizedEnv(overrides)
}
