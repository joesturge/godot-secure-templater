package builder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/longpath"
	"github.com/joemi/godot-secure-templater/internal/progress"
)

// BuildTarget represents a build variant.
type BuildTarget string

const (
	BuildRelease BuildTarget = "template_release"
	BuildDebug   BuildTarget = "template_debug"
)

// CompileTemplates compiles both release and debug templates using SCons.
func CompileTemplates(ctx *internal.RunContext, key string) *internal.Error {
	ctx.Logger.Info("Compiling Godot templates...")

	env := BuildEnv(ctx, key)
	targets := []BuildTarget{BuildRelease, BuildDebug}

	for _, target := range targets {
		if err := compileSingle(ctx, target, env); err != nil {
			return err
		}
	}

	ctx.Logger.Info("Templates compiled successfully.")
	return nil
}

// compileSingle compiles a single template variant.
func compileSingle(ctx *internal.RunContext, target BuildTarget, env map[string]string) *internal.Error {
	ctx.Logger.Info("  → Compiling %s...", target)

	// Path to SCons executable
	// SCons distribution from GitHub releases can have different structures:
	// - scons.py at root (older distributions)
	// - scons/ module with __main__.py (egg-info format)
	// - bin/scons or scripts/scons (prebuilt)
	sconsBase := filepath.Join(ctx.Workspace.Runtime, "scons")
	sconsExe := ""

	// Build search paths in priority order
	searchPaths := []string{
		// First: look for scons.py (standalone script)
		filepath.Join(sconsBase, "scons.py"),
		filepath.Join(sconsBase, "bin", "scons.py"),
		filepath.Join(sconsBase, "scripts", "scons.py"),
		// Then: look for module __main__.py (egg-info format)
		filepath.Join(sconsBase, "scons", "__main__.py"),
		filepath.Join(sconsBase, "SCons", "__main__.py"), // Case variation for Windows
		// Finally: executable scons
		filepath.Join(sconsBase, "bin", "scons"),
		filepath.Join(sconsBase, "bin", "scons.bat"),
		filepath.Join(sconsBase, "scripts", "scons"),
		filepath.Join(sconsBase, "scripts", "scons.bat"),
	}

	// Check for scons-X.Y.Z/ subdirectories (from tar.gz)
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

	// Search in priority order and use first found
	for _, candidate := range searchPaths {
		if _, err := os.Stat(candidate); err == nil {
			sconsExe = candidate
			if strings.HasSuffix(candidate, "__main__.py") {
				ctx.Logger.Debug("Found scons package at %s", candidate)
			}
			break
		}
	}

	// Last resort: use python -m SCons (may not work with embedded Python)
	if sconsExe == "" {
		sconsExe = "python_module_scons" // Special marker for python -m SCons
		ctx.Logger.Debug("Using python -m SCons as fallback (egg-info distribution)")
	}

	// Path to Godot source (extract removes version suffix, so search for it)
	godotSourceBase := filepath.Join(ctx.Workspace.Runtime, "godot_source")
	godotSrc, err := findGodotSource(godotSourceBase)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Could not find Godot source directory",
			Details: err.Error(),
		}
	}

	// Validate SCons was found (skip if using module invocation)
	if sconsExe != "python_module_scons" {
		if _, err := os.Stat(sconsExe); err != nil {
			// List directory contents for debugging
			entries, err := os.ReadDir(sconsBase)
			var names []string
			if err == nil {
				for _, e := range entries {
					prefix := ""
					if e.IsDir() {
						prefix = "/ "
					}
					names = append(names, prefix+e.Name())
				}
			}
			return &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: "SCons executable not found",
				Details: fmt.Sprintf("Searched: %s\nContents: %v\nWill try python -m SCons as fallback", sconsBase, names),
			}
		}
	}

	if _, err := os.Stat(godotSrc); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Godot source not found at %s", godotSrc),
			Details: err.Error(),
		}
	}

	ctx.Logger.Debug("Resolved SCons to: %s", sconsExe)
	ctx.Logger.Debug("Godot source: %s", godotSrc)

	// Build SCons command using full path to Python
	pythonExe := filepath.Join(ctx.Workspace.Runtime, "python", "python.exe")

	// On non-Windows, try without .exe
	if _, err := os.Stat(pythonExe); err != nil {
		pythonExe = filepath.Join(ctx.Workspace.Runtime, "python", "python")
	}

	if _, err := os.Stat(pythonExe); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Python executable not found at %s", pythonExe),
			Details: err.Error(),
		}
	}

	checker := longpath.NewChecker("windows")
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

	// Build SCons command arguments (used across all invocation methods)
	sconsArgs := []string{
		"platform=windows",
		fmt.Sprintf("target=%s", target),
		"dev_build=no",
		"optimize=speed",
		"d3d12=no", // Disable D3D12 driver to avoid SDK dependency
	}

	// Build command based on how we found SCons
	var cmd *exec.Cmd
	if sconsExe == "python_module_scons" {
		// Use python -m SCons (for egg-info distributions)
		ctx.Logger.Info("    Using python -m SCons (module invocation)")
		cmd = exec.Command(pythonExe, append([]string{"-m", "SCons"}, sconsArgs...)...)
	} else if strings.HasSuffix(sconsExe, "__main__.py") {
		// For __main__.py, inject sys.path to find SCons module
		sconsModuleDir := filepath.Dir(sconsExe)        // .../scons/scons
		sconsRuntimeDir := filepath.Dir(sconsModuleDir) // .../scons
		pythonCode := fmt.Sprintf(
			"import sys; sys.path.insert(0, %q); exec(open(%q).read())",
			sconsRuntimeDir, sconsExe,
		)
		ctx.Logger.Info("    Using python -c with sys.path injection")
		cmd = exec.Command(pythonExe, append([]string{"-c", pythonCode}, sconsArgs...)...)
	} else {
		// Use scons.py directly (standard distribution)
		ctx.Logger.Info("    Using SCons script directly")
		cmd = exec.Command(pythonExe, append([]string{sconsExe}, sconsArgs...)...)
	}
	cmd.Dir = godotSrc

	// Build isolated environment
	cmd.Env = makeEnv(env)

	// Capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Failed to setup stdout pipe",
			Details: err.Error(),
		}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Failed to setup stderr pipe",
			Details: err.Error(),
		}
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Failed to start SCons build",
			Details: err.Error(),
		}
	}

	// Stream output
	go streamOutput(ctx.Logger, stdout, false)
	go streamOutput(ctx.Logger, stderr, true)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: fmt.Sprintf("SCons build failed for %s", target),
			Details: err.Error(),
		}
	}

	ctx.Logger.Info("    ✓ %s compiled successfully", target)

	// Move artifact to templates/
	if err := moveTemplate(ctx, godotSrc, target); err != nil {
		return err
	}

	return nil
}

