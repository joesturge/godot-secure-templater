# Contributing to Godot Secure Templater

Thank you for contributing. This guide explains the current workflow, expectations, and quality bar for this repository.

## TDD-First Is Mandatory

For behavioural changes, use red -> green -> refactor every time.

1. Red: write or update a test that fails for the intended behaviour.
2. Green: make the smallest production change needed to pass.
3. Refactor: clean up only after tests are green.

Do not bundle unrelated refactors into the red -> green step.
If a change is difficult to test directly, add the nearest executable check rather than skipping test-first development.

## Current Scope

- Current supported build target: Windows templates.
- Linux target is not currently implemented.
- `gst create` compiles templates and prints manual setup guidance for Godot export configuration.

## Development Setup

Prerequisites:

- Go 1.21+
- Git

Build locally:

```bash
mkdir -p dist
go build -o dist/gst ./cmd/gst
./dist/gst --help
```

## Typical Change Workflow

1. Start with a failing test for behaviour changes.
2. Implement the minimum code change.
3. Run focused tests for touched packages.
4. Run full test suite before opening or updating a PR.

Recommended commands:

```bash
# focused packages (example)
go test ./cmd/gst ./internal/platform ./internal/platforms/windows

# full suite
go test ./...
```

## Testing Conventions

For `*_test.go` files:

- Use `github.com/stretchr/testify/assert` assertions.
- Use GIVEN/WHEN/THEN comments in tests and subtests.
- Use `t.TempDir()` for filesystem test isolation.
- Prefer table-driven tests for scenario coverage.

For full detail, follow:

- `.github/instructions/go-tests.instructions.md`

## Coding Conventions

- Keep changes slice-local and incremental.
- Preserve stable exit-code behaviour defined in `internal/errors.go`.
- Treat secrets and encryption key material as sensitive.
- Keep public behaviour and docs aligned when changing workflow.

Repository-wide instruction index:

- `.github/copilot-instructions.md`

## Documentation Updates

If your change modifies user-visible behaviour, update docs in the same PR.

Use these as references:

- `README.md` for user workflow and CLI usage
- `docs/design.md` for architecture decisions
- `docs/plan.md` for slice scope and roadmap

## Pull Request Checklist

Before requesting review:

1. Tests added or updated first for behavioural changes.
2. `go test ./...` passes locally.
3. Docs updated if behaviour changed.
4. Change scope is minimal and focused.
5. No secret material committed.

## Release Build Commands

Use these when generating binaries manually:

```bash
mkdir -p dist

# Windows (64-bit)
GOOS=windows GOARCH=amd64 go build -o dist/gst.exe ./cmd/gst

# Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -o dist/gst ./cmd/gst

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dist/gst ./cmd/gst

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dist/gst ./cmd/gst
```

These commands build release binaries for distribution; they do not imply runtime target support for all platforms in `gst create`.
