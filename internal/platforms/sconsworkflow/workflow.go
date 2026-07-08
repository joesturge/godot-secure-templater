package sconsworkflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/longpath"
	"github.com/joemi/godot-secure-templater/internal/platforms/targetprofiles"
)

const pythonModuleSCons = "python_module_scons"

type RuntimeTools struct {
	PythonExe   string
	SConsExe    string
	GodotSource string
}

type HostAdapter interface {
	NormalizeRuntimeTools(tools *RuntimeTools)
	BuildEnv(workspace *internal.Workspace, key string) map[string]string
}

type windowsHostAdapter struct{}

type posixHostAdapter struct{}

// AdapterForHostTuple returns host-specific runtime/env behavior based on a host tuple.
func AdapterForHostTuple(hostTuple string) HostAdapter {
	normalized := strings.ToLower(strings.TrimSpace(hostTuple))
	if strings.HasPrefix(normalized, "windows/") {
		return windowsHostAdapter{}
	}
	return posixHostAdapter{}
}

// WindowsHostAdapter returns the explicit adapter for Windows-hosted compilation.
func WindowsHostAdapter() HostAdapter {
	return windowsHostAdapter{}
}

// ResolveRuntimeTools locates python, scons and Godot source directories under runtime.
func ResolveRuntimeTools(workspace *internal.Workspace, logger internal.Logger) (*RuntimeTools, *internal.Error) {
	sconsBase := filepath.Join(workspace.Runtime, "scons")
	sconsExe := resolveSConsExecutable(sconsBase, logger)

	pythonExe, err := resolvePythonExecutable(workspace.Runtime)
	if err != nil {
		return nil, &internal.Error{Code: internal.ExitGenericFailure, Message: fmt.Sprintf("Python executable not found at %s", pythonExe), Details: err.Error()}
	}

	godotSrc, err := findGodotSource(filepath.Join(workspace.Runtime, "godot_source"))
	if err != nil {
		return nil, &internal.Error{Code: internal.ExitGenericFailure, Message: "Could not find Godot source directory", Details: err.Error()}
	}

	return &RuntimeTools{
		PythonExe:   pythonExe,
		SConsExe:    sconsExe,
		GodotSource: godotSrc,
	}, nil
}

// BuildCommand constructs the SCons invocation command based on the discovered executable layout.
func BuildCommand(pythonExe string, sconsExe string, sconsArgs []string, logger internal.Logger) *exec.Cmd {
	if sconsExe == pythonModuleSCons {
		logger.Info("    Using python -m SCons (module invocation)")
		return exec.Command(pythonExe, append([]string{"-m", "SCons"}, sconsArgs...)...)
	}
	if strings.HasSuffix(sconsExe, "__main__.py") {
		sconsModuleDir := filepath.Dir(sconsExe)
		sconsRuntimeDir := filepath.Dir(sconsModuleDir)
		pythonCode := fmt.Sprintf(
			"import sys; sys.path.insert(0, %q); exec(open(%q).read())",
			sconsRuntimeDir, sconsExe,
		)
		logger.Info("    Using python -c with sys.path injection")
		return exec.Command(pythonExe, append([]string{"-c", pythonCode}, sconsArgs...)...)
	}

	logger.Info("    Using SCons script directly")
	return exec.Command(pythonExe, append([]string{sconsExe}, sconsArgs...)...)
}

// VerifyCompileReadiness validates that runtime tools are discoverable and that a no-build SCons invocation succeeds.
func VerifyCompileReadiness(ctx *internal.RunContext, hostTuple string, profile targetprofiles.SConsTargetProfile) *internal.Error {
	tools, err := ResolveRuntimeTools(ctx.Workspace, ctx.Logger)
	if err != nil {
		return err
	}

	hostAdapter := AdapterForHostTuple(hostTuple)
	hostAdapter.NormalizeRuntimeTools(tools)
	env := mergedEnv(hostAdapter.BuildEnv(ctx.Workspace, "verify-only"))
	zigExe, zigErr := resolveZigExecutable(ctx.Workspace.Runtime)
	if zigErr != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Compile readiness check failed: zig version",
			Details: zigErr.Error(),
		}
	}

	if err := runProbe("python version", exec.Command(tools.PythonExe, "--version"), env, ""); err != nil {
		return err
	}
	if err := runProbe("zig version", exec.Command(zigExe, "version"), env, ""); err != nil {
		return err
	}

	sconsVersion := BuildCommand(tools.PythonExe, tools.SConsExe, []string{"--version"}, ctx.Logger)
	if err := runProbe("scons version", sconsVersion, env, ""); err != nil {
		return err
	}

	sconsArgs := []string{
		fmt.Sprintf("platform=%s", profile.SConsPlatform),
		"target=template_release",
		"dev_build=no",
		"optimize=speed",
	}
	if len(profile.ExtraSConsArgs) > 0 {
		sconsArgs = append(sconsArgs, profile.ExtraSConsArgs...)
	}
	sconsArgs = append(sconsArgs, "-n")

	dryRun := BuildCommand(tools.PythonExe, tools.SConsExe, sconsArgs, ctx.Logger)
	if err := runProbe("scons dry-run", dryRun, env, tools.GodotSource); err != nil {
		return err
	}

	return nil
}

