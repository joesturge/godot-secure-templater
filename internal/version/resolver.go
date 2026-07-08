package version

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
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
// and returns the semantic version (e.g., "4.3.1"). Patchless feature releases such as "4.7" normalize
// to "4.7.0" so the rest of the tool can treat them as semver.
// Returns error if the version doesn't match expected patterns.
func NormalizeVersion(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("version string is empty")
	}

	// Match semantic version at the start: X.Y or X.Y.Z
	// Build metadata (e.g., ".stable.official") is stripped.
	pattern := regexp.MustCompile(`^(\d+\.\d+(?:\.\d+)?)`)
	matches := pattern.FindStringSubmatch(input)

	if len(matches) < 2 {
		return "", fmt.Errorf("invalid version format: %s (expected X.Y or X.Y.Z, optionally with metadata)", input)
	}

	version := matches[1]
	if strings.Count(version, ".") == 1 {
		version += ".0"
	}

	return version, nil
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
	candidates := []string{}

	if s.EditorPath != "" {
		candidates = append(candidates, s.EditorPath)
	} else {
		for _, name := range []string{"godot4", "godot"} {
			if resolved, err := exec.LookPath(name); err == nil {
				candidates = append(candidates, resolved)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	var lastErr error
	for _, candidate := range candidates {
		cmd := exec.Command(candidate, "--version")
		output, err := cmd.Output()
		if err != nil {
			lastErr = err
			continue
		}

		normalized, normalizeErr := normalizeFromToolOutput(string(output))
		if normalizeErr != nil {
			lastErr = normalizeErr
			continue
		}

		return &Resolution{
			Version: normalized,
			Method:  MethodLocalEditor,
			Source:  candidate,
		}, nil
	}

	if s.EditorPath != "" {
		return nil, fmt.Errorf("local editor version resolution failed for %s: %w", s.EditorPath, lastErr)
	}

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
	if s.MinorVersion == "" {
		return nil, nil
	}

	if cached := s.readCache(); cached != "" {
		return &Resolution{Version: cached, Method: MethodGitHubAPI, Source: "GitHub API (cache)"}, nil
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/godotengine/godot/releases", nil)
	if err != nil {
		return nil, fmt.Errorf("construct github request: %w", err)
	}
	if s.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.AuthToken)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github releases query failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases query returned status %d", resp.StatusCode)
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode github releases: %w", err)
	}

	best := pickLatestPatchForMinor(releases, s.MinorVersion)
	if best == "" {
		return nil, fmt.Errorf("no stable release found for minor version %s", s.MinorVersion)
	}

	s.writeCache(best)

	return &Resolution{
		Version: best,
		Method:  MethodGitHubAPI,
		Source:  "GitHub API",
	}, nil
}

// InteractiveStrategy prompts the user for a version (stub for now).
type InteractiveStrategy struct {
	// Prompt function for testing; if nil, use stdin
	PromptFunc func(prompt string) (string, error)
}

func (s *InteractiveStrategy) Resolve() (*Resolution, error) {
	if s.PromptFunc != nil {
		input, err := s.PromptFunc("Enter Godot version (X.Y or X.Y.Z): ")
		if err != nil {
			return nil, fmt.Errorf("interactive prompt failed: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			return nil, nil
		}
		normalized, err := NormalizeVersion(input)
		if err != nil {
			return nil, fmt.Errorf("interactive version invalid: %w", err)
		}
		return &Resolution{Version: normalized, Method: MethodInteractive, Source: "interactive prompt"}, nil
	}

	if stat, err := os.Stdin.Stat(); err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
		return nil, nil
	}

	if _, err := fmt.Fprint(os.Stdout, "Enter Godot version (X.Y or X.Y.Z): "); err != nil {
		return nil, fmt.Errorf("interactive prompt write failed: %w", err)
	}
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("interactive prompt read failed: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	normalized, err := NormalizeVersion(input)
	if err != nil {
		return nil, fmt.Errorf("interactive version invalid: %w", err)
	}

	return &Resolution{Version: normalized, Method: MethodInteractive, Source: "interactive prompt"}, nil
}

type versionCacheEntry struct {
	Version   string    `json:"version"`
	FetchedAt time.Time `json:"fetched_at"`
}

func (s *GitHubAPIStrategy) cacheFilePath() string {
	base := s.CacheDir
	if base == "" {
		base = filepath.Join(os.TempDir(), "gst-cache")
	}
	return filepath.Join(base, fmt.Sprintf("godot-%s.json", strings.ReplaceAll(s.MinorVersion, ".", "_")))
}

func (s *GitHubAPIStrategy) readCache() string {
	path := s.cacheFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var entry versionCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ""
	}

	if entry.Version == "" {
		return ""
	}

	if time.Since(entry.FetchedAt) > 6*time.Hour {
		return ""
	}

	return entry.Version
}

func (s *GitHubAPIStrategy) writeCache(version string) {
	path := s.cacheFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}

	entry := versionCacheEntry{Version: version, FetchedAt: time.Now().UTC()}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0644)
}

func normalizeFromToolOutput(output string) (string, error) {
	re := regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?(?:\.[A-Za-z0-9._-]+)?)`)
	match := re.FindString(strings.TrimSpace(output))
	if match == "" {
		return "", fmt.Errorf("could not parse Godot version from output: %q", strings.TrimSpace(output))
	}

	return NormalizeVersion(match)
}

func pickLatestPatchForMinor(releases []struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}, minor string) string {
	candidates := []string{}
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		tag := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
		normalized, err := NormalizeVersion(tag)
		if err != nil {
			continue
		}

		versionMinor, err := ExtractMinor(normalized)
		if err != nil || versionMinor != minor {
			continue
		}

		candidates = append(candidates, normalized)
	}

	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool {
		return compareSemver(candidates[i], candidates[j]) > 0
	})

	return candidates[0]
}

func compareSemver(a, b string) int {
	parse := func(v string) [3]int {
		parts := strings.Split(v, ".")
		out := [3]int{}
		for i := 0; i < 3 && i < len(parts); i++ {
			if _, err := fmt.Sscanf(parts[i], "%d", &out[i]); err != nil {
				out[i] = 0
			}
		}
		return out
	}

	av := parse(a)
	bv := parse(b)
	for i := 0; i < 3; i++ {
		if av[i] > bv[i] {
			return 1
		}
		if av[i] < bv[i] {
			return -1
		}
	}
	return 0
}
