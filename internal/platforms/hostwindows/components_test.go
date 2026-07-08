package hostwindows

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowsComponents(t *testing.T) {
	// GIVEN a Godot version
	version := "4.6.3"

	// WHEN calling Components
	components := Components(version)

	// THEN it should return 4 components
	assert.Equal(t, 4, len(components), "Should return exactly 4 components")

	// AND components should have correct names and URLs
	expectedNames := []string{"python", "zig", "scons", "godot_source"}
	for i, expectedName := range expectedNames {
		assert.Equal(t, expectedName, components[i].Name, "Component %d should be %s", i, expectedName)
		assert.NotEmpty(t, components[i].URL, "Component %d should have non-empty URL", i)
		if components[i].Name == "godot_source" {
			assert.Empty(t, components[i].SHA256, "Godot source should not require a checksum")
		} else {
			assert.NotEmpty(t, components[i].SHA256, "Component %d should have non-empty SHA256", i)
		}
		assert.NotEmpty(t, components[i].ExtractTo, "Component %d should have non-empty ExtractTo", i)
	}
}

func TestWindowsComponents_GodotURL(t *testing.T) {
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
			wantURL: "https://github.com/godotengine/godot/archive/refs/tags/4.7-stable.tar.gz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			// WHEN calling Components
			components := Components(tt.version)

			// THEN Godot source URL should match version
			godotComponent := components[3]
			assert.Equal(t, tt.wantURL, godotComponent.URL, "URL should include correct Godot version")
			assert.Empty(t, godotComponent.SHA256, "Godot source should not require a checksum")
		})
	}
}
