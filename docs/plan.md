# Godot Template Tool — Sliced Architecture & Delivery Plan

**Product:** `Godot Secure Templater (gst)`
**What it is:** a standalone Go CLI that automates provisioning, compilation, and configuration of
secure (encrypted) Godot export templates natively inside a user's project directory — so a
developer never has to hand-install a C++ toolchain, Python, SCons, or the Godot source tree.

**How to read this document.** It is both an architecture spec and a delivery plan. Work ships in
**incremental slices**: Slice 0 is the smallest end-to-end MVP that proves the core value loop;
each later slice hardens or extends it *without rewriting* what came before. Design risks are not
quarantined in an appendix — each one is folded into the slice that resolves it. §9 is a compact
traceability table mapping every known concern to where it is handled.

---

## 1. The Core Value Loop

The one thing this tool exists to do:

> \*\*Compile encrypted Godot export templates from source and wire them into the project's export
> configuration, without the developer manually installing a toolchain or the Godot source.\*\*

Everything else — version inference, caching, CI mode, the multi-platform plugin framework — is
convenience or generalization *around* this loop. Slice 0 proves the loop end-to-end; later slices
harden and extend it.

---

## 2. Design Principles

These hold across every slice and are the reason the plan can start tiny yet grow without rewrites.

1. **MVP-first, but never a throwaway.** Slice 0 is deliberately minimal, but its structure is the
   real structure. Later slices *add* to it; they do not replace it.
2. **Extensibility via seams, not speculative frameworks.** We do **not** build the plugin/registry
  machinery up front for a single platform. Instead, Slice 0 places its platform-specific work
  (toolchain provisioning, build invocation, config writing) behind \*\*narrow, single-purpose
  internal functions whose signatures already match the future interfaces\*\*. Slice 2 then extracts
  those interfaces — a mechanical refactor, not a redesign. The seams are named explicitly in §4.6.
3. **Fail closed, never guess.** Any unrecognized Godot version, config schema, host/target
   combination, or integrity-check failure is an explicit, specific error — never a silent
   best-effort that could corrupt a user's project or ship a broken build.
4. **Treat two things as sacred:** the user's **key material** and the user's \*\*existing project
   config\*\*. Both get defensive handling (atomic writes, backup-once, restrictive permissions,
   ordered/rollback-safe mutations) starting in Slice 0, because these are expensive to retrofit and
   catastrophic to get wrong.
5. **Deterministic and auditable.** Every consequential decision (which version, which method, which
   toolchain checksums) is echoed to the user and — from Slice 1 — recorded in a manifest, so runs
   are reproducible.
6. **Verifiable by construction.** All nondeterministic dependencies (clock, network, filesystem
  root) are injected, so every stage is testable without a 40-minute compile or live network. Each
  slice ships with contract tests; the highest-risk subsystem (config editing) is guarded by
  byte-preservation golden tests, and the Slice 2 plugin extraction is guarded by a \*\*cross-slice
  contract test\*\* — the extracted `windows` plugin must pass the *same* suite the concrete Slice 0/1
  code passed, which is what makes "extraction, not rewrite" a checkable claim rather than a hope.

> **Companion document.** This is the delivery plan (*what ships when, and why*). The full
> engineering detail — data structures, interfaces, algorithms, error taxonomy, threat model, and
> test plan — lives in `gst-design.md`.

---

## 3. Slice Map

| Slice | Theme | Delivers | Deliberately deferred |
| --- | --- | --- | --- |
| **0** | Walking skeleton (MVP) | One hard-coded happy path: Windows→Windows, Godot 4.3+, explicit version, compile + wire-in, safe config/key handling | Version inference, manifest/caching, CI mode, runtime pruning, long-path prefixing, multi-era config, plugin framework |
| **1** | Robust single-platform tool | Version resolution chain, manifest + idempotency, staged progress, cleanup, multi-era config, long-path handling, key-regen guardrails | CI/non-interactive, multi-platform |
| **2** | Multi-platform extensibility | Extract plugin registry + `Provisioner`/`EnvironmentBuilder` interfaces + host/target matrix, driven by a real 2nd target (Linux) | Web/Android/macOS/iOS specifics |
| **3** | CI / automation | `--non-interactive`/`--yes`, `--json`, manifest-keyed caching, secret-safe logging, parallel release+debug compilation | — |
| **4+** | Breadth & hardening | Concrete Web/Android/macOS/iOS plugins, signature verification, documented CI workflow, `list-platforms` | — |

