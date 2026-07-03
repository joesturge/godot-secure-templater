# Godot Secure Templater (gst) — Developer conventions (compact)

This file is a high-level index. Detailed, targeted rules live under `.github/instructions/` and
are applied by glob via `applyTo` to keep per-file-context small.

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
- TDD: all behavioural changes follow red → green → refactor; write a failing test first, make it pass with minimal code, then clean up.

If you need to change a specific rule, add or edit the corresponding file in `.github/instructions/`.
