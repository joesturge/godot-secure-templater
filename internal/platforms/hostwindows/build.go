package hostwindows

import (
	"fmt"
	"io"
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
		if err := ensureWindowsZigShims(ctx.Workspace.Runtime); err != nil {
			return nil, err
		}

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
	if err := ensureWindowsZigShims(ctx.Workspace.Runtime); err != nil {
		return err
	}

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
	env["CC"] = "zig cc"
	env["CXX"] = "zig c++"
	env["AR"] = "zig ar"

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

func ensureWindowsZigShims(runtimeDir string) *internal.Error {
	shimRoot := filepath.Join(runtimeDir, "zig-shims")
	shimBin := filepath.Join(shimRoot, "bin")
	zigExe, err := resolveBundledZigExecutable(runtimeDir)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Compile readiness check failed: zig shim setup",
			Details: err.Error(),
		}
	}
	if err := os.MkdirAll(shimBin, 0o755); err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Compile readiness check failed: zig shim setup",
			Details: fmt.Sprintf("failed to create shim dir %s: %v", shimBin, err),
		}
	}

	launcherExe := filepath.Join(shimBin, "zig-shim-launcher.exe")
	launcherSrc := filepath.Join(shimRoot, "zig-shim-launcher.c")
	if err := os.WriteFile(launcherSrc, []byte(windowsShimLauncherSource(zigExe)), 0o644); err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Compile readiness check failed: zig shim setup",
			Details: fmt.Sprintf("failed to write shim launcher source %s: %v", launcherSrc, err),
		}
	}

	compileLauncher := exec.Command(zigExe, "cc", "-target", "x86_64-windows-gnu", "-municode", launcherSrc, "-o", launcherExe)
	if output, err := compileLauncher.CombinedOutput(); err != nil {
		return &internal.Error{
			Code:    internal.ExitBuildFailed,
			Message: "Compile readiness check failed: zig shim setup",
			Details: fmt.Sprintf("failed to build shim launcher with zig: %v\nOutput: %s", err, string(output)),
		}
	}

	shims := map[string]string{
		"clang":   "cc",
		"clang++": "c++",
		"gcc":     "cc",
		"g++":     "c++",
		"ar":      "ar",
		"ranlib":  "ranlib",
		"objcopy": "objcopy",
		"strip":   "strip",
		"dlltool": "dlltool",
		"windres": "rc",
	}

	for _, prefix := range []string{"", "x86_64-w64-mingw32-"} {
		for name := range shims {
			filePath := filepath.Join(shimBin, prefix+name+".exe")
			if err := copyFile(launcherExe, filePath); err != nil {
				return &internal.Error{
					Code:    internal.ExitBuildFailed,
					Message: "Compile readiness check failed: zig shim setup",
					Details: fmt.Sprintf("failed to write shim %s: %v", filePath, err),
				}
			}
			_ = subcommand
		}
	}

	return nil
}

func windowsShimLauncherSource(zigExe string) string {
	return fmt.Sprintf(`
#define WIN32_LEAN_AND_MEAN
#include <process.h>
#include <stdlib.h>
#include <wchar.h>

static const wchar_t ZIG_EXE[] = L"%s";

static const wchar_t *tool_for_name(const wchar_t *name) {
	const wchar_t *base = name;
	const wchar_t *slash = wcsrchr(name, L'\\');
	if (slash != NULL && slash[1] != L'\0') {
		base = slash + 1;
	}
	slash = wcsrchr(base, L'/');
	if (slash != NULL && slash[1] != L'\0') {
		base = slash + 1;
	}

	size_t len = wcslen(base);
	if (len > 4 && _wcsicmp(base + len - 4, L".exe") == 0) {
		len -= 4;
	}
	if (len > 19 && _wcsnicmp(base, L"x86_64-w64-mingw32-", 19) == 0) {
		base += 19;
		len -= 19;
	}

	if (_wcsicmp(base, L"clang") == 0 || _wcsicmp(base, L"gcc") == 0) {
		return L"cc";
	}
	if (_wcsicmp(base, L"clang++") == 0 || _wcsicmp(base, L"g++") == 0) {
		return L"c++";
	}
	if (_wcsicmp(base, L"ar") == 0) {
		return L"ar";
	}
	if (_wcsicmp(base, L"ranlib") == 0) {
		return L"ranlib";
	}
	if (_wcsicmp(base, L"objcopy") == 0) {
		return L"objcopy";
	}
	if (_wcsicmp(base, L"strip") == 0) {
		return L"strip";
	}
	if (_wcsicmp(base, L"dlltool") == 0) {
		return L"dlltool";
	}
	if (_wcsicmp(base, L"windres") == 0) {
		return L"rc";
	}
	return L"cc";
}

int wmain(int argc, wchar_t **argv) {
	const wchar_t *tool = tool_for_name(argv[0]);
	wchar_t **child_argv = calloc((size_t)argc + 2, sizeof(*child_argv));
	if (child_argv == NULL) {
		return 1;
	}

	child_argv[0] = (wchar_t *)ZIG_EXE;
	child_argv[1] = (wchar_t *)tool;
	for (int i = 1; i < argc; i++) {
		child_argv[i + 1] = argv[i];
	}
	child_argv[argc + 1] = NULL;

	int code = _wspawnv(_P_WAIT, ZIG_EXE, (const wchar_t * const *)child_argv);
	free(child_argv);
	if (code == -1) {
		return 1;
	}
	return code;
}
`, windowsWideStringLiteral(zigExe))
}

func windowsWideStringLiteral(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			builder.WriteString(`\\`)
		case '"':
			builder.WriteString(`\"`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func resolveBundledZigExecutable(runtimeDir string) (string, error) {
	zigBase := filepath.Join(runtimeDir, "zig")
	candidates := []string{
		filepath.Join(zigBase, "zig.exe"),
		filepath.Join(zigBase, "zig"),
	}

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
