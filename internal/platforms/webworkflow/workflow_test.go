package webworkflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