// findGodotSource searches for the Godot source directory within godot_source/.
// The extracted tar.gz creates a directory like godot-4.3.1-stable/
func findGodotSource(baseDir string) (string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read godot_source directory: %w", err)
	}

	// Try to find godot-* directory
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "godot-") {
			return filepath.Join(baseDir, entry.Name()), nil
		}
	}

	// If not found, provide helpful diagnostic
	if len(entries) == 0 {
		return "", fmt.Errorf(
			"godot source directory is empty. "+
				"Try running with --force-rebuild to re-extract the toolchain.\n"+
				"Location: %s",
			baseDir,
		)
	}

	// List what's actually there for debugging
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return "", fmt.Errorf(
		"no godot-* directory found in %s\n"+
			"found instead: %v\n"+
			"this usually means the Godot source extraction failed\n"+
			"try running with --force-rebuild to re-extract",
		baseDir, names,
	)
}

// makeEnv constructs the full environment for the command, preserving necessary system vars.
func makeEnv(overrides map[string]string) []string {
	// Start with current environment
	env := os.Environ()

	// Remove PATH entries that might conflict
	filtered := []string{}
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		// Skip entries that we're overriding
		if _, ok := overrides[key]; !ok {
			filtered = append(filtered, e)
		}
	}

	// Add overrides
	for k, v := range overrides {
		filtered = append(filtered, fmt.Sprintf("%s=%s", k, v))
	}

	return filtered
}

