package hostwindows

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
	def, ok := platform.LookupHostTarget("windows/amd64", "windows/amd64")

	// THEN the registry should contain the windows platform
	assert.True(t, ok, "Windows plugin should register a windows platform definition")
	assert.Equal(t, "windows/amd64", def.TargetTuple, "Windows plugin should target windows/amd64 tuple")

	// AND required callbacks should be available
	assert.NotNil(t, def.Components, "Windows platform should provide a component resolver callback")
	assert.NotNil(t, def.Compile, "Windows platform should provide a compile callback")
	assert.NotNil(t, def.ArtifactPaths, "Windows platform should provide artifact-path resolver callback")
	assert.NotNil(t, def.SuccessNextSteps, "Windows platform should provide success-next-steps callback")

	// AND host tuple should be windows/amd64
	assert.Equal(t, "windows/amd64", def.HostTuple, "Windows plugin should declare windows/amd64 host tuple")
}

func TestWindowsPluginComponents(t *testing.T) {
	// GIVEN a registered windows platform definition
	def, ok := platform.LookupHostTarget("windows/amd64", "windows/amd64")
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
	assert.True(t, names["zig"], "Windows component list should include zig artifact")
	assert.True(t, names["scons"], "Windows component list should include scons artifact")
	assert.True(t, names["godot_source"], "Windows component list should include godot_source artifact")

	for _, c := range components {
		if c.Name == "godot_source" {
			assert.Empty(t, c.SHA256, "Godot source should not require a checksum")
		}
	}
}

func TestWindowsPluginArtifactPaths(t *testing.T) {
	// GIVEN a registered windows platform definition and workspace path
	def, ok := platform.LookupHostTarget("windows/amd64", "windows/amd64")
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
	def, ok := platform.LookupHostTarget("windows/amd64", "windows/amd64")
	assert.True(t, ok, "Windows platform should exist in registry for success-next-steps tests")

	// WHEN resolving success next steps
	steps := def.SuccessNextSteps()

	// THEN plugin should provide non-empty, windows-specific steps
	assert.NotEmpty(t, steps, "Windows plugin should provide at least one success next step")
	assert.Contains(t, steps[0], "Godot Editor", "Windows success steps should guide the user in the editor flow")
	assert.Contains(t, steps[2], "windows_template_release.exe", "Windows success steps should mention the release template path")
	assert.Contains(t, steps[4], "encryption.key", "Windows success steps should mention the key file path")
}

func TestWindowsHostLinuxTargetRegistration(t *testing.T) {
	// GIVEN the windows plugin package init has registered host/target definitions

	// WHEN looking up the windows-host linux-target tuple pair
	def, ok := platform.LookupHostTarget("windows/amd64", "linux/amd64")

	// THEN the registry should contain the linux target entry for windows host
	assert.True(t, ok, "Windows plugin should register a windows/amd64 -> linux/amd64 tuple definition")
	if !ok {
		t.FailNow()
	}
	assert.Equal(t, "windows/amd64", def.HostTuple, "Host tuple should match windows/amd64")
	assert.Equal(t, "linux/amd64", def.TargetTuple, "Target tuple should match linux/amd64")

	// AND callbacks should exist to satisfy the platform definition contract
	assert.NotNil(t, def.Components, "Linux-target tuple entry should provide a component resolver callback")
	assert.NotNil(t, def.Compile, "Linux-target tuple entry should provide a compile callback")
	assert.NotNil(t, def.ArtifactPaths, "Linux-target tuple entry should provide artifact-path resolver callback")
	assert.NotNil(t, def.SuccessNextSteps, "Linux-target tuple entry should provide success-next-steps callback")

	// AND component resolution should be wired to real toolchain inputs
	components, err := def.Components("4.6.3")
	assert.Nil(t, err, "Linux-target tuple entry should resolve components without typed errors")
	assert.NotEmpty(t, components, "Linux-target tuple entry should return build components")

	// AND artifact paths and user guidance should reflect linux templates
	workspace := &internal.Workspace{Templates: filepath.Join("/tmp", "project", ".gst", "templates")}
	releasePath, debugPath := def.ArtifactPaths(workspace)
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "linux_template_release.x86_64"), releasePath, "Linux tuple release artifact path should use linux filename")
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "linux_template_debug.x86_64"), debugPath, "Linux tuple debug artifact path should use linux filename")

	steps := def.SuccessNextSteps()
	assert.NotEmpty(t, steps, "Linux-target tuple should provide success guidance")
	assert.Contains(t, steps[1], "Linux export preset", "Linux tuple steps should guide Linux preset wiring")
}
