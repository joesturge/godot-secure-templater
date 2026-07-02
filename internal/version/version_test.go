package version

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeVersion(t *testing.T) {
	// GIVEN various version strings
	tests := []struct {
		name    string
		input   string
		wantVer string
		wantErr bool
	}{
		{
			name:    "simple semantic version",
			input:   "4.3.0",
			wantVer: "4.3.0",
			wantErr: false,
		},
		{
			name:    "version with build metadata",
			input:   "4.3.1.stable.official",
			wantVer: "4.3.1",
			wantErr: false,
		},
		{
			name:    "version with long metadata",
			input:   "4.4.0.dev1.g123abc",
			wantVer: "4.4.0",
			wantErr: false,
		},
		{
			name:    "invalid format - missing patch",
			input:   "4.3",
			wantVer: "",
			wantErr: true,
		},
		{
			name:    "invalid format - non-numeric",
			input:   "latest",
			wantVer: "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantVer: "",
			wantErr: true,
		},
		{
			name:    "with v prefix",
			input:   "v4.3.0",
			wantVer: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN normalizing the version
			got, err := NormalizeVersion(tt.input)

			// THEN the result should match expectations
			if tt.wantErr {
				assert.NotNil(t, err, "NormalizeVersion should error")
				assert.Empty(t, got)
			} else {
				assert.Nil(t, err, "NormalizeVersion should not error")
				assert.Equal(t, tt.wantVer, got)
			}
		})
	}
}

func TestExtractMinor(t *testing.T) {
	// GIVEN various version strings
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "simple semantic version",
			input:   "4.3.0",
			want:    "4.3",
			wantErr: false,
		},
		{
			name:    "patch 5",
			input:   "4.3.5",
			want:    "4.3",
			wantErr: false,
		},
		{
			name:    "missing patch",
			input:   "4.3",
			want:    "4.3",
			wantErr: false,
		},
		{
			name:    "missing minor",
			input:   "4",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN extracting the minor version
			got, err := ExtractMinor(tt.input)

			// THEN the result should match expectations
			if tt.wantErr {
				assert.NotNil(t, err, "ExtractMinor should error")
			} else {
				assert.Nil(t, err, "ExtractMinor should not error")
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// MockStrategy is a test double for resolution strategies.
type MockStrategy struct {
	WillResolve *Resolution
	WillError   error
	WillDecline bool
}

func (m *MockStrategy) Resolve() (*Resolution, error) {
	if m.WillError != nil {
		return nil, m.WillError
	}
	if m.WillDecline {
		return nil, nil
	}
	return m.WillResolve, nil
}

func TestResolverFirstStrategyWins(t *testing.T) {
	// GIVEN multiple strategies
	strategy1 := &MockStrategy{
		WillResolve: &Resolution{
			Version: "4.3.0",
			Method:  MethodExplicit,
			Source:  "strategy1",
		},
	}
	strategy2 := &MockStrategy{
		WillResolve: &Resolution{
			Version: "4.3.1",
			Method:  MethodLocalEditor,
			Source:  "strategy2",
		},
	}

	// WHEN resolving
	resolver := NewResolver(strategy1, strategy2)
	got, err := resolver.Resolve()

	// THEN the first successful strategy should be used
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "4.3.0", got.Version)
	assert.Equal(t, "strategy1", got.Source)
}

func TestResolverDeclineToNext(t *testing.T) {
	// GIVEN strategies where first declines, second succeeds
	strategy1 := &MockStrategy{
		WillDecline: true,
	}
	strategy2 := &MockStrategy{
		WillResolve: &Resolution{
			Version: "4.3.1",
			Method:  MethodLocalEditor,
			Source:  "local-editor",
		},
	}

	// WHEN resolving
	resolver := NewResolver(strategy1, strategy2)
	got, err := resolver.Resolve()

	// THEN it should use the second strategy
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "4.3.1", got.Version)
}

func TestResolverErrorShortCircuits(t *testing.T) {
	// GIVEN a strategy that errors
	strategy1 := &MockStrategy{
		WillError: fmt.Errorf("strategy error"),
	}
	strategy2 := &MockStrategy{
		WillResolve: &Resolution{
			Version: "4.3.1",
			Method:  MethodLocalEditor,
		},
	}

	// WHEN resolving
	resolver := NewResolver(strategy1, strategy2)
	got, err := resolver.Resolve()

	// THEN the error should be returned immediately
	assert.NotNil(t, err)
	assert.Nil(t, got)
	assert.Equal(t, "strategy error", err.Error())
}

