package linux

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
	def, ok := platform.Lookup("linux")

	// THEN the registry should contain the linux platform
	assert.True(t, ok, "Linux plugin should register a linux platform definition")
	assert.Equal(t, "linux", def.ID, "Linux platform id should be normalized to linux")
	assert.Equal(t, "linux/amd64", def.TargetTuple, "Linux plugin should target linux/amd64 tuple")

	// AND required callbacks should be available
	assert.NotNil(t, def.Components, "Linux platform should provide a component resolver callback")
	assert.NotNil(t, def.Compile, "Linux platform should provide a compile callback")
	assert.NotNil(t, def.ArtifactPaths, "Linux platform should provide artifact-path resolver callback")
	assert.NotNil(t, def.SuccessNextSteps, "Linux platform should provide success-next-steps callback")

	// AND optimistic host support should include linux/amd64 and windows/amd64
	_, linuxHost := def.SupportedHostTuples["linux/amd64"]
	_, windowsHost := def.SupportedHostTuples["windows/amd64"]
	assert.True(t, linuxHost, "Linux plugin should declare linux/amd64 host compatibility")
	assert.True(t, windowsHost, "Linux plugin should declare windows/amd64 host compatibility")
}

func TestLinuxPluginReturnsNotImplemented(t *testing.T) {
	// GIVEN a registered linux platform definition
	def, ok := platform.Lookup("linux")
	assert.True(t, ok, "Linux platform should exist in registry for not-implemented behaviour tests")

	// WHEN resolving components and invoking compile callback
	components, componentsErr := def.Components("4.6.3")
	compileErr := def.Compile(nil, "")
	templatesDir := filepath.Join("/tmp", "project", ".gst", "templates")
	releasePath, debugPath := def.ArtifactPaths(&internal.Workspace{Templates: templatesDir})

	// THEN components should fail with typed not-implemented error
	assert.Nil(t, components, "Linux component resolver should return nil components while unimplemented")
	assert.NotNil(t, componentsErr, "Linux component resolver should return typed not-implemented error")
	assert.Equal(t, internal.ExitUsageError, componentsErr.Code, "Linux component resolver should use usage-error exit code while unimplemented")
	assert.Contains(t, componentsErr.Message, "not yet implemented", "Linux component resolver should mention not-yet-implemented status")

	// AND compile should fail with typed not-implemented error
	assert.NotNil(t, compileErr, "Linux compile callback should return typed not-implemented error")
	assert.Equal(t, internal.ExitUsageError, compileErr.Code, "Linux compile callback should use usage-error exit code while unimplemented")
	assert.Contains(t, compileErr.Message, "not yet implemented", "Linux compile callback should mention not-yet-implemented status")

	// AND artifact paths should resolve deterministically for manifest hashing
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "linux_template_release"), releasePath, "Linux release artifact path should use deterministic filename")
	assert.Equal(t, filepath.Join("/tmp", "project", ".gst", "templates", "linux_template_debug"), debugPath, "Linux debug artifact path should use deterministic filename")

	// AND success next steps should indicate implementation status
	steps := def.SuccessNextSteps()
	assert.NotEmpty(t, steps, "Linux plugin should provide at least one success-next-step message")
	assert.Contains(t, steps[0], "not implemented", "Linux success message should signal implementation status")
}
