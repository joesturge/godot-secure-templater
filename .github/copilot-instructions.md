# Godot Secure Templater (gst) — Copilot instructions

## Priority order
- Behaviour changes: use red -> green -> refactor.
- Update docs as you go when user-visible behaviour changes.
- Follow `docs/design.md` and `docs/plan.md`; if a change affects either one, ask first before editing them, then update them in the same change.

This file is a high-level index. Detailed, targeted rules live under `.github/instructions/` and are applied by glob via `applyTo` to keep per-file context small.

- Tests: see `.github/instructions/go-tests.instructions.md`
- Docs: see `.github/instructions/docs.instructions.md`
- CI / Workflows: see `.github/instructions/ci.instructions.md`

Key repo conventions (short):
- Follow Slice-based plan in `docs/plan.md` (keep changes slice-local and incremental).
- Stable exit codes: defined in `internal/errors.go` and relied on by CI workflows.
- Release artifacts: tag-triggered (`v*`) GitHub Actions only; artifact names follow `gst-<os>-<arch>[-debug].zip`.
- No unsafe INI round-trips: `internal/config` must perform byte-preserving, targeted edits only.
- Crypto: AES-256 keys via `crypto/rand`, owner-only perms, atomic writes; never print raw keys in logs.
- Toolchain: pinned dependencies in code; manifest-based caching (`manifest.json`) is the CI cache key.
- Self-contained builds: verify and compile paths must not assume host compilers are preinstalled. On Windows host paths, SCons must use provisioned Zig compiler environment (`CC=zig cc`, `CXX=zig c++`, `AR=zig ar`) and must not rely on `MINGW_PREFIX` being present on the host.
- Embedded Python/SCons bootstrapping: when installing SCons into the embedded runtime, bootstrap `setuptools` first (often via `ensurepip` or `python -m pip install --upgrade setuptools`) before `setup.py install`; do not assume the bundled Python includes packaging tools.
- Emscripten/PYTHONPATH: when SCons is launched via a packaged `__main__.py` bootstrap, preserve inherited `PYTHONPATH` entries and split them with `os.pathsep`; otherwise `emcc`/`em++.py` may fail to import sibling Emscripten modules even though the SDK is present.
- Windows runtime paths: use the provisioned runtime paths under `.gst/runtime`, not host-installed toolchains; normalize long Windows paths only where needed for the runtime toolchain, and always keep the bundled Python and SDK on PATH/PYTHONPATH.
- Validation order: for root-cause bugs in toolchains or build scripts, reproduce with the smallest package-scoped test command first, then run the relevant `golangci-lint` command before broad suite expansion.

Common gotchas / anti-patterns:
- Do not assume the embedded Python is full-featured; some distributions ship without `setuptools`, pip, or site support enabled.
- Do not assume the host environment is the build environment; use runtime-provisioned compiler and SDK paths consistently.
- Do not drop inherited environment variables when wrapping Python/SCons entrypoints; static `sys.path` injection without `PYTHONPATH` preservation is a common cause of Emscripten import failures.
- Do not add broad refactors while fixing a toolchain regression; keep the change slice-local and verify with a failing regression first.

If you need to change a specific rule, add or edit the corresponding file in `.github/instructions/`.