---

## 4. Slice 0 — Walking Skeleton (MVP)

**Goal:** a single hard-coded happy path, end to end, on a developer's Windows machine → Windows
Desktop target, Godot **≥ 4.3** only. Prove the core loop (§1) and establish the safety and
extensibility structure the rest of the plan builds on.

### 4.1 Scope constraints

* **Host:** Windows. **Target:** Windows Desktop. **Godot:** 4.3+ only.
* **Isolation:** per-project toolchain under `.gst/`. No system env vars or registry
  modified.
* **Role:** configuration + compilation prep. The tool builds custom binaries and injects
  paths/keys; the Godot Editor (or, later, a CI step) performs the final `.pck` packaging/export.

### 4.2 Directory layout

```javascript
MyGame/
├── project.godot
├── export_presets.cfg
├── export_presets.cfg.bak          <-- Backup of the ORIGINAL presets (created once; §4.5)
├── .godot/
│   ├── export_credentials.cfg       <-- Godot 4.3+ target for the encryption key (§4.5)
│   └── export_credentials.cfg.bak
├── .gst/            <-- Isolated CLI workspace (lockfile-guarded, §4.3)
│   ├── runtime/                     <-- Toolchain + Godot source (kept in Slice 0; pruning is Slice 1)
│   │   ├── python/  mingw/  scons/  godot_source/
│   ├── templates/
│   │   ├── windows_release.exe
│   │   └── windows_debug.exe
│   ├── logs/
│   ├── .lock                        <-- Run lock (§4.3)
│   └── encryption.key               <-- AES-256 key, owner-only perms (§4.4)
└── .gitignore                       <-- CLI appends `.gst/`
```

### 4.3 Command, context detection, and run safety

* Entry point: `gst create --godot-version=X.Y.Z`. In Slice 0 `--godot-version` is
  **required** — all inference is deferred to Slice 1, which keeps the MVP deterministic and free of
  network/editor-probe surface.
* Verify the directory is a Godot project by locating `project.godot`; read the minor line from
  `config/features`. **Fail fast** if the supplied version's minor line ≠ the project's declared
  minor line (minor mismatch is where Godot's compatibility guarantees end; patch differences are
  tolerated and expected to work with custom templates).
* **Concurrency + interruption safety (from day one):** acquire an exclusive lockfile in
  `.gst/.lock` for the run's duration, so two invocations can't race on `runtime/`,
  the key, or config. Combined with atomic writes (§4.5), an interrupted run never leaves corrupt
  state.
* Generate the workspace and append `.gst/` to the project's root `.gitignore` if
  absent (tools and keys must never be committed).

### 4.4 Toolchain provisioning & key generation

* **Disk-space preflight** before any download: a source checkout + toolchain is multiple GB and the
  compile runs 20–60+ min; check for sufficient free space and fail early with an actionable message
  rather than dying at 90%.
* Download portable Python, MinGW-w64, SCons, and the Godot source tarball for the requested version
  into `runtime/`. No system state touched.
* **Integrity verification (non-negotiable, in the MVP):** every artifact is checked against a
  SHA-256 checksum **pinned in the CLI's own release**, not fetched at runtime from the same channel
  as the artifact. A mismatch aborts with a clear error. This tool handles key material downstream,
  so supply-chain integrity is a hard requirement.
* **Key generation:** if `encryption.key` is absent, generate a 256-bit AES key with `crypto/rand`
  and write it **with owner-only file permissions**; document that `.gitignore` is not a secrecy
  mechanism. If present, **reuse** it (reuse is always safe; regeneration is the dangerous op and is
  deferred to Slice 1 behind explicit confirmation).

### 4.5 Compilation & manual setup guidance

* **Compile:** construct an isolated environment mapping execution paths to `runtime/` binaries and
  invoke headless SCons for `platform=windows`, producing release + debug templates in `templates/`.
  Slice 0 streams raw SCons output to `stdout` + `logs/` (staged parsing is Slice 1).
* **Key handoff & exposure:** the AES key is passed to the build via Godot's conventional
  `SCRIPT_AES256_ENCRYPTION_KEY` env var. This surface (visible in process listings; frequently
  captured by CI logs) is acknowledged now and mitigated when it matters most in Slice 3 (log
  masking; prefer a file/stdin handoff where feasible).