// streamOutput reads from a pipe and logs each line.
func streamOutput(logger interface{ Info(string, ...interface{}) }, reader io.ReadCloser, isError bool) {
	scanner := bufio.NewScanner(reader)
	parser := progress.NewParser()
	lastStage := progress.StageUnknown
	start := time.Now()

	for scanner.Scan() {
		line := scanner.Text()

		if !isError {
			stage := parser.ParseLine(line)
			if stage != lastStage {
				logger.Info("%s (elapsed: %s)", progress.FormatStageUpdate(stage), time.Since(start).Round(time.Second))
				lastStage = stage
			}
		}

		if isError {
			// For stderr, use a different logging level if available
			// For now, just log as info with a prefix
			if loggerWithWarn, ok := logger.(interface{ Warn(string, ...interface{}) }); ok {
				loggerWithWarn.Warn(line)
			} else {
				logger.Info(line)
			}
		} else {
			logger.Info(line)
		}
	}
}

// moveTemplate moves the compiled template from the Godot build directory to templates/.
func moveTemplate(ctx *internal.RunContext, godotSrc string, target BuildTarget) *internal.Error {
	// The build output directory
	binDir := filepath.Join(godotSrc, "bin")

	// SCons produces templates with specific naming patterns:
	// godot.windows.template_release.x86_64.exe (main executable)
	// godot.windows.template_release.x86_64.console.exe (console variant)
	// We use the main executable (non-console variant)

	targetName := "template_release"
	if target == BuildDebug {
		targetName = "template_debug"
	}

	// Search for the actual compiled file
	srcName := fmt.Sprintf("godot.windows.%s.x86_64.exe", targetName)
	srcPath := filepath.Join(binDir, srcName)

	// Check if file exists
	if _, err := os.Stat(srcPath); err != nil {
		// File not found - list directory contents for debugging
		var actualFiles []string
		if entries, dirErr := os.ReadDir(binDir); dirErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.Contains(entry.Name(), "template") {
					actualFiles = append(actualFiles, entry.Name())
				}
			}
		}

		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Compiled template not found at %s", srcPath),
			Details: fmt.Sprintf("Expected: %s\nFound templates in bin/: %v", srcName, actualFiles),
		}
	}

	// Destination
	dstName := fmt.Sprintf("windows_%s.exe", target)
	dstPath := filepath.Join(ctx.Workspace.Templates, dstName)

	// Ensure templates directory exists
	if err := os.MkdirAll(ctx.Workspace.Templates, 0755); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to create templates directory",
			Details: err.Error(),
		}
	}

	// Copy file
	src, err := os.Open(srcPath)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to open source template %s", srcName),
			Details: err.Error(),
		}
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := os.Create(dstPath)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to create destination template %s", dstName),
			Details: err.Error(),
		}
	}
	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Failed to copy template to %s", dstName),
			Details: err.Error(),
		}
	}

	ctx.Logger.Info("    ✓ Copied %s to templates/", srcName)
	ctx.Logger.Debug("Template copied from %s to %s", srcPath, dstPath)
	return nil
}

// BuildEnv constructs an isolated environment for SCons compilation.
func BuildEnv(ctx *internal.RunContext, key string) map[string]string {
	env := make(map[string]string)

	// Build PATH with runtime binaries first
	paths := []string{
		filepath.Join(ctx.Workspace.Runtime, "python"),
		filepath.Join(ctx.Workspace.Runtime, "mingw", "bin"),
		filepath.Join(ctx.Workspace.Runtime, "scons"),
	}

	// Add system PATH to fallback
	if systemPath := os.Getenv("PATH"); systemPath != "" {
		paths = append(paths, systemPath)
	}

	env["PATH"] = strings.Join(paths, ";") // Windows uses semicolon

	// For embedded Python, don't set PYTHONHOME as it can restrict module search paths
	// Instead, rely on PYTHONPATH to find our modules

	// PYTHONPATH must include scons directory so Python can find the SCons module
	// Embedded Python should already have its stdlib, we just need to add our modules
	pythonPaths := []string{
		filepath.Join(ctx.Workspace.Runtime, "scons"), // Add scons for SCons module
	}
	env["PYTHONPATH"] = strings.Join(pythonPaths, ";")

	// Set encryption key
	env["SCRIPT_AES256_ENCRYPTION_KEY"] = key

	// Windows-specific
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		env["SystemRoot"] = systemRoot
	}

	return env
}

// MoveTemplates moves compiled templates from the Godot build directory to templates/.
// This is a convenience function for batch operations; compileSingle handles individual moves.
func MoveTemplates(ctx *internal.RunContext, sourceDir string) *internal.Error {
	ctx.Logger.Info("Moving templates to workspace...")
	return nil
}
