package platform

import (
	"runtime"
	"strings"
	"sync"

	"github.com/joemi/godot-secure-templater/internal"
)

// Definition describes one target platform implementation.
type Definition struct {
	HostTuple        string
	TargetTuple      string
	Components       func(version string) ([]internal.Artifact, *internal.Error)
	Compile          func(ctx *internal.RunContext, key string) *internal.Error
	Verify           func(ctx *internal.RunContext) *internal.Error
	ArtifactPaths    func(workspace *internal.Workspace) (releasePath string, debugPath string)
	SuccessNextSteps func() []string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Definition{}

	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
)

func hostTargetKey(hostTuple, targetTuple string) string {
	return NormalizeTuple(hostTuple) + "->" + NormalizeTuple(targetTuple)
}

// Register adds a platform definition to the registry.
func Register(def Definition) {
	if def.Components == nil {
		panic("platform.Register: components resolver is nil")
	}
	if def.Compile == nil {
		panic("platform.Register: compiler callback is nil")
	}
	if def.Verify == nil {
		panic("platform.Register: verify callback is nil")
	}
	if def.ArtifactPaths == nil {
		panic("platform.Register: artifact-path callback is nil")
	}
	if def.SuccessNextSteps == nil {
		panic("platform.Register: success-next-steps callback is nil")
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	def.HostTuple = NormalizeTuple(def.HostTuple)
	if def.HostTuple == "" {
		panic("platform.Register: empty host tuple")
	}
	def.TargetTuple = NormalizeTuple(def.TargetTuple)
	if def.TargetTuple == "" {
		panic("platform.Register: empty target tuple")
	}

	pairKey := hostTargetKey(def.HostTuple, def.TargetTuple)
	if _, exists := registry[pairKey]; exists {
		panic("platform.Register: duplicate host/target tuple pair: " + pairKey)
	}

	registry[pairKey] = def
}

// LookupHostTarget retrieves a platform definition by exact host/target tuple pair.
func LookupHostTarget(hostTuple, targetTuple string) (Definition, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	def, ok := registry[hostTargetKey(hostTuple, targetTuple)]
	return def, ok
}

// IsTargetRegistered reports whether any host supports the target tuple.
func IsTargetRegistered(targetTuple string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()

	normalizedTarget := NormalizeTuple(targetTuple)
	for _, def := range registry {
		if def.TargetTuple == normalizedTarget {
			return true
		}
	}
	return false
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
	}

	return tuple
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
	if NormalizeTuple(def.HostTuple) == normalizedHost {
		return nil
	}
	return internal.ErrHostTargetUnsupported(normalizedHost, def.TargetTuple)
}