* **Manual setup guidance — no default config mutation:** after a verified successful compile,
  print the template paths and the local `.gst/encryption.key` path so the user can wire the
  preset in the Godot Editor themselves. The tool never prints the raw key value.

* **`ConfigFile`, not INI — manual setup avoids the parser in the default flow.** `export_presets.cfg` /
  `export_credentials.cfg` are Godot `ConfigFile` format with typed literals
  (`PackedStringArray(...)`, `Dictionary(...)`, resource refs, nested quoting). The default runtime
  flow now prints the values the user needs and leaves the editor files untouched. A full parser is
  still a future option if automatic setup returns as an explicit opt-in.
* **Version-era gate:** Slice 0 supports **only** Godot 4.3+. Any other version is an explicit
  "not yet supported" error — never a guess.

### 4.6 Module layout and the extensibility seams

Flat by design — no registry or interfaces yet — but the platform-specific work is already isolated
behind functions whose shapes match the Slice 2 interfaces, so extraction is mechanical.

| Package | Slice 0 responsibility | Future-proofing seam |
| --- | --- | --- |
| `cmd/` | Cobra CLI: `create`, flags. | Already routes on a `platform` value (hard-wired `"windows"`); Slice 2 makes it a registry lookup. |
| `internal/project` | Locate/parse `project.godot`, validate version, folder + `.gitignore` setup, lockfile. | OS-agnostic already; version handling is a single function Slice 1 grows into the resolver chain. |
| `internal/toolchain` | Download + SHA-256-verify + extract Windows toolchain & Godot source. | Concrete `provisionWindows(...)` **matches the future `Provisioner` interface signature** — Slice 2 extracts the interface, this becomes the `windows` plugin's impl. |
| `internal/crypto` | AES-256 gen/reuse, secure storage. | Fully platform-agnostic; unchanged by all later slices. |
| `internal/builder` | Build isolated env, invoke SCons `platform=windows`, stream output. | Concrete `buildWindows(...)` **matches the future `EnvironmentBuilder` interface** — extracted in Slice 2. |
| `internal/config` | Targeted `ConfigFile` edits, atomic writes, backup-once, rollback, `.gitignore`. | Key-writing goes through a **single `writeCredential(version, key)` function** — Slice 1 turns it into a version-range `CredentialWriter` table without touching callers. |

**Slice 0 acceptance:** on a clean Windows + Godot 4.3+ project,
`create --godot-version=4.3.x` produces working encrypted templates, wires them in safely, and the
developer can export from the Godot Editor and run the encrypted build. An interrupted or repeated
run never corrupts config or the key.

---

## 5. Slice 1 — Robust Single-Platform Tool

Still Windows→Windows, still no plugin framework. Adds the reliability + UX layer Slice 0 dropped —
all through the seams already in place.

* **Version resolution chain.** Priority: (1) explicit `--godot-version`; (2) local editor via
  `PATH` / common install / `--godot-editor-path`, using `godot --version` as ground truth (parse
  build-metadata suffixes like `4.4.1.stable.official` carefully); (3) latest published patch via the
  Godot GitHub releases API; (4) interactive prompt as last resort. Resolved version + method are
  echoed and recorded.
    - **Determinism guard (addresses the non-deterministic-fallback risk):** the "latest patch"
    fallback is time-dependent and hits an unauthenticated, rate-limited API (60/hr, shared CI IPs)
    that fails offline. Support an authenticated token and cache release metadata; treat API failure
    as a clear, specific error. In non-interactive contexts (Slice 3) prefer *requiring* an explicit
    version over silently taking latest.
* **Build manifest (`manifest.json`).** Records resolved version + method, **toolchain checksums**,
  target platform (`"windows"` from day one, so it generalizes), timestamp, config, success/failure.
* **Idempotency — key on the *full* build inputs.** Skip recompilation only when the manifest shows a
  prior success matching Godot version **and toolchain/tool identity** (MinGW/SCons/Python + tool
  version), not just the Godot version — otherwise a toolchain bump silently reuses a stale artifact.
  `--force-rebuild` overrides. Re-provision `runtime/` if it was pruned.
