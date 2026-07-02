package version

import (
	"fmt"
	"regexp"
	"strings"
)

// Method represents how a version was resolved.
type Method string

const (
	MethodExplicit    Method = "explicit"
	MethodLocalEditor Method = "local-editor"
	MethodGitHubAPI   Method = "github-api"
	MethodInteractive Method = "interactive"
)

// Resolution is the result of resolving a Godot version.
type Resolution struct {
	// Version is the resolved semantic version (e.g., "4.3.0").
	Version string

	// Method is how the version was determined.
	Method Method

	// Source provides additional context (e.g., "/usr/bin/godot" for local editor, "GitHub API" for releases).
	Source string
}

// Resolver encapsulates version resolution logic with pluggable strategies.
type Resolver struct {
	// strategies are tried in order until one succeeds.
	strategies []ResolutionStrategy
}

// ResolutionStrategy defines how to attempt resolving a version.
type ResolutionStrategy interface {
	// Resolve returns the resolved version, or nil if this strategy doesn't apply / fails.
	// If an error is returned, it represents an actionable failure (e.g., invalid path, API error).
	Resolve() (*Resolution, error)
}

// NewResolver creates a Resolver with strategies in priority order.
func NewResolver(strategies ...ResolutionStrategy) *Resolver {
	return &Resolver{
		strategies: strategies,
	}
}

// Resolve attempts each strategy in order until one succeeds.
// Returns an error only if a strategy explicitly fails (not if all just decline to apply).
func (r *Resolver) Resolve() (*Resolution, error) {
	var lastErr error

	for _, s := range r.strategies {
		resolution, err := s.Resolve()
		if err != nil {
			// Strategy encountered an actionable failure. Propagate it.
			return nil, err
		}
		if resolution != nil {
			return resolution, nil
		}
		// Strategy declined (nil, nil); try next.
	}

	// All strategies declined.
	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("no version resolution strategy succeeded")
}

// NormalizeVersion takes a Godot version string (possibly with build metadata like "4.3.1.stable.official")
// and returns the semantic version (e.g., "4.3.1").
// Returns error if the version doesn't match expected patterns.
func NormalizeVersion(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("version string is empty")
	}

	// Match semantic version at the start: X.Y.Z
	// Build metadata (e.g., ".stable.official") is stripped.
	pattern := regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
	matches := pattern.FindStringSubmatch(input)

	if len(matches) < 2 {
		return "", fmt.Errorf("invalid version format: %s (expected X.Y.Z or X.Y.Z.metadata)", input)
	}

	return matches[1], nil
}

// ExtractMinor extracts the major.minor from a version (e.g., "4.3.0" → "4.3").
func ExtractMinor(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("version %s does not contain major.minor", version)
	}
	return parts[0] + "." + parts[1], nil
}

// ExplicitStrategy returns the version if explicitly provided.
type ExplicitStrategy struct {
	Version string
}

func (s *ExplicitStrategy) Resolve() (*Resolution, error) {
	if s.Version == "" {
		return nil, nil // Doesn't apply; try next strategy.
	}

	normalized, err := NormalizeVersion(s.Version)
	if err != nil {
		return nil, fmt.Errorf("explicit version invalid: %w", err)
	}

	return &Resolution{
		Version: normalized,
		Method:  MethodExplicit,
		Source:  "command-line flag",
	}, nil
}

// LocalEditorStrategy attempts to locate and query a Godot editor on the system.
// For now, this is a stub; Slice 1 expands it to check PATH, common install locations, etc.
type LocalEditorStrategy struct {
	EditorPath string // "" means use system defaults or PATH
}

func (s *LocalEditorStrategy) Resolve() (*Resolution, error) {
	// Stub: In Slice 1, implement:
	// 1. If EditorPath is set, run it with --version
	// 2. Check PATH for "godot"
	// 3. Check common install paths (/usr/bin/godot, %ProgramFiles%/Godot, etc.)
	// 4. Parse the output (e.g., "Godot v4.3.0.stable.official")
	return nil, nil
}

// GitHubAPIStrategy attempts to fetch the latest published patch version from GitHub releases.
// For now, this is a stub; Slice 1 implements the actual API call with caching.
type GitHubAPIStrategy struct {
	// Major.Minor to fetch the latest patch for (e.g., "4.3")
	MinorVersion string
	// Optional auth token for higher rate limits
	AuthToken string
	// Optional cache directory for release metadata
	CacheDir string
}

func (s *GitHubAPIStrategy) Resolve() (*Resolution, error) {
	// Stub: In Slice 1, implement:
	// 1. Check cache for recent release metadata
	// 2. If cache miss or stale, query GitHub API /repos/godotengine/godot/releases
	// 3. Filter for releases matching MinorVersion (e.g., v4.3.x)
	// 4. Pick the latest stable patch
	// 5. Cache the result with timestamp
	// 6. Handle API errors (rate limit, offline) as actionable errors
	return nil, nil
}

// InteractiveStrategy prompts the user for a version (stub for now).
type InteractiveStrategy struct {
	// Prompt function for testing; if nil, use stdin
	PromptFunc func(prompt string) (string, error)
}

func (s *InteractiveStrategy) Resolve() (*Resolution, error) {
	// Stub: In Slice 1, implement:
	// 1. Print a prompt asking for version
	// 2. Read user input (or use PromptFunc for testing)
	// 3. Validate and normalize
	// 4. Return Resolution with Method=MethodInteractive
	return nil, nil
}
