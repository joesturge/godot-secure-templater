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
		if err := ensureWindowsMingwPrefix(ctx.Workspace.Runtime); err != nil {
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

func verifyCompileReadiness(ctx *internal.RunContext, profile targetprofiles.SConsTargetProfile) *internal.Error {
	if err := ensureWindowsMingwPrefix(ctx.Workspace.Runtime); err != nil {
		return err
	}

	return sconsworkflow.VerifyCompileReadinessWithEnv(ctx, hostTuple, profile, buildEnv(ctx.Workspace, "verify-only"))
}

func buildEnv(workspace *internal.Workspace, key string) map[string]string {
	hostAdapter := sconsworkflow.AdapterForHostTuple(hostTuple)
	env := hostAdapter.BuildEnv(workspace, key)
	mingwPrefix := mingwPrefixForEnv(workspace.Runtime)
	mingwBin := filepath.Join(mingwPrefix, "bin")
	env["PATH"] = prependWindowsPath(mingwBin, env["PATH"])
	env["MINGW_PREFIX"] = mingwPrefix
	env["CC"] = "gcc"
	env["CXX"] = "g++"
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

func ensureWindowsMingwPrefix(runtimeDir string) *internal.Error {
	if _, err := resolveBundledMingwPrefix(runtimeDir); err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Compile readiness check failed: MinGW setup",
			Details: err.Error(),
		}
	}
	return nil
}

func mingwPrefixForEnv(runtimeDir string) string {
	prefix, err := resolveBundledMingwPrefix(runtimeDir)
	if err != nil {
		return filepath.Join(runtimeDir, "mingw")
	}
	return prefix
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

		children, childErr := os.ReadDir(candidate)
		if childErr != nil {
			continue
		}
		for _, child := range children {
			if !child.IsDir() {
				continue
			}
			grandchild := filepath.Join(candidate, child.Name())
			if hasBinDir(grandchild) {
				return grandchild, nil
			}
		}
	}

	return "", fmt.Errorf("no MinGW toolchain with a bin directory found under %s", base)
}

func hasBinDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "bin"))
	return err == nil && info.IsDir()
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