* **Cleanup & disk footprint.** Prune `runtime/` by default on success; `--keep-runtime` to skip;
  `gst clean` on demand. Never touch `templates/`, `encryption.key`, or
  `manifest.json`.
* **Staged progress.** Parse SCons module boundaries into staged updates (`Compiling core…`,
  `Compiling drivers…`, `Linking…`) + elapsed-time counter, replacing raw output.
* **Key-regeneration guardrails.** `--regenerate-key` requires interactive confirmation (or `--force`
  in automation), because regenerating invalidates any build that embedded the old key.
* **Multi-era config routing.** Grow `writeCredential` into a `CredentialWriter` table keyed by
  version range: 4.3+ → `export_credentials.cfg`; 4.1–4.2 → `script_encryption_key` in
  `export_presets.cfg`; < 4.1 → error early; unrecognized future schema → **fail closed**. (This is
  *config-era* strategy, distinct from the *platform* plugin framework in Slice 2.)
* **Long-path handling.** Read-only detection of Windows `LongPathsEnabled`; warn with remediation;
  use `\\?\` extended-length prefixes internally where possible; fail fast with a clear diagnostic on
  a path that will exceed `MAX_PATH`.
* **Teammate/UX note.** `.godot/export_credentials.cfg` is machine-local and usually gitignored;
  surface in output that teammates must re-run the tool (with the same backed-up key) on their
  machines.

---

## 6. Slice 2 — Multi-Platform Extensibility (the plugin framework)

\*\*Introduced here — driven by a real second target (Linux), not built speculatively for one
platform.\*\* Designing the abstraction against two concrete cases yields a better interface than
designing it against one. Because Slice 0 already isolated the platform work behind matching function
signatures (§4.6), this is an **extraction/refactor, not a rewrite**.

* **`internal/platform` registry.** Maps a platform id (`windows`, `linux`, …) to its `Provisioner`,
  `EnvironmentBuilder`, and metadata (SCons platform string, artifact naming, host/target
  compatibility). Platforms self-register via `init()` (like `database/sql` drivers) from
  `internal/platforms/<name>/`. Core CLI never imports a platform package directly.
* **`Provisioner` interface.** Per-target toolchain resolution + checksum verification + download —
  deliberately *not* assumed to be a C++ cross-compiler: Linux → GCC/Clang, Web → **Emscripten SDK**,
  Android → SDK/NDK, macOS/iOS → Xcode toolchain (macOS host). Checksum verification is a shared
  contract step, not reimplemented per plugin.
* **`EnvironmentBuilder` interface.** Builds the isolated env and invokes SCons with the correct
  `platform=` value and target flags; handles path-separator/shell differences.
* **Host/target compatibility matrix.** `cmd/` checks it before invoking a plugin and fails with a
  specific message (e.g. "Target `macos` is not buildable from a Windows host without cross-compilation
  tooling not covered by this tool") instead of failing deep inside SCons.
* **What moves, what doesn't.** Slice 0/1's `provisionWindows` / `buildWindows` / config code become
  the `windows` plugin's implementation. `cmd/`, `internal/project`, `internal/crypto`, \`internal/
  manifest\`, and the core flow are untouched.
* **Adding a platform thereafter** = implement `Provisioner` + `EnvironmentBuilder` under
  `internal/platforms/<name>/`, register the SCons string + compatibility entries, and add a config
  `CredentialWriter`/metadata only if that target needs handling beyond the generic path.
* **Extraction guarded by a contract test.** The extracted `windows` plugin must pass the *same*
  integration suite the concrete Slice 0/1 code passed (per principle 6). This is the objective check
  that the refactor preserved behavior — if it fails, the extraction changed something and must be
  fixed before any second platform is added on top.

---

## 7. Slice 3 — CI / Automation

Makes the tool safe to run unattended (e.g. GitHub Actions).

* **`--non-interactive` / `--yes`.** Every step that could prompt (key regeneration, unresolvable
  version, long-path warning) instead fails fast with a non-zero exit and a machine-readable error.
* **`--json` output mode.** Structured events so a workflow asserts on success/failure without
  scraping human text.
* **Secret-safe logging (closes the key-exposure surface from §4.5).** Mask the key in all output,
  avoid echoing the env var, and prefer a file/stdin handoff to the build over the process
  environment where feasible.
* **Manifest-keyed caching.** `manifest.json` is the natural `actions/cache` key — restore
  `templates/` + `manifest.json` keyed on resolved Godot version + platform **+ toolchain identity**
  (per the Slice 1 idempotency key); the existing idempotency check then skips the 20–60+ min compile
  on a hit. No new caching mechanism required.
* **Parallel release/debug compilation.** Build release and debug templates concurrently to reduce
  wall-clock time in CI. Keep artefact names and config-injection outputs unchanged, collect logs per
  target, and fail the run if either target fails.
* **Explicit-version recommendation.** In non-interactive mode, prefer requiring `--godot-version`
  over the latest-patch fallback to avoid time-based drift.
* **Stable exit-code contract.** CI needs to branch on *why* a run failed, not scrape text. Every
  typed error maps to a fixed exit code (e.g. `0` success, `4` version-resolution, `5`
  integrity/checksum, `6` insufficient disk, `7` build failed, `8` manual setup guidance, `9`
  unsupported version/schema, `10` lock held). The full table is in the design doc; the contract is
  that these codes are stable across releases so workflows can rely on them.

---

## 8. Slice 4+ — Breadth & Hardening

* Concrete `linux`, `web`, `android`, `macos`/`ios` plugins against §7. macOS/iOS needs a decision on
  supporting a cross-toolchain (e.g. osxcross) vs. requiring a macOS host/runner.
* Signed-artifact verification beyond checksum pinning, if upstream begins publishing signatures.
* A documented, tested GitHub Actions example using `--non-interactive` + manifest-keyed caching.
* `gst list-platforms` to introspect the registry (useful once >1 plugin exists).

---

## 9. Concern → Resolution Traceability

Every design risk identified in review, and where the plan handles it. Nothing is dropped; several
are pulled forward into the MVP because they're cheap to get right early and costly to retrofit.

| # | Concern | Severity | Resolved in |
| --- | --- | --- | --- |
| 1 | `ConfigFile` (not INI) round-trip can corrupt user settings; no Go library | High | §4.5 — targeted minimal edits, byte-preserving; full parser only if later needed |
| 2 | Single-slot `.bak` overwrites the pristine original on 2nd run | High | §4.5 — backup-once (create only if absent) |
| 3 | No transactional ordering/rollback between compile and manual setup guidance | High | §4.5 — print guidance only after verified compile; keep user files untouched |
| 4 | Key material exposed: plaintext on disk + env-var handoff (CI log leak) | High (security) | §4.4 owner-only perms; §4.5 acknowledged; §6 log masking + file/stdin handoff |
| 5 | Idempotency/cache key omits toolchain identity → stale artifacts | Medium | §5 idempotency keyed on toolchain/tool identity; §6 cache key follows suit |
| 6 | "Latest patch" fallback non-deterministic; GitHub API rate-limited/offline | Medium | §5 token + metadata cache + specific errors; §6 prefer explicit version in CI |
| 7 | No disk-space preflight before multi-GB, 20–60 min build | Medium | §4.4 — free-space check before download/compile |
| 8 | No concurrency guard / interruption safety | Medium | §4.3 — run lockfile + §4.5 atomic writes |
| 9 | Plugin framework built speculatively for one platform (over-engineering) | Medium (design) | §2.2 seams in Slice 0; §7 framework extracted in Slice 2, driven by a real 2nd target |
| 10 | Machine-local `export_credentials.cfg` surprises teammates | Low (UX) | §5 — explicit note in output/docs |
| 11 | Parsing `godot --version` build-metadata suffixes | Low | §5 — careful suffix parsing in resolver |
| 12 | Patch-mismatch tolerance (sound, but bounded to same minor line) | Low (noted) | §4.3 — minor-mismatch hard fail; patch tolerated by design |
| 13 | Config edit could write against unexpected structure and corrupt it | High | §4.5 — fail closed if section/keys not unambiguously locatable; marker-stamped edits |
| 14 | CI can't distinguish failure causes from text | Medium | §6 — stable exit-code contract (design doc has the full table) |
| 15 | "Extraction not rewrite" is an unverified claim | Medium (design) | §2 principle 6 + §7 — cross-slice contract test: `windows` plugin passes the same suite |
| 16 | Untestable due to real network/clock/40-min compile | Medium | §2 principle 6 — injected clock/HTTP/FS; stubbed-SCons integration tests |