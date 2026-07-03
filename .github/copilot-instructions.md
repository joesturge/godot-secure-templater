# Godot Secure Templater (gst)

CLI tool for Godot 4.3+ template compilation with secure AES-256 encryption. ~85% test coverage.

## Tests
- Follow TDD when it is possible. Always start new changes by writing new test cases (or changing existing tests). Remeber to consult `.github/instructions/go-tests.instructions.md` when creating tests.
- Use `github.com/stretchr/testify/assert` only (no mocks)
- Custom `*Error`: `assert.Nil(t, err)` | Standard: `assert.NoError(t, err)`
- **MANDATORY**: GIVEN/WHEN/THEN comments on every test
- Use `t.TempDir()` for filesystem tests
- Consolidate in single `*_test.go` per package, not split files
- All assertion messages descriptive, table-driven for scenarios

## Errors
- Exit codes 0-10 defined in `internal/errors.go` only
- Factory functions: `ErrMinorMismatch(projectMinor, supplied)`
- Struct: `Code`, `Message` (brief), `Details` (full context)

## Code Style
- Packages: lowercase single-word (`errors`, `crypto`, `config`, `project`)
- Exported: PascalCase | Unexported: camelCase
- Imports: stdlib, blank line, 3rd-party

## Crypto & Config
- AES-256: `crypto/rand`, 32 bytes, key files 0600 perms, atomic writes
- Config: targeted line edits (no parse/serialize), backup-once, atomic writes

## Toolchain (internal/toolchain/)
- **Python 3.11**: embed ZIP, no system Python needed
- **MinGW 14.2.0**: x86_64 posix seh ucrt
- **SCons 4.4.0**: egg-info format with `scons/scons/__main__.py`
- **Godot Source**: version-tagged tar.gz from GitHub
- Extraction: Pure-Go (ZIP, tar.gz with strip, 7z/LZMA2 via bodgit/sevenzip)
- Checksums: SHA-256 with placeholder fallback for unknown versions

## SCons Compilation (internal/builder/)
- **Discovery** (priority): scons.py → scons/__main__.py → SCons/__main__.py → bin/scons → python -m SCons
- **Embedded Python workaround**: Inject sys.path via `python -c "import sys; sys.path.insert(0, '/path'); exec(open(...).read())"` for __main__.py
- **Env**: PATH (python/mingw/scons), PYTHONPATH (scons only), SCRIPT_AES256_ENCRYPTION_KEY, SystemRoot
- **Build**: `python scons platform=windows target=release|debug dev_build=no optimize=speed d3d12=no`
- **Output**: `godot.windows.template_release.x86_64.exe` → `.gst/templates/windows_template_release.exe`

## Dependencies (Pinned)
Go 1.21+, cobra v1.10.2, golang.org/x/crypto v0.53.0, testify v1.11.1, bodgit/sevenzip v1.6.4
