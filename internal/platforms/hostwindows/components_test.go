package hostwindows

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal/toolchain"
)

func TestWindowsComponents(t *testing.T) {
	// GIVEN a Godot version
	version := "4.6.3"
	resolveGodotChecksum = func(version string) string { return "stub-checksum" }
	defer func() {
		resolveGodotChecksum = toolchain.GodotChecksumForVersion
	}()

	// WHEN calling Components
	components := Components(version)

	// THEN it should return 4 components
	assert.Equal(t, 4, len(components), "Should return exactly 4 components")

	// AND components should have correct names and URLs
	expectedNames := []string{"python", "zig", "scons", "godot_source"}
	for i, expectedName := range expectedNames {
		assert.Equal(t, expectedName, components[i].Name, "Component %d should be %s", i, expectedName)
		assert.NotEmpty(t, components[i].URL, "Component %d should have non-empty URL", i)
		assert.NotEmpty(t, components[i].SHA256, "Component %d should have non-empty SHA256", i)
		assert.NotEmpty(t, components[i].ExtractTo, "Component %d should have non-empty ExtractTo", i)
	}
}

func TestWindowsComponents_GodotURL(t *testing.T) {
	resolveGodotChecksum = func(version string) string { return "stub-checksum" }
	defer func() {
		resolveGodotChecksum = toolchain.GodotChecksumForVersion
	}()

	// GIVEN different Godot versions
	tests := []struct {
		version string
		wantURL string
	}{
		{
			version: "4.6.3",
			wantURL: "https://github.com/godotengine/godot/archive/refs/tags/4.6.3-stable.tar.gz",
		},
		{
			version: "4.7.0",
			wantURL: "https://github.com/godotengine/godot/archive/refs/tags/4.7.0-stable.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			// WHEN calling Components
			components := Components(tt.version)

			// THEN Godot source URL should match version
			godotComponent := components[3]
			assert.Equal(t, tt.wantURL, godotComponent.URL, "URL should include correct Godot version")
			assert.Equal(t, "stub-checksum", godotComponent.SHA256, "Checksum should come from resolver callback")
		})
	}
}
