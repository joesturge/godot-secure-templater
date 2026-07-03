---
applyTo: ".github/workflows/**"
---

# CI / Workflows — Compact Rules

- Trigger releases only on annotated tag pushes that match `v*`.
- Artifact naming: `gst-<os>-<arch>[-debug].zip` or `.exe` inside the archive. Do not publish GOARCH=386 artifacts.
- Build matrix: parallelize `target=release` and `target=debug`, collect per-target logs, fail if either fails.
- Cache key: use `manifest.json` + platform + toolchain identity (MinGW/Python/SCons versions) as `actions/cache` key.
- CI must run `gst create --non-interactive --godot-version <x.y.z>` (prefer explicit version). Avoid relying on `latest` in non-interactive runs.
- Secrets: avoid printing raw keys; prefer file/stdin handoff for `SCRIPT_AES256_ENCRYPTION_KEY` where possible; mask secrets in logs.
- Exit codes: rely on `internal/errors.go` contract; workflows should branch on codes (0 success, 4 version-resolution, 5 integrity/checksum, 6 disk, 7 build failed, 8 config injection, 9 unsupported, 10 lock held).
- Lint & tests: workflows must run `go test ./...` and `golangci-lint run ./...` before producing artifacts.

Keep this file minimal — add detailed CI examples to `docs/` when more than one concrete example is needed.