func TestResolverAllDecline(t *testing.T) {
	// GIVEN strategies that all decline
	strategy1 := &MockStrategy{WillDecline: true}
	strategy2 := &MockStrategy{WillDecline: true}

	// WHEN resolving
	resolver := NewResolver(strategy1, strategy2)
	got, err := resolver.Resolve()

	// THEN an error should be returned
	assert.NotNil(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "no version resolution strategy succeeded")
}

func TestResolverEmpty(t *testing.T) {
	// GIVEN no strategies
	// WHEN resolving
	resolver := NewResolver()
	got, err := resolver.Resolve()

	// THEN an error should be returned
	assert.NotNil(t, err)
	assert.Nil(t, got)
}

func TestExplicitStrategySuccess(t *testing.T) {
	// GIVEN an explicit version
	strategy := &ExplicitStrategy{Version: "4.3.0"}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN it should return the version
	assert.Nil(t, err, "ExplicitStrategy should not error")
	assert.NotNil(t, got)
	assert.Equal(t, "4.3.0", got.Version)
	assert.Equal(t, MethodExplicit, got.Method)
}

func TestExplicitStrategyWithMetadata(t *testing.T) {
	// GIVEN an explicit version with metadata
	strategy := &ExplicitStrategy{Version: "4.3.1.stable.official"}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN it should normalize and return
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "4.3.1", got.Version, "should normalize version")
}

func TestExplicitStrategyEmpty(t *testing.T) {
	// GIVEN an empty explicit strategy
	strategy := &ExplicitStrategy{Version: ""}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN it should decline
	assert.Nil(t, err)
	assert.Nil(t, got, "empty explicit strategy should decline")
}

func TestExplicitStrategyInvalid(t *testing.T) {
	// GIVEN an invalid explicit version
	strategy := &ExplicitStrategy{Version: "latest"}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN an error should be returned
	assert.NotNil(t, err, "should error on invalid version")
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "explicit version invalid")
}

func TestLocalEditorStrategyStub(t *testing.T) {
	// GIVEN a local editor strategy (stub in Slice 0 / early Slice 1)
	strategy := &LocalEditorStrategy{EditorPath: ""}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN it should decline (stub)
	assert.Nil(t, err)
	assert.Nil(t, got, "stub local editor strategy should decline")
}

func TestGitHubAPIStrategyStub(t *testing.T) {
	// GIVEN a GitHub API strategy (stub in Slice 0 / early Slice 1)
	strategy := &GitHubAPIStrategy{MinorVersion: "4.3"}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN it should decline (stub)
	assert.Nil(t, err)
	assert.Nil(t, got, "stub GitHub API strategy should decline")
}

func TestInteractiveStrategyStub(t *testing.T) {
	// GIVEN an interactive strategy (stub in Slice 0 / early Slice 1)
	strategy := &InteractiveStrategy{}

	// WHEN resolving
	got, err := strategy.Resolve()

	// THEN it should decline (stub)
	assert.Nil(t, err)
	assert.Nil(t, got, "stub interactive strategy should decline")
}

func TestResolverIntegration(t *testing.T) {
	// GIVEN a resolver with strategies in priority order
	resolver := NewResolver(
		&ExplicitStrategy{Version: "4.3.0"},
		&LocalEditorStrategy{},
		&GitHubAPIStrategy{MinorVersion: "4.3"},
	)

	// WHEN resolving
	got, err := resolver.Resolve()

	// THEN it should use the explicit strategy
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "4.3.0", got.Version)
	assert.Equal(t, MethodExplicit, got.Method)
}

func TestResolverFallback(t *testing.T) {
	// GIVEN a resolver where explicit is empty (declines)
	resolver := NewResolver(
		&ExplicitStrategy{Version: ""},
		&MockStrategy{
			WillResolve: &Resolution{
				Version: "4.4.0",
				Method:  MethodGitHubAPI,
				Source:  "GitHub releases",
			},
		},
	)

	// WHEN resolving
	got, err := resolver.Resolve()

	// THEN it should fall back to the second strategy
	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "4.4.0", got.Version)
	assert.Equal(t, MethodGitHubAPI, got.Method)
}
