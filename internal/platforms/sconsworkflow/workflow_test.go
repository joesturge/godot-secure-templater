package sconsworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
)

func TestAdapterForHostTuple(t *testing.T) {
	tests := []struct {
		name       string
		hostTuple  string
		expectsWin bool
	}{
		{
			name:       "windows host selects windows adapter",
			hostTuple:  "windows/amd64",
			expectsWin: true,
		},
		{
			name:       "linux host selects posix adapter",
			hostTuple:  "linux/amd64",
			expectsWin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a host tuple

			// WHEN resolving the host adapter
			adapter := AdapterForHostTuple(tt.hostTuple)

			// THEN adapter type should match host family
			_, isWindowsAdapter := adapter.(windowsHostAdapter)
			assert.Equal(t, tt.expectsWin, isWindowsAdapter, "AdapterForHostTuple should return expected adapter type for host tuple")
		})
	}
}

func TestPosixHostBuildEnv(t *testing.T) {
	// GIVEN a POSIX host adapter and workspace paths
	t.Setenv("PATH", "/host/system/bin")
	adapter := posixHostAdapter{}
	workspace := &internal.Workspace{Runtime: filepath.Join("/tmp", "runtime")}

	// WHEN building environment overrides
	env := adapter.BuildEnv(workspace, "test-key")

	// THEN PATH should use POSIX separator, include python/bin, and set the key
	assert.Contains(t, env["PATH"], ":", "POSIX PATH should use colon separator")
	assert.Contains(t, env["PATH"], filepath.Join("/tmp", "runtime", "python", "bin"),
		"POSIX PATH should include python/bin so the provisioned python binary is reachable")
	assert.Contains(t, env["PATH"], filepath.Join("/tmp", "runtime", "bin"),
		"POSIX PATH should include runtime/bin so the provisioned pkg-config stub is reachable")
	assert.True(t, strings.HasPrefix(env["PATH"], filepath.Join("/tmp", "runtime", "bin")+":"),
		"POSIX PATH should prioritise provisioned runtime tools")
	assert.True(t, strings.HasSuffix(env["PATH"], ":/host/system/bin"),
		"POSIX PATH should retain host system utilities after provisioned runtime tools")
	assert.Equal(t, "test-key", env["SCRIPT_AES256_ENCRYPTION_KEY"], "BuildEnv should include encryption key override")
}

func TestResolvePythonExecutableFailsWithoutRuntimePython(t *testing.T) {
	// GIVEN no provisioned runtime python
	runtimeDir := t.TempDir()

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should fail fast instead of falling back to host binaries
	assert.Error(t, err, "resolvePythonExecutable should fail when runtime python is missing")
	assert.Equal(t, filepath.Join(runtimeDir, "python", "python"), resolved, "resolvePythonExecutable should return the expected runtime python path on POSIX hosts")
}

func TestResolvePythonExecutable_BinLayout(t *testing.T) {
	// GIVEN a python-build-standalone layout (python/bin/python)
	runtimeDir := t.TempDir()
	binDir := filepath.Join(runtimeDir, "python", "bin")
	err := os.MkdirAll(binDir, 0755)
	assert.NoError(t, err, "python/bin directory should be creatable")

	pythonPath := filepath.Join(binDir, "python")
	err = os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "python/bin/python executable should be creatable")

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should find the bin/python layout
	assert.NoError(t, err, "resolvePythonExecutable should succeed for python-build-standalone layout")
	assert.Equal(t, pythonPath, resolved, "resolvePythonExecutable should return python/bin/python for python-build-standalone layout")
}

func TestResolvePythonExecutable_NestedSubdirLayout(t *testing.T) {
	// GIVEN a python-build-standalone layout where the archive extracts to a
	// nested python/ subdirectory (e.g. runtime/python/python/bin/python3)
	runtimeDir := t.TempDir()
	binDir := filepath.Join(runtimeDir, "python", "python", "bin")
	err := os.MkdirAll(binDir, 0755)
	assert.NoError(t, err, "python/python/bin directory should be creatable")

	pythonPath := filepath.Join(binDir, "python3")
	err = os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "python/python/bin/python3 executable should be creatable")

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should find the nested subdirectory layout
	assert.NoError(t, err, "resolvePythonExecutable should succeed for nested python-build-standalone layout")
	assert.Equal(t, pythonPath, resolved, "resolvePythonExecutable should return python/python/bin/python3 for nested layout")
}

