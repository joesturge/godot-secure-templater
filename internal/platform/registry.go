package platform

import (
	"runtime"
	"strings"
	"sync"

	"github.com/joemi/godot-secure-templater/internal"
)

// Definition describes one target platform implementation.
type Definition struct {
	ID                  string
	TargetTuple         string
	SupportedHostTuples map[string]struct{}
	Components          func(version string) ([]internal.Artifact, *internal.Error)
	Compile             func(ctx *internal.RunContext, key string) *internal.Error
	ArtifactPaths       func(workspace *internal.Workspace) (releasePath string, debugPath string)
	SuccessNextSteps    func() []string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Definition{}

	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
)

// Register adds a platform definition to the registry.
func Register(def Definition) {
	id := strings.ToLower(strings.TrimSpace(def.ID))
	if id == "" {
		panic("platform.Register: empty platform id")
	}
	if def.Components == nil {
		panic("platform.Register: components resolver is nil")
	}
	if def.Compile == nil {
		panic("platform.Register: compiler callback is nil")
	}
	if def.ArtifactPaths == nil {
		panic("platform.Register: artifact-path callback is nil")
	}
	if def.SuccessNextSteps == nil {
		panic("platform.Register: success-next-steps callback is nil")
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[id]; exists {
		panic("platform.Register: duplicate platform id: " + id)
	}

	def.ID = id
	def.TargetTuple = NormalizeTuple(def.TargetTuple)
	if def.TargetTuple == "" {
		def.TargetTuple = id + "/amd64"
	}
	if def.SupportedHostTuples == nil {
		def.SupportedHostTuples = map[string]struct{}{}
	}

	normalizedHosts := map[string]struct{}{}
	for tuple := range def.SupportedHostTuples {
		normalizedHosts[NormalizeTuple(tuple)] = struct{}{}
	}
	def.SupportedHostTuples = normalizedHosts

	registry[id] = def
}

// Lookup retrieves a platform definition by id.
func Lookup(id string) (Definition, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	def, ok := registry[strings.ToLower(strings.TrimSpace(id))]
	return def, ok
}

// DetectHostTuple returns the executing host tuple (goos/goarch).
func DetectHostTuple() string {
	return strings.ToLower(runtimeGOOS + "/" + runtimeGOARCH)
}

// NormalizeTuple canonicalizes tuple input.
func NormalizeTuple(input string) string {
	tuple := strings.ToLower(strings.TrimSpace(input))
	if tuple == "" {
		return ""
	}

	switch tuple {
	case "windows":
		return "windows/amd64"
	case "linux":
		return "linux/amd64"
	}

	return tuple
}

// PlatformIDFromTuple extracts the platform id prefix from a tuple.
func PlatformIDFromTuple(tuple string) string {
	normalized := NormalizeTuple(tuple)
	parts := strings.Split(normalized, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// ResolveTargetTuple resolves an input tuple, defaulting to host tuple when blank.
func ResolveTargetTuple(raw string, hostTuple string) (string, *internal.Error) {
	tuple := NormalizeTuple(raw)
	if tuple == "" {
		tuple = NormalizeTuple(hostTuple)
	}
	if tuple == "" || !strings.Contains(tuple, "/") {
		return tuple, internal.ErrUnsupportedPlatformTuple(tuple)
	}
	return tuple, nil
}

// ValidateHostSupport checks whether host tuple is allowed for a selected platform.
func ValidateHostSupport(def Definition, hostTuple string) *internal.Error {
	normalizedHost := NormalizeTuple(hostTuple)
	if normalizedHost == "" {
		return internal.ErrHostTargetUnsupported("unknown", def.TargetTuple)
	}
	if len(def.SupportedHostTuples) == 0 {
		return nil
	}
	if _, ok := def.SupportedHostTuples[normalizedHost]; ok {
		return nil
	}
	return internal.ErrHostTargetUnsupported(normalizedHost, def.TargetTuple)
}
