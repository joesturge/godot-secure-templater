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
			"import os, sys; env_path = os.environ.get('PYTHONPATH', ''); entries = [entry for entry in env_path.split(os.pathsep) if entry];\nfor entry in entries:\n    if entry not in sys.path:\n        sys.path.insert(0, entry)\nif %q not in sys.path:\n    sys.path.insert(0, %q)\nif %q not in entries:\n    entries.insert(0, %q)\nos.environ['PYTHONPATH'] = os.pathsep.join(entries)\nexec(open(%q).read())",
			sconsRuntimeDir, sconsRuntimeDir, sconsRuntimeDir, sconsRuntimeDir, sconsExe,
		)
		logger.Info("    Using python -c with sys.path injection")
		return exec.Command(pythonExe, append([]string{"-c", pythonCode}, sconsArgs...)...)
	}

	logger.Info("    Using SCons script directly")
	return exec.Command(pythonExe, append([]string{sconsExe}, sconsArgs...)...)
}

// VerifyCompileReadinessWithEnv validates readiness using optional env overrides.
func VerifyCompileReadinessWithEnv(ctx *internal.RunContext, hostTuple string, profile targetprofiles.SConsTargetProfile, extraEnv map[string]string) *internal.Error {
	tools, err := ResolveRuntimeTools(ctx.Workspace, ctx.Logger)
	if err != nil {
		return err
	}

	hostAdapter := AdapterForHostTuple(hostTuple)
	hostAdapter.NormalizeRuntimeTools(tools)
	envOverrides := hostAdapter.BuildEnv(ctx.Workspace, "verify-only")
	for k, v := range extraEnv {
		envOverrides[k] = v
	}
	env := mergedEnv(envOverrides)
	ctx.Logger.Debug("Verify-only host tuple: %s", hostTuple)
	ctx.Logger.Debug("Verify-only compiler env: CC=%q CXX=%q AR=%q MINGW_PREFIX=%q", envOverrides["CC"], envOverrides["CXX"], envOverrides["AR"], envOverrides["MINGW_PREFIX"])
	if pathValue, ok := envOverrides["PATH"]; ok {
		ctx.Logger.Debug("Verify-only PATH head: %s", pathHead(pathValue, hostTuple))
	}
	if err := runProbe("python version", exec.Command(tools.PythonExe, "--version"), env, ""); err != nil {
		return err
	}
	if usesZigCompiler(envOverrides) {
		zigExe, zigErr := resolveZigExecutable(ctx.Workspace.Runtime)
		if zigErr != nil {
			return &internal.Error{
				Code:    internal.ExitBuildFailed,
				Message: "Compile readiness check failed: zig version",
				Details: zigErr.Error(),
			}
		}
		if err := runProbe("zig version", exec.Command(zigExe, "version"), env, ""); err != nil {
			return err
		}
	} else {
		pathSep := string(os.PathListSeparator)
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(hostTuple)), "windows/") {
			pathSep = ";"
		}
		if ccExe := envOverrides["CC"]; ccExe != "" {
			resolvedCC, resolveErr := resolveExeFromEnvPath(ccExe, envOverrides["PATH"], pathSep)
			if resolveErr != nil {
				return &internal.Error{
					Code:    internal.ExitBuildFailed,
					Message: "Compile readiness check failed: cc not found",
					Details: resolveErr.Error(),
				}
			}
			if err := runProbe("cc version", exec.Command(resolvedCC, "--version"), env, ""); err != nil {
				return err
			}
		}
		if arExe := envOverrides["AR"]; arExe != "" {
			resolvedAR, resolveErr := resolveExeFromEnvPath(arExe, envOverrides["PATH"], pathSep)
			if resolveErr != nil {
				return &internal.Error{
					Code:    internal.ExitBuildFailed,
					Message: "Compile readiness check failed: ar not found",
					Details: resolveErr.Error(),
				}
			}
			if err := runProbe("ar version", exec.Command(resolvedAR, "--version"), env, ""); err != nil {
				return err
			}
		}
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
	ctx.Logger.Debug("Verify-only SCons args: %s", strings.Join(sconsArgs, " "))

	dryRun := BuildCommand(tools.PythonExe, tools.SConsExe, sconsArgs, ctx.Logger)
	ctx.Logger.Debug("Verify-only SCons command: %s %s", dryRun.Path, strings.Join(dryRun.Args[1:], " "))
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
		filepath.Join(workspace.Runtime, "bin"),
		filepath.Join(workspace.Runtime, "python", "bin"),
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

// ResolveZigExecutable locates the zig binary under the runtime directory.
func ResolveZigExecutable(runtimeDir string) (string, error) {
	return resolveZigExecutable(runtimeDir)
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
	candidates := []string{
		filepath.Join(runtimeDir, "python", "python.exe"),
		filepath.Join(runtimeDir, "python", "python"),
		filepath.Join(runtimeDir, "python", "bin", "python"),
		filepath.Join(runtimeDir, "python", "bin", "python3"),
		// python-build-standalone archives extract to a nested python/ subdirectory,
		// e.g. runtime/python/python/bin/python3
		filepath.Join(runtimeDir, "python", "python", "bin", "python3"),
		filepath.Join(runtimeDir, "python", "python", "bin", "python"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	// Fallback: glob for versioned python3.* binaries (e.g. python3.11) in known bin
	// directories. python-build-standalone archives ship symlinks (python → python3.11)
	// that are intentionally not materialised during extraction, so the versioned binary
	// may be the only executable present.
	for _, binDir := range []string{
		filepath.Join(runtimeDir, "python", "bin"),
		filepath.Join(runtimeDir, "python", "python", "bin"),
	} {
		matches, _ := filepath.Glob(filepath.Join(binDir, "python3*"))
		for _, match := range matches {
			info, err := os.Stat(match)
			if err == nil && !info.IsDir() {
				return match, nil
			}
		}
	}
	return filepath.Join(runtimeDir, "python", "python"), fmt.Errorf("runtime python executable not found under %s", filepath.Join(runtimeDir, "python"))
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
	return internal.SanitizedEnv(overrides)
}

func pathHead(pathValue string, hostTuple string) string {
	separator := string(os.PathListSeparator)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(hostTuple)), "windows/") {
		separator = ";"
	}
	parts := strings.Split(pathValue, separator)
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, separator)
}

func resolveExeFromEnvPath(name string, envPath string, separator string) (string, error) {
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("compiler executable not found: %s", name)
	}
	exeNames := []string{name, name + ".exe"}
	for _, dir := range strings.Split(envPath, separator) {
		if dir == "" {
			continue
		}
		for _, exeName := range exeNames {
			candidate := filepath.Join(dir, exeName)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%q not found in PATH", name)
}

func usesZigCompiler(env map[string]string) bool {
	for _, key := range []string{"CC", "CXX", "AR"} {
		if strings.Contains(strings.ToLower(env[key]), "zig") {
			return true
		}
	}
	return false
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