func TestResolvePythonExecutable_BinLayoutVersionedOnly(t *testing.T) {
	// GIVEN a python-build-standalone layout where symlinks were not extracted
	// and only the versioned binary (e.g. python3.11) exists in bin/
	runtimeDir := t.TempDir()
	binDir := filepath.Join(runtimeDir, "python", "bin")
	err := os.MkdirAll(binDir, 0755)
	assert.NoError(t, err, "python/bin directory should be creatable")

	pythonPath := filepath.Join(binDir, "python3.11")
	err = os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "python/bin/python3.11 executable should be creatable")

	// WHEN resolving the python executable
	resolved, err := resolvePythonExecutable(runtimeDir)

	// THEN it should find the versioned binary even without symlinks
	assert.NoError(t, err, "resolvePythonExecutable should succeed when only python3.11 is present")
	assert.Equal(t, pythonPath, resolved, "resolvePythonExecutable should return python/bin/python3.11 when symlinks are absent")
}

func TestBuildCommand_InjectsPythonPathForSConsPackage(t *testing.T) {
	// GIVEN a packaged SCons module and a runtime PYTHONPATH that includes the emscripten SDK
	oldValue := os.Getenv("PYTHONPATH")
	empPath := filepath.Join("C:\\", "runtime", "emsdk", "upstream", "emscripten") + string(os.PathListSeparator) + filepath.Join("C:\\", "runtime", "scons")
	err := os.Setenv("PYTHONPATH", empPath)
	assert.NoError(t, err, "PYTHONPATH should be set for the test")
	defer func() {
		if oldValue == "" {
			_ = os.Unsetenv("PYTHONPATH")
			return
		}
		_ = os.Setenv("PYTHONPATH", oldValue)
	}()

	sconsMain := filepath.Join("C:\\", "runtime", "scons", "SCons", "__main__.py")
	wrapper := BuildCommand("python.exe", sconsMain, []string{"--version"}, internal.NewSimpleLogger(false))

	// WHEN bootstrapping SCons via the package __main__.py entry point
	// THEN the generated python -c payload must preserve PYTHONPATH entries before SCons executes
	assert.Equal(t, "-c", wrapper.Args[1], "BuildCommand should use a python -c bootstrap")
	assert.Contains(t, wrapper.Args[2], "os.environ.get('PYTHONPATH'", "python bootstrap must inspect PYTHONPATH")
	assert.Contains(t, wrapper.Args[2], "os.pathsep", "python bootstrap must split PYTHONPATH using the host path separator")
	assert.Contains(t, wrapper.Args[2], "os.environ['PYTHONPATH']", "python bootstrap must re-export PYTHONPATH so child subprocesses inherit the Emscripten paths")
	assert.Contains(t, wrapper.Args[2], "sys.path.insert(0, entry)", "python bootstrap must restore PYTHONPATH entries before startup")
	assert.Contains(t, wrapper.Args[2], "runtime/scons", "python bootstrap must include the SCons runtime in sys.path")
}

func TestResolveZigExecutable_RuntimeRoot(t *testing.T) {
	// GIVEN a runtime directory with zig at the runtime root layout
	runtimeDir := t.TempDir()
	zigDir := filepath.Join(runtimeDir, "zig")
	err := os.MkdirAll(zigDir, 0755)
	assert.NoError(t, err, "Runtime zig directory should be creatable")

	zigPath := filepath.Join(zigDir, "zig")
	err = os.WriteFile(zigPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "Root zig executable should be creatable")

	// WHEN resolving the zig executable
	resolved, err := resolveZigExecutable(runtimeDir)

	// THEN it should return the runtime root zig executable
	assert.NoError(t, err, "resolveZigExecutable should resolve root runtime zig executable")
	assert.Equal(t, zigPath, resolved, "resolveZigExecutable should return runtime root zig path")
}

