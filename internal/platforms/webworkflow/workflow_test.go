package webworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/stretchr/testify/assert"
)

func TestBuildEnvIncludesEmscriptenInPythonPath(t *testing.T) {
	tests := []struct {
		name      string
		hostTuple string
		separator string
	}{
		{name: "windows host", hostTuple: "windows/amd64", separator: ";"},
		{name: "linux host", hostTuple: "linux/amd64", separator: string(os.PathListSeparator)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a runtime directory with an emsdk layout
			runtimeDir := t.TempDir()
			sdkRoot := filepath.Join(runtimeDir, "emsdk")
			emscriptenDir := filepath.Join(sdkRoot, "upstream", "emscripten")
			err := os.MkdirAll(emscriptenDir, 0755)
			assert.NoError(t, err, "emscripten directory should be creatable")
			err = os.WriteFile(filepath.Join(sdkRoot, "emsdk.py"), []byte("# test"), 0644)
			assert.NoError(t, err, "emsdk.py should be creatable")

			// WHEN building the env for the web workflow
			ws := &internal.Workspace{Runtime: runtimeDir}
			env := buildEnv(ws, tt.hostTuple, "test-key")

			// THEN PYTHONPATH must include the emscripten directory so that em++.py can import emcc
			pythonPath, ok := env["PYTHONPATH"]
			assert.True(t, ok, "PYTHONPATH must be set in the web build environment")
			parts := strings.Split(pythonPath, tt.separator)
			var found bool
			for _, p := range parts {
				if p == emscriptenDir {
					found = true
					break
				}
			}
			assert.True(t, found, "PYTHONPATH should contain the emscripten directory (%s), got: %s", emscriptenDir, pythonPath)
		})
	}
}

func TestResolveSDKRoot(t *testing.T) {
	tests := []struct {
		name   string
		nested bool
	}{
		{name: "stripped archive root", nested: false},
		{name: "nested archive root", nested: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN an extracted emsdk archive layout
			runtimeDir := t.TempDir()
			sdkRoot := filepath.Join(runtimeDir, "emsdk")
			if tt.nested {
				sdkRoot = filepath.Join(sdkRoot, "emsdk-3.1.62")
			}
			err := os.MkdirAll(sdkRoot, 0755)
			assert.NoError(t, err, "SDK root should be creatable")
			err = os.WriteFile(filepath.Join(sdkRoot, "emsdk.py"), []byte("# test"), 0644)
			assert.NoError(t, err, "emsdk.py should be creatable")

			// WHEN resolving the SDK root
			resolved := resolveSDKRoot(runtimeDir)

			// THEN the root containing emsdk.py should be returned
			assert.Equal(t, sdkRoot, resolved, "resolveSDKRoot should handle archive root layout")
		})
	}
}
