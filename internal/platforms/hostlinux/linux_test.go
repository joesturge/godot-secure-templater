package hostlinux

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/platform"
)

func TestLinuxPluginRegistersDefinition(t *testing.T) {
	// GIVEN the linux platform plugin package init has registered itself

	// WHEN looking up the linux platform definition
	def, ok := platform.LookupHostTarget("linux/amd64", "linux/amd64")

	// THEN the registry should contain the linux platform
	assert.True(t, ok, "Linux plugin should register a linux platform definition")
	assert.Equal(t, "linux/amd64", def.TargetTuple, "Linux plugin should target linux/amd64 tuple")

	// AND required callbacks should be available
	assert.NotNil(t, def.Components, "Linux platform should provide a component resolver callback")
	assert.NotNil(t, def.Compile, "Linux platform should provide a compile callback")
	assert.NotNil(t, def.ArtifactPaths, "Linux platform should provide artifact-path resolver callback")
	assert.NotNil(t, def.SuccessNextSteps, "Linux platform should provide success-next-steps callback")

	// AND host tuple should be linux/amd64
	assert.Equal(t, "linux/amd64", def.HostTuple, "Linux plugin should declare linux/amd64 host tuple")
}

func TestLinuxPluginComponents(t *testing.T) {
	// GIVEN a registered linux platform definition
	def, ok := platform.LookupHostTarget("linux/amd64", "linux/amd64")
	assert.True(t, ok, "Linux platform should exist in registry for component-resolution tests")

	// WHEN resolving components for a known Godot version
	components, err := def.Components("4.6.3")

	// THEN the resolver should succeed
	assert.Nil(t, err, "Linux component resolver should not return typed errors for supported versions")
	assert.NotEmpty(t, components, "Linux component resolver should return at least one artifact")

	// AND the expected component names should be present
	names := map[string]bool{}
	for _, c := range components {
		names[c.Name] = true
	}
	assert.True(t, names["python"], "Linux component list should include python artifact")
	assert.True(t, names["zig"], "Linux component list should include zig artifact")
	assert.True(t, names["scons"], "Linux component list should include scons artifact")
	assert.True(t, names["godot_source"], "Linux component list should include godot_source artifact")
}

func TestLinuxPluginArtifactPaths(t *testing.T) {
	// GIVEN a registered linux platform definition and workspace path
	def, ok := platform.LookupHostTarget("linux/amd64", "linux/amd64")
	assert.True(t, ok, "Linux platform should exist in registry for artifact-path tests")

	workspace := &internal.Workspace{Templates: filepath.Join("/tmp", "project", ".gst", "templates")}

	// WHEN resolving plugin artifact paths
	releasePath, debugPath := def.ArtifactPaths(workspace)

	// THEN linux template paths should map to expected filenames
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "linux_template_release.x86_64"), releasePath, "Linux release artifact path should use expected filename")
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "linux_template_debug.x86_64"), debugPath, "Linux debug artifact path should use expected filename")
}