func TestResolveZigExecutable_NestedLayout(t *testing.T) {
	// GIVEN a runtime directory with zig in nested extracted layout
	runtimeDir := t.TempDir()
	nestedDir := filepath.Join(runtimeDir, "zig", "zig-x86_64-linux-0.16.0")
	err := os.MkdirAll(nestedDir, 0755)
	assert.NoError(t, err, "Nested zig directory should be creatable")

	zigPath := filepath.Join(nestedDir, "zig")
	err = os.WriteFile(zigPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "Nested zig executable should be creatable")

	// WHEN resolving the zig executable
	resolved, err := resolveZigExecutable(runtimeDir)

	// THEN it should return the nested zig executable
	assert.NoError(t, err, "resolveZigExecutable should resolve nested runtime zig executable")
	assert.Equal(t, zigPath, resolved, "resolveZigExecutable should return nested zig path")
}

func TestResolveExeFromEnvPath_FindsExeByName(t *testing.T) {
	// GIVEN a bin directory with a gcc executable
	binDir := t.TempDir()
	gccPath := filepath.Join(binDir, "gcc")
	err := os.WriteFile(gccPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "gcc executable should be creatable")

	// WHEN resolving gcc from the env PATH
	resolved, err := resolveExeFromEnvPath("gcc", binDir, ":")

	// THEN it should return the gcc executable path
	assert.NoError(t, err, "resolveExeFromEnvPath should find gcc in PATH")
	assert.Equal(t, gccPath, resolved, "resolveExeFromEnvPath should return the correct gcc path")
}

func TestResolveExeFromEnvPath_FindsExeExtension(t *testing.T) {
	// GIVEN a bin directory with a gcc.exe executable (Windows-style)
	binDir := t.TempDir()
	gccPath := filepath.Join(binDir, "gcc.exe")
	err := os.WriteFile(gccPath, []byte(""), 0755)
	assert.NoError(t, err, "gcc.exe executable should be creatable")

	// WHEN resolving gcc from the env PATH using semicolon separator
	resolved, err := resolveExeFromEnvPath("gcc", binDir, ";")

	// THEN it should find gcc.exe when gcc is absent
	assert.NoError(t, err, "resolveExeFromEnvPath should find gcc.exe in PATH")
	assert.Equal(t, gccPath, resolved, "resolveExeFromEnvPath should return gcc.exe when gcc is absent")
}

func TestResolveExeFromEnvPath_ReturnsErrorWhenNotFound(t *testing.T) {
	// GIVEN a bin directory without the requested executable
	binDir := t.TempDir()

	// WHEN resolving a missing executable
	_, err := resolveExeFromEnvPath("ar", binDir, ":")

	// THEN it should return an error
	assert.Error(t, err, "resolveExeFromEnvPath should fail when executable is absent from PATH")
}

func TestResolveExeFromEnvPath_SearchesMultiplePathEntries(t *testing.T) {
	// GIVEN two bin directories where ar is only in the second
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	arPath := filepath.Join(dir2, "ar")
	err := os.WriteFile(arPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "ar executable should be creatable")

	envPath := dir1 + ":" + dir2

	// WHEN resolving ar from the multi-entry env PATH
	resolved, err := resolveExeFromEnvPath("ar", envPath, ":")

	// THEN it should find ar in the second directory
	assert.NoError(t, err, "resolveExeFromEnvPath should search all PATH entries")
	assert.Equal(t, arPath, resolved, "resolveExeFromEnvPath should return ar from the second PATH entry")
}

func TestResolveExeFromEnvPath_AbsolutePathUsedDirectly(t *testing.T) {
	// GIVEN an absolute executable path that exists on disk
	dir := t.TempDir()
	gccPath := filepath.Join(dir, "gcc")
	err := os.WriteFile(gccPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	assert.NoError(t, err, "gcc executable should be creatable")

	// WHEN resolving using an absolute path as the name
	resolved, err := resolveExeFromEnvPath(gccPath, "", ":")

	// THEN it should return the absolute path without searching PATH
	assert.NoError(t, err, "resolveExeFromEnvPath should accept an absolute path directly")
	assert.Equal(t, gccPath, resolved, "resolveExeFromEnvPath should return the absolute path unchanged")
}
