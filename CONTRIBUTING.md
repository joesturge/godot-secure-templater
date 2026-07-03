# Contributing to Godot Secure Templater

Thank you for your interest in contributing! This guide covers the architecture, conventions, and development workflow.

---

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Slices](#slices)
- [Code Conventions](#code-conventions)
- [Safety Patterns](#safety-patterns)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Adding a Platform](#adding-a-platform)

---

## Architecture Overview

### Project Structure

The codebase is organised around functional packages:

- **`cmd/gst/`** — CLI entry point (Cobra-based)
- **`internal/project/`** — Project detection and version validation
- **`internal/config/`** — Configuration injection and era routing
- **`internal/crypto/`** — Encryption key generation
- **`internal/manifest/`** — Build fingerprinting and caching
- **`internal/version/`** — Version resolution chain
- **`internal/pipeline/`** — Orchestrator coordinating all phases
- **`internal/cleanup/`** — Artifact pruning and workspace management
- **`internal/toolchain/`** — Toolchain provisioning (HTTP download, SHA-256 verification, pure-Go archive extraction)
- **`internal/builder/`** — Template compilation via SCons with sys.path injection for embedded Python
- **`internal/progress/`** — Progress reporting from build output
- **`internal/longpath/`** — Windows path-length validation
- **`.github/instructions/`** — Code and documentation conventions
- **`docs/`** — Design specifications and implementation guides

See the `cmd/` and `internal/` directories for the actual structure. The test files (`*_test.go`) live alongside their implementation.

### Core Workflow

The `create` command follows these phases:

1. **Validation** — Detect Godot project, validate version, initialise workspace
2. **Orchestration** — Pipeline coordinates all subsequent phases
3. **Preflight** — Check paths, resolve version, determine era
4. **Caching** — Check if build is idempotent (skip if inputs unchanged)
5. **Provisioning** — Download and verify toolchain
6. **Compilation** — Compile templates with encryption key
7. **Configuration** — Inject key into export settings
8. **Recording** — Write manifest for future idempotency
9. **Cleanup** — Remove toolchain (optionally), preserve templates and keys
10. **Reporting** — Display per-machine key warning and next steps

---

## Slices

### Slice 0 (MVP / Walking Skeleton)
**Status:** ✅ Complete & Stable

**What's Done:**
- Project detection and version validation
- Encryption key generation (AES-256, `crypto/rand`)
- Workspace initialisation
- Configuration injection (byte-preserving, backup-once, atomic writes)
- Toolchain provisioning (HTTP download, SHA-256 verification, archive extraction)
- **Template compilation via SCons** (real compilation, not stubs)
- CLI and typed error system (exit codes 0-10, stable)

**Implementation details:**
- Pure-Go archive extraction (ZIP, tar.gz, 7z with LZMA2 support)
- Multicore SCons compilation (auto-detects CPU cores)
- sys.path injection workaround for embedded Python compatibility
- d3d12=no flag to compile without Windows SDK dependency

**Test coverage:** ~50 tests across all packages

### Slice 1 (Robust Single-Platform Tool)
**Status:** ✅ Complete & Tested

**What's Done:**
1. **Manifest System** — Build fingerprint caching and idempotency detection
2. **Version Resolution Chain** — Pluggable strategy system with fallback options
3. **Cleanup & Disk Footprint** — Smart pruning; preserve critical files
4. **Progress Reporting** — Staged output parsing from build tools
5. **Multi-Era Configuration** — Version-to-era routing with fail-closed semantics
6. **Long-Path Validation** — Windows MAX_PATH detection and guidance
7. **Key Regeneration Guardrails** — Confirmation and audit trail
8. **Team Messaging** — Per-machine key warnings and guidance
9. **Pipeline Orchestrator** — Coordinates all phases end-to-end

**Overall test coverage:** ~85% across all packages

### Slice 2 (CI/Automation)
**Status:** 🔧 Planned

**What's Needed:**
- `--non-interactive` flag
- `--json` output format
- Secret-safe logging (key masking)
- CI environment detection

### Slice 3+ (Multi-Platform)
**Status:** 🔧 Planned

**What's Needed:**
- Plugin registry
- `Provisioner` interface extraction
- `EnvironmentBuilder` interface
- Platform-specific implementations (Linux, Web, macOS/iOS, Android)

---

## Code Conventions

### Package Names
- Lowercase, single word (e.g. `crypto`, `config`, `manifest`, `version`, `pipeline`)

### Naming
- **Exported:** `PascalCase` (functions, types, constants that are public)
- **Unexported:** `camelCase` (internal functions and variables)

### Imports
Grouped in this order:
1. Standard library
2. Blank line
3. Third-party (`github.com/...`)

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/joemi/godot-secure-templater/internal"
)
```

### Comments
- **Package:** `// Package <name> <description>` at top
- **Exported functions:** `// <FunctionName> <what it does>.` above signature
- **Unexported:** Follow same pattern
- **Complex logic:** Explain *why*, not *what* (code shows what)

---

## Safety Patterns

### Crypto: AES-256 with crypto/rand
```go
// Always use crypto/rand, never math/rand
key, err := crypto.GenerateAES256Key() // Returns 32 bytes (64-char hex)
// File permissions: 0600 (owner-read-only)
os.WriteFile(keyPath, keyBytes, 0600)
```

### Key Material Handling
- **Generated with:** `crypto/rand`
- **Stored with:** Owner-only permissions (`0600`)
- **Atomicity:** Temp file + `os.Rename` (no partial writes)
- **.gitignore:** Automatic (add `.gst/` to project)
- **No transmission:** Keys stay on machine

### Config Editing: Byte-Preserving

**Principle:** Never parse-modify-serialize (loses formatting). Always use targeted line edits.

**Pattern:** Find the relevant line with regex, replace in-place, write back. This preserves user formatting and comments.

### Backup-Once Semantics

**Principle:** Create a pristine backup only once; never overwrite it.

**Pattern:** First write creates `.bak`. Subsequent edits modify the working file only. On rollback, restore from `.bak` and delete it.

### Atomic Writes (Temp + Rename)

**Principle:** Critical files (keys, manifests) must never be partially written.

**Pattern:** Write to a temporary file, then atomically rename it to the final path. This ensures all-or-nothing semantics.

### Error Handling: Typed Errors

**Principle:** All errors carry an exit code (0–10, stable contract for CI).

**Pattern:** Define custom error types in `internal/errors.go` with factory functions. This ensures consistent error reporting across the codebase.

### Version Routing: Fail-Closed

**Principle:** Never guess. Unknown versions must be errors, never silent best-effort.

**Pattern:** Explicitly handle recognised versions (e.g. 4.3+, 4.1–4.2). Reject anything else with a clear error message.

### Manifest: Nil-Tolerant Reads

**Principle:** Missing or corrupted manifests indicate a first run, not an error state.

**Pattern:** `Read()` returns `nil` if the manifest is missing or invalid. Calling code treats this as a graceful "no prior build" condition.

---

## Development Workflow

### Build

```bash
mkdir -p dist
go build -o dist/gst ./cmd/gst
./dist/gst --help
```

### Test

```bash
# All tests
go test ./...

# With coverage
go test ./... -cover

# With verbose output
go test ./... -v

# Specific package
go test ./internal/config -v

# With race detector
go test -race ./...
```

### Format & Lint

```bash
go fmt ./...
golangci-lint run ./...
```

### Before Committing

```bash
go fmt ./...
go build -o dist/gst ./cmd/gst
go test ./...
```

---

## Testing

### Test Framework
- **Assertion Library:** `github.com/stretchr/testify/assert` (MANDATORY)
- **BDD Comments:** MANDATORY on every test (Gherkin-like)
- **Isolation:** `t.TempDir()` for filesystem operations
- **Table-driven tests:** For parametric scenarios

### BDD Comment Pattern
```go
func TestSomething(t *testing.T) {
	// GIVEN: initial condition
	// WHEN: action is taken
	// THEN: expected outcome
	// AND: additional assertions (optional)

	// GIVEN a version string "4.3.1"
	version := "4.3.1"

	// WHEN we normalize it
	normalized, err := version.Normalize(version)

	// THEN it should remain unchanged
	assert.NoError(t, err)
	assert.Equal(t, "4.3.1", normalized)

	// AND an error-typed result should use assert.Nil, not assert.NoError
	if err != nil {
		assert.Nil(t, err) // For custom *Error types
	}
}
```

### Assertion Guidelines
- **Standard Go errors:** `assert.NoError(t, err)` or `assert.Error(t, err)`
- **Custom `*Error` types:** `assert.Nil(t, err)` or `assert.NotNil(t, err)`
- **Always include descriptive messages:** `assert.Equal(t, expected, actual, "should match template pattern")`
- **Never manual `if` checks:** Testify does comparisons better

### Coverage Targets
- **Per-package:** 85-95%
- **Overall:** ~85%
- **Critical paths (crypto, config):** 95%+

### Table-Driven Test Example
```go
func TestVersionToEra(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    Era
		wantErr bool
	}{
		{"4.3.0", "4.3.0", Era43Plus, false},
		{"4.2.0", "4.2.0", Era41To42, false},
		{"4.0.0", "4.0.0", EraUnknown, true},
		{"invalid", "invalid", EraUnknown, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// GIVEN a version
			// WHEN we determine its era
			got, err := VersionToEra(tc.version)
			// THEN era should match
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
```

---

## Adding a Platform

To add a new platform (e.g. Linux, Web):

1. **Define constants** in manifest (platform identifier)
2. **Create provisioner** for the platform (download toolchain, verify checksums)
3. **Extend era routing** (version → configuration path mapping)
4. **Update platform detection** in CLI (fail-closed for unknown platforms)
5. **Add tests** for provisioner and routing logic

Each step should follow the established patterns: use `t.TempDir()` for tests, validate inputs, preserve existing files, and provide clear error messages.

---

## Dependencies (Fixed & Stable)

- **Go:** 1.21+
- **cobra:** v1.10.2 (CLI framework)
- **golang.org/x/crypto:** v0.53.0 (cryptographic operations)
- **testify:** v1.11.1 (assertions only; never testify/mock)

**Never upgrade without consensus.** Dependencies are pinned in `go.mod`.

---

## Troubleshooting Development Issues

### Test Fails with "use assert.Nil for custom *Error"
You're using `assert.NoError()` on a custom `*Error` type. Use `assert.Nil(t, err)` instead for custom error types.

### Import cycles
Packages should not import each other. If you need shared types, put them in `internal/types.go` or create a shared package.

### Unused imports
Run `go fmt ./...` to clean up automatically.

### Subtests for clarity
Use `t.Run()` to organise related test cases and improve output readability.

---

## Questions?

Open an issue on GitHub or check [docs/design.md](docs/design.md) for the original specification.

Happy coding! 🚀
