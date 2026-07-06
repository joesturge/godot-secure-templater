package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/longpath"
)

func sourceTemplateName(target builder.BuildTarget) string {
	if target == builder.BuildDebug {
		return "godot.windows.template_debug.x86_64.exe"
	}
	return "godot.windows.template_release.x86_64.exe"
}

func destinationTemplateName(target builder.BuildTarget) string {
	return fmt.Sprintf("windows_%s.exe", target)
}

func buildCommand(ctx *internal.RunContext, target builder.BuildTarget, key string) (*exec.Cmd, *internal.Error) {
	sconsBase := filepath.Join(ctx.Workspace.Runtime, "scons")
	sconsExe := ""

	searchPaths := []string{
		filepath.Join(sconsBase, "scons.py"),
		filepath.Join(sconsBase, "bin", "scons.py"),
		filepath.Join(sconsBase, "scripts", "scons.py"),
		filepath.Join(sconsBase, "scons", "__main__.py"),
		filepath.Join(sconsBase, "SCons", "__main__.py"),
		filepath.Join(sconsBase, "bin", "scons"),
		filepath.Join(sconsBase, "bin", "scons.bat"),
		filepath.Join(sconsBase, "scripts", "scons"),
		filepath.Join(sconsBase, "scripts", "scons.bat"),
	}

	if entries, err := os.ReadDir(sconsBase); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "scons-") {
				searchPaths = append(searchPaths,
					filepath.Join(sconsBase, entry.Name(), "scons.py"),
					filepath.Join(sconsBase, entry.Name(), "bin", "scons.py"),
				)
			}
		}
	}

	for _, candidate := range searchPaths {
		if _, err := os.Stat(candidate); err == nil {
			sconsExe = candidate
			if strings.HasSuffix(candidate, "__main__.py") {
				ctx.Logger.Debug("Found scons package at %s", candidate)
			}
			break
		}
	}

	if sconsExe == "" {
		sconsExe = "python_module_scons"
		ctx.Logger.Debug("Using python -m SCons as fallback (egg-info distribution)")
	}

	pythonExe := filepath.Join(ctx.Workspace.Runtime, "python", "python.exe")
	if _, err := os.Stat(pythonExe); err != nil {
		pythonExe = filepath.Join(ctx.Workspace.Runtime, "python", "python")
	}
	if _, err := os.Stat(pythonExe); err != nil {
		return nil, &internal.Error{Code: internal.ExitGenericFailure, Message: fmt.Sprintf("Python executable not found at %s", pythonExe), Details: err.Error()}
	}

	checker := longpath.NewChecker("windows")
	godotSrc, err := findGodotSource(filepath.Join(ctx.Workspace.Runtime, "godot_source"))
	if err != nil {
		return nil, &internal.Error{Code: internal.ExitGenericFailure, Message: "Could not find Godot source directory", Details: err.Error()}
	}

	if checker.NeedsPrefixing(godotSrc) {
		godotSrc = checker.ExtendedLengthPath(godotSrc)
	}
	if checker.NeedsPrefixing(sconsExe) && sconsExe != "python_module_scons" {
		sconsExe = checker.ExtendedLengthPath(sconsExe)
	}
	if checker.NeedsPrefixing(pythonExe) {
		pythonExe = checker.ExtendedLengthPath(pythonExe)
	}

	ctx.Logger.Debug("Using Python: %s", pythonExe)
	ctx.Logger.Debug("Using SCons: %s", sconsExe)
	ctx.Logger.Debug("Godot source: %s", godotSrc)

	sconsArgs := []string{
		"platform=windows",
		fmt.Sprintf("target=%s", target),
		"dev_build=no",
		"optimize=speed",
		"d3d12=no",
	}

	var cmd *exec.Cmd
	if sconsExe == "python_module_scons" {
		ctx.Logger.Info("    Using python -m SCons (module invocation)")
		cmd = exec.Command(pythonExe, append([]string{"-m", "SCons"}, sconsArgs...)...)
	} else if strings.HasSuffix(sconsExe, "__main__.py") {
		sconsModuleDir := filepath.Dir(sconsExe)
		sconsRuntimeDir := filepath.Dir(sconsModuleDir)
		pythonCode := fmt.Sprintf(
			"import sys; sys.path.insert(0, %q); exec(open(%q).read())",
			sconsRuntimeDir, sconsExe,
		)
		ctx.Logger.Info("    Using python -c with sys.path injection")
		cmd = exec.Command(pythonExe, append([]string{"-c", pythonCode}, sconsArgs...)...)
	} else {
		ctx.Logger.Info("    Using SCons script directly")
		cmd = exec.Command(pythonExe, append([]string{sconsExe}, sconsArgs...)...)
	}

	cmd.Dir = godotSrc
	cmd.Env = makeEnv(buildEnv(ctx, key))
	return cmd, nil
}

func buildEnv(ctx *internal.RunContext, key string) map[string]string {
	env := map[string]string{}
	paths := []string{
		filepath.Join(ctx.Workspace.Runtime, "python"),
		filepath.Join(ctx.Workspace.Runtime, "mingw", "bin"),
		filepath.Join(ctx.Workspace.Runtime, "scons"),
	}
	if systemPath := os.Getenv("PATH"); systemPath != "" {
		paths = append(paths, systemPath)
	}
	env["PATH"] = strings.Join(paths, ";")
	env["PYTHONPATH"] = strings.Join([]string{filepath.Join(ctx.Workspace.Runtime, "scons")}, ";")
	env["SCRIPT_AES256_ENCRYPTION_KEY"] = key
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		env["SystemRoot"] = systemRoot
	}
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

func findGodotSource(baseDir string) (string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read godot_source directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "godot-") {
			return filepath.Join(baseDir, entry.Name()), nil
		}
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("godot source directory is empty. try running with --force-rebuild to re-extract the toolchain.\nLocation: %s", baseDir)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return "", fmt.Errorf("no godot-* directory found in %s\nfound instead: %v\nthis usually means the Godot source extraction failed\ntry running with --force-rebuild to re-extract", baseDir, names)
}
