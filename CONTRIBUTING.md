# Contributing to Godot Secure Templater

This guide covers the current workflow, expectations, and quality bar for this repository.

## Current Scope

- Supported build target: Windows templates on Windows hosts.
- Linux and Web targets are not yet implemented here. Planned additions stay within upstream Godot host/toolchain constraints.
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

Use the release-build commands below when you need explicit GOOS/GOARCH output names.

## Typical Change Workflow

1. Start with a failing test for behaviour changes.
2. Implement the minimum code change.
3. Run focused tests for touched packages.
4. Run full test suite before opening or updating a PR.

Recommended commands:

```bash
# focused packages (example)
go test ./cmd/gst ./internal/platforms/hostwindows ./internal/toolchain

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

Use these when generating binaries manually. The README points here for the canonical source-build commands.

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