func (windowsHostAdapter) NormalizeRuntimeTools(tools *RuntimeTools) {
	checker := longpath.NewChecker("windows")

	if checker.NeedsPrefixing(tools.GodotSource) {
		tools.GodotSource = checker.ExtendedLengthPath(tools.GodotSource)
	}
	if checker.NeedsPrefixing(tools.SConsExe) && tools.SConsExe != pythonModuleSCons {
		tools.SConsExe = checker.ExtendedLengthPath(tools.SConsExe)
	}
	if checker.NeedsPrefixing(tools.PythonExe) {
		tools.PythonExe = checker.ExtendedLengthPath(tools.PythonExe)
	}
}

func (windowsHostAdapter) BuildEnv(workspace *internal.Workspace, key string) map[string]string {
	env := map[string]string{}
	paths := []string{
		filepath.Join(workspace.Runtime, "python"),
		filepath.Join(workspace.Runtime, "zig"),
		filepath.Join(workspace.Runtime, "scons"),
	}

	if zigEntries, err := os.ReadDir(filepath.Join(workspace.Runtime, "zig")); err == nil {
		for _, entry := range zigEntries {
			if entry.IsDir() {
				paths = append(paths, filepath.Join(workspace.Runtime, "zig", entry.Name()))
			}
		}
	}

	if systemPath := os.Getenv("PATH"); systemPath != "" {
		paths = append(paths, systemPath)
	}
	env["PATH"] = strings.Join(paths, ";")
	env["PYTHONPATH"] = strings.Join([]string{filepath.Join(workspace.Runtime, "scons")}, ";")
	env["SCRIPT_AES256_ENCRYPTION_KEY"] = key
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		env["SystemRoot"] = systemRoot
	}

	return env
}

func (posixHostAdapter) NormalizeRuntimeTools(tools *RuntimeTools) {
	// POSIX hosts do not need Windows long-path normalization.
}

func (posixHostAdapter) BuildEnv(workspace *internal.Workspace, key string) map[string]string {
	env := map[string]string{}
	paths := []string{
		filepath.Join(workspace.Runtime, "python"),
		filepath.Join(workspace.Runtime, "zig"),
		filepath.Join(workspace.Runtime, "scons"),
	}

	if zigEntries, err := os.ReadDir(filepath.Join(workspace.Runtime, "zig")); err == nil {
		for _, entry := range zigEntries {
			if entry.IsDir() {
				paths = append(paths, filepath.Join(workspace.Runtime, "zig", entry.Name()))
			}
		}
	}

	if systemPath := os.Getenv("PATH"); systemPath != "" {
		paths = append(paths, systemPath)
	}
	separator := string(os.PathListSeparator)
	env["PATH"] = strings.Join(paths, separator)
	env["PYTHONPATH"] = strings.Join([]string{filepath.Join(workspace.Runtime, "scons")}, separator)
	env["SCRIPT_AES256_ENCRYPTION_KEY"] = key

	return env
}

func resolveSConsExecutable(sconsBase string, logger internal.Logger) string {
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
				logger.Debug("Found scons package at %s", candidate)
			}
			break
		}
	}

	if sconsExe == "" {
		sconsExe = pythonModuleSCons
		logger.Debug("Using python -m SCons as fallback (egg-info distribution)")
	}

	return sconsExe
}

func resolvePythonExecutable(runtimeDir string) (string, error) {
	pythonExe := filepath.Join(runtimeDir, "python", "python.exe")
	if _, err := os.Stat(pythonExe); err != nil {
		pythonExe = filepath.Join(runtimeDir, "python", "python")
	}
	if _, err := os.Stat(pythonExe); err == nil {
		return pythonExe, nil
	}

	for _, candidate := range []string{"python3", "python"} {
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}

	_, err := os.Stat(pythonExe)
	return pythonExe, err
}

func resolveZigExecutable(runtimeDir string) (string, error) {
	candidates := []string{
		filepath.Join(runtimeDir, "zig", "zig.exe"),
		filepath.Join(runtimeDir, "zig", "zig"),
	}

	zigBase := filepath.Join(runtimeDir, "zig")
	if entries, err := os.ReadDir(zigBase); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidates = append(candidates,
					filepath.Join(zigBase, entry.Name(), "zig.exe"),
					filepath.Join(zigBase, entry.Name(), "zig"),
				)
			}
		}
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if resolved, err := exec.LookPath("zig"); err == nil {
		return resolved, nil
	}

	return "", fmt.Errorf("zig executable not found under %s", zigBase)
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

func mergedEnv(overrides map[string]string) []string {
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

func runProbe(name string, cmd *exec.Cmd, env []string, dir string) *internal.Error {
	cmd.Env = env
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: fmt.Sprintf("Compile readiness check failed: %s", name),
			Details: fmt.Sprintf("%v\n%s", err, strings.TrimSpace(string(output))),
		}
	}

	return nil
}
