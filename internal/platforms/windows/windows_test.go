package windows

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/platform"
)

func TestWindowsPluginRegistersDefinition(t *testing.T) {
	// GIVEN the windows platform plugin package init has registered itself

	// WHEN looking up the windows platform definition
	def, ok := platform.Lookup("windows")

	// THEN the registry should contain the windows platform
	assert.True(t, ok, "Windows plugin should register a windows platform definition")
	assert.Equal(t, "windows", def.ID, "Windows platform id should be normalized to windows")
	assert.Equal(t, "windows/amd64", def.TargetTuple, "Windows plugin should target windows/amd64 tuple")

	// AND required callbacks should be available
	assert.NotNil(t, def.Components, "Windows platform should provide a component resolver callback")
	assert.NotNil(t, def.Compile, "Windows platform should provide a compile callback")
	assert.NotNil(t, def.ConfigureProject, "Windows platform should provide a configure-project callback")
	assert.NotNil(t, def.ArtifactPaths, "Windows platform should provide artifact-path resolver callback")
	assert.NotNil(t, def.SuccessNextSteps, "Windows platform should provide success-next-steps callback")

	// AND host support should include windows/amd64
	_, hostSupported := def.SupportedHostTuples["windows/amd64"]
	assert.True(t, hostSupported, "Windows plugin should declare windows/amd64 host compatibility")
}

func TestWindowsPluginComponents(t *testing.T) {
	// GIVEN a registered windows platform definition
	def, ok := platform.Lookup("windows")
	assert.True(t, ok, "Windows platform should exist in registry for component-resolution tests")

	// WHEN resolving components for a known Godot version
	components, err := def.Components("4.6.3")

	// THEN the resolver should succeed
	assert.Nil(t, err, "Windows component resolver should not return typed errors for supported versions")
	assert.NotEmpty(t, components, "Windows component resolver should return at least one artifact")

	// AND the expected core component names should be present
	names := map[string]bool{}
	for _, c := range components {
		names[c.Name] = true
	}
	assert.True(t, names["python"], "Windows component list should include python artifact")
	assert.True(t, names["mingw"], "Windows component list should include mingw artifact")
	assert.True(t, names["scons"], "Windows component list should include scons artifact")
	assert.True(t, names["godot_source"], "Windows component list should include godot_source artifact")
}

func TestWindowsPluginConfigureProject(t *testing.T) {
	// GIVEN a temporary project and workspace layout
	def, ok := platform.Lookup("windows")
	assert.True(t, ok, "Windows platform should exist in registry for configure-project tests")

	tmpDir := t.TempDir()
	workspace := &internal.Workspace{Templates: filepath.Join(tmpDir, ".gst", "templates")}

	key := "abcdef0123456789"
	logger := internal.NewSimpleLogger(false)

	// WHEN applying plugin-specific project configuration
	configureErr := def.ConfigureProject(tmpDir, workspace, "4.6.3", key, logger)

	// THEN configuration injection should be a no-op in manual setup mode
	assert.Nil(t, configureErr, "Windows configure-project callback should not mutate project config in manual mode")

	// AND no export config files should be created by the plugin
	assert.NoFileExists(t, filepath.Join(tmpDir, "export_presets.cfg"), "Windows configure-project should not create export_presets.cfg in manual mode")
	assert.NoFileExists(t, filepath.Join(tmpDir, ".godot", "export_credentials.cfg"), "Windows configure-project should not create export_credentials.cfg in manual mode")
}

func TestWindowsPluginArtifactPaths(t *testing.T) {
	// GIVEN a registered windows platform definition and workspace path
	def, ok := platform.Lookup("windows")
	assert.True(t, ok, "Windows platform should exist in registry for artifact-path tests")

	workspace := &internal.Workspace{Templates: filepath.Join("/tmp", "project", ".gst", "templates")}

	// WHEN resolving plugin artifact paths
	releasePath, debugPath := def.ArtifactPaths(workspace)

	// THEN windows template paths should map to expected filenames
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "windows_template_release.exe"), releasePath, "Windows release artifact path should use expected filename")
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "windows_template_debug.exe"), debugPath, "Windows debug artifact path should use expected filename")
}

func TestWindowsPluginSuccessNextSteps(t *testing.T) {
	// GIVEN a registered windows platform definition
	def, ok := platform.Lookup("windows")
	assert.True(t, ok, "Windows platform should exist in registry for success-next-steps tests")

	// WHEN resolving success next steps
	steps := def.SuccessNextSteps()

	// THEN plugin should provide non-empty, windows-specific steps
	assert.NotEmpty(t, steps, "Windows plugin should provide at least one success next step")
	assert.Contains(t, steps[0], "Godot Editor", "Windows success steps should guide the user in the editor flow")
	assert.Contains(t, steps[2], "windows_template_release.exe", "Windows success steps should mention the release template path")
	assert.Contains(t, steps[4], "encryption.key", "Windows success steps should mention the key file path")
}
