# Godot Template Tool — Full Technical Design

**Companion to:** `gst-spec.md` (the sliced delivery plan).
**Purpose:** the sliced plan says *what ships when and why*. This document is the *how* — the
complete, detailed engineering design: data structures, algorithms, file formats, interfaces,
error taxonomy, and edge-case handling for every subsystem. Where a feature is scoped to a later
slice, it is marked `[Slice N]`; the design is described in full here regardless so the slices have
a single detailed reference to build against.

---

## Table of Contents
1. Background & domain model
2. System context & process model
3. On-disk layout & the workspace contract
4. Version resolution subsystem
5. Toolchain provisioning subsystem
6. Cryptographic key management
7. Build/compilation subsystem
8. Godot `ConfigFile` handling & config injection
9. Manifest, idempotency & caching
10. Platform plugin framework
11. CLI surface, flags & exit codes
12. Concurrency, atomicity & failure recovery
13. Error taxonomy & user messaging
14. Security model & threat analysis
15. Observability & logging
16. Testing strategy
17. Cross-cutting invariants

---

## 1. Background & Domain Model

### 1.1 The problem Godot poses
Godot ships prebuilt "export templates" — the engine binaries into which a game's packed data
(`.pck`) is embedded to produce a distributable executable. To enable **script/resource
encryption**, the templates must be **compiled from source** with the encryption key baked in at
build time (Godot reads the key from the `SCRIPT_AES256_ENCRYPTION_KEY` environment variable during
the SCons build). There is no way to encrypt scripts with the stock downloaded templates.

Compiling templates from source requires a full native toolchain (a C++ compiler, Python, SCons)
plus the exact Godot source tree. Setting that up by hand is error-prone and pollutes the
developer's machine. **This tool automates that provisioning + compile + wire-in, in an isolated,
per-project sandbox.**

### 1.2 Key domain terms
| Term | Meaning |
|---|---|
| **Export template** | Engine binary (`.exe` on Windows) used as the base for an exported game. Release and debug variants. |
| **Custom template** | An export preset field (`custom_template/release`/`debug`) pointing at a user-supplied template, bypassing Godot's managed template store and its strict exact-version check. |
| **Encryption key** | 256-bit AES key. Compiled into the template; also referenced by the project so the editor encrypts the `.pck` to match. |
| **`export_presets.cfg`** | Godot `ConfigFile` describing each export target's settings. Project-local, usually committed. |
| **`export_credentials.cfg`** | Godot **4.3+** `ConfigFile` (under `.godot/`) holding secrets including the script encryption key. Machine-local, usually gitignored. |
| **`ConfigFile`** | Godot's own serialization format. Superficially INI-like but carries typed literals; **not** INI. |
| **SCons** | Godot's build system. Invoked headless with `platform=…` and target flags. |

### 1.3 Godot version/era matrix (authoritative for config routing)
| Godot line | Encryption key location | Tool support |
|---|---|---|
| `< 4.1` | pre-credentials, differing conventions | **Unsupported** — hard error at version resolution `[Slice 1]` |
| `4.1 – 4.2` | `script_encryption_key` inside the preset in `export_presets.cfg` | Supported `[Slice 1]` |
| `4.3+` | `encryption/script_encryption_key` in `.godot/export_credentials.cfg` | Supported `[Slice 0]` |
| unknown newer | unknown schema | **Fail closed** — no guessing |

### 1.4 Compatibility model (why patch drift is tolerated)
Godot guarantees API/ABI stability across **patch** releases within a minor line. A project authored
against 4.3.0 built with templates compiled from 4.3.2 source is expected to work. Because the tool
injects a **custom** template path, it is exempt from the editor's exact-version match on managed
templates. Therefore the tool:
* **Blocks on a minor-line mismatch** (compatibility guarantees end there).
* **Tolerates and does not warn on** a patch-level difference between the project's authored version
  and the resolved build version.

---

## 2. System Context & Process Model

### 2.1 Actors
* **Developer (interactive):** runs the tool at a terminal, may answer prompts.
* **CI runner (non-interactive) `[Slice 3]`:** no stdin; requires deterministic inputs and
  machine-readable output.
* **External services:** Godot GitHub Releases API (version + source), toolchain artifact hosts
  (Python, MinGW-w64, SCons).

### 2.2 High-level execution pipeline
```
create
  → [2] preflight            (env, disk, lock, project detection)
  → [4] resolve version
  → [3] init workspace + .gitignore
  → [5] provision toolchain  (download → verify checksum → extract)
  → [6] ensure key
  → [7] compile templates    (SCons, staged progress)
  → [9] write/refresh manifest
  → [8] inject config        (backup-once → atomic write → verify → or rollback)
  → [3] prune runtime        [Slice 1, default on success]
  → print success summary
```
Each stage is a pure-ish function taking a `RunContext` (resolved config + paths + logger) and
returning either the next state or a typed error (§13). Stages are individually testable and, where
marked, individually skippable via idempotency (§9).

### 2.3 `RunContext` (the value threaded through the pipeline)
```go
type RunContext struct {
    ProjectRoot   string          // absolute; dir containing project.godot
    Workspace     Workspace       // resolved .gst/ paths
    Godot         ResolvedVersion // §4
    Platform      PlatformID      // "windows" in Slice 0; registry key later
    Flags         Flags           // parsed CLI flags (§11)
    Interactive   bool            // false in CI [Slice 3]
    Logger        Logger          // structured; JSON sink optional [Slice 3]
    Clock         func() time.Time
    HTTP          Doer            // injectable for tests
}
```
Everything nondeterministic (time, network, filesystem root) is injected so the pipeline is
fully testable (§16).

---

## 3. On-Disk Layout & the Workspace Contract

### 3.1 Canonical layout
```
<ProjectRoot>/
├── project.godot
├── export_presets.cfg
├── export_presets.cfg.bak                 # backup-once (§8.4)
├── .godot/
│   ├── export_credentials.cfg             # 4.3+ key target
│   └── export_credentials.cfg.bak
├── .gst/
│   ├── runtime/                           # ephemeral; pruned [Slice 1]
│   │   ├── python/
│   │   ├── mingw/
│   │   ├── scons/
│   │   └── godot_source/<version>/
│   ├── templates/
│   │   ├── windows_release.exe
│   │   └── windows_debug.exe
│   ├── logs/<timestamp>-<stage>.log
│   ├── manifest.json                      # [Slice 1]
│   ├── .lock                              # run lock (§12)
│   └── encryption.key                     # 0600 perms
└── .gitignore                             # `.gst/` appended
```

### 3.2 Workspace struct
```go
type Workspace struct {
    Root        string // <ProjectRoot>/.gst
    Runtime     string // Root/runtime
    Templates   string // Root/templates
    Logs        string // Root/logs
    Manifest    string // Root/manifest.json
    Lock        string // Root/.lock
    KeyFile     string // Root/encryption.key
}
```

### 3.3 Invariants (enforced at every stage)
* Nothing outside `<ProjectRoot>` is ever written (isolation).
* `templates/`, `encryption.key`, and `manifest.json` are **never** deleted by pruning or `clean`
  without explicit confirmation.
* `.gitignore` contains `.gst/` before any secret is written to disk.
* All paths are normalized to absolute early; on Windows, extended-length (`\\?\`) prefixes are
  applied internally where a path may exceed `MAX_PATH` `[Slice 1]`.

---

## 4. Version Resolution Subsystem

### 4.1 Data model
```go
type ResolvedVersion struct {
    Minor   string  // "4.3"  (from project.godot config/features)
    Patch   string  // "4.3.2"
    Method  ResolutionMethod // Override | LocalEditor | LatestPatch | Prompt
    Source  string  // e.g. "godot --version", "GitHub releases API"
}
type ResolutionMethod int
```

### 4.2 Minor-line extraction (all slices)
Parse `project.godot`'s `config/features` — a `PackedStringArray` such as
`PackedStringArray("4.4", "Forward Plus")`. The first token matching `^\d+\.\d+$` is the minor line.
If absent or unparseable → `ErrProjectVersionUnreadable` (§13).

### 4.3 Resolution strategy chain (ordered; `[Slice 1]` except override)
Implemented as an ordered slice of `Resolver`s so a future strategy (e.g. a version-lock file)
inserts without touching callers:
```go
type Resolver interface {
    // Returns (version, true, nil) on success; (_, false, nil) to defer to next;
    // (_, _, err) only on a hard, non-recoverable failure.
    Resolve(ctx *RunContext, minor string) (patch string, ok bool, err error)
}
```
1. **`OverrideResolver`** — `--godot-version=X.Y.Z` used verbatim. **Required in Slice 0.**
   Validates that `X.Y` == project minor line, else `ErrMinorMismatch`.
2. **`LocalEditorResolver` `[Slice 1]`** — discovers an editor via `--godot-editor-path`, then
   `PATH`, then per-OS common install locations. Runs `godot --version`; parses output of the form
   `4.4.1.stable.official.<hash>` → keeps `4.4.1`, discards build-metadata suffix. Only accepts if
   the reported minor line matches; otherwise defers.
3. **`LatestPatchResolver` `[Slice 1]`** — queries the Godot GitHub Releases API for the newest
   stable tag on the minor line. Concerns handled:
   * Rate limiting: unauthenticated 60/hr shared across a CI egress IP → supports
     `GITHUB_TOKEN`/`--github-token`; caches the API response under `runtime/` keyed by minor line
     with a short TTL.
   * Offline/API failure → returns a **hard** error (specific: `ErrVersionAPIUnreachable`), not a
     silent guess.
   * Determinism caveat: result is time-dependent. The chosen patch is **always** written to the
     manifest so re-runs are deterministic, and CI is advised to pass `--godot-version` (§11).
4. **`PromptResolver` `[Slice 1, interactive only]`** — last resort; asks the user to confirm/enter.
   In non-interactive mode this resolver is omitted and the chain ends in `ErrVersionUnresolved`.

### 4.4 Output
Resolution always echoes, e.g. `Detected Godot 4.4 → resolved 4.4.2 via local editor`, and records
`{minor, patch, method, source}` in the manifest `[Slice 1]`.

---

## 5. Toolchain Provisioning Subsystem

### 5.1 Responsibilities
Resolve → download → **verify** → extract the components a target needs, into `runtime/`, touching
no system state. In Slice 0 this is the concrete `provisionWindows`; its signature already matches
the future `Provisioner` interface (§10) so extraction is mechanical.

### 5.2 Component descriptor
```go
type Artifact struct {
    Name        string // "mingw", "python", "scons", "godot_source"
    URL         string // resolved per version
    SHA256      string // PINNED in the tool's release, not fetched at runtime
    ExtractTo   string // subdir under runtime/
    Kind        ArchiveKind // zip | tar.xz | tar.gz | raw
}
```

### 5.3 Windows component set (Slice 0)
| Component | Purpose | Notes |
|---|---|---|
| Portable Python | Runs SCons | Embeddable distribution; no installer |
| MinGW-w64 | C++ toolchain | GCC targeting Windows |
| SCons | Build driver | Installed into the portable Python env |
| Godot source tarball | The thing being compiled | Tag = resolved patch version |

### 5.4 Provisioning algorithm
```
preflight_disk_space(required_estimate)            # §5.6
for artifact in components(version):
    if cached_and_verified(artifact): continue
    tmp = download(artifact.URL) -> runtime/.dl/    # streamed, resumable-friendly
    got = sha256(tmp)
    if got != artifact.SHA256: abort ErrChecksumMismatch(artifact, got)
    extract(tmp, artifact.ExtractTo)                # long-path aware [Slice 1]
    fsync + record checksum
```

### 5.5 Integrity verification (security-critical, **in the MVP**)
* Checksums are **pinned in the CLI's own release manifest**, never fetched at runtime from the same
  channel as the artifact (defeats a compromised mirror serving matching artifact+hash).
* A mismatch is a **hard abort** with a specific error — the tool handles key material downstream,
  so a tampered toolchain is unacceptable.
* `[Slice 4+]` optional signature verification if upstreams publish signatures.

### 5.6 Disk-space preflight
A source checkout + toolchain is multiple GB; the compile is 20–60+ min. Before downloading, stat
free space on the workspace volume against a per-version estimate (with headroom) and fail early
with `ErrInsufficientDisk` naming the shortfall, rather than dying mid-build.

### 5.7 Windows long-path handling `[Slice 1]`
`.gst/runtime/godot_source/...` nested under an already-deep project can exceed the
260-char `MAX_PATH`. The tool: (1) read-only detects `LongPathsEnabled` (never modifies the
registry); (2) warns with the exact remediation if disabled; (3) uses `\\?\` extended-length
prefixes internally where possible; (4) fails fast with a clear diagnostic if a path will exceed the
limit, instead of a cryptic SCons/compiler error.

---

## 6. Cryptographic Key Management

### 6.1 Generation & reuse
```
if exists(KeyFile):
    key = read(KeyFile)                 # reuse — always safe
else:
    key = crypto/rand 32 bytes          # AES-256
    write(KeyFile, hex(key), perm=0600) # owner-only
```
The key is stored hex-encoded (Godot expects a 64-char hex string). File perms are owner-only on
POSIX; on Windows an equivalent restrictive ACL is applied.

### 6.2 Regeneration `[Slice 1]`
Regeneration invalidates every previously exported build that embedded the old key, so it never
happens implicitly. `--regenerate-key` requires interactive confirmation (or `--force` in
automation). The old key is backed up to `encryption.key.bak` before overwrite.

### 6.3 Handoff to the build
Passed to SCons via `SCRIPT_AES256_ENCRYPTION_KEY` (Godot's convention). Exposure surface and its
mitigations are detailed in §14.3.

### 6.4 What the tool never does
* Never transmits the key over the network.
* Never writes the key to a log, manifest, or any file other than `encryption.key`(+`.bak`).
* Never commits it — `.gitignore` is ensured before generation, though this is defense-in-depth, not
  the primary secrecy boundary (perms are).

---

## 7. Build / Compilation Subsystem

### 7.1 Isolated environment construction
Build a bespoke environment map rather than mutating the process/user environment:
```
PATH      = runtime/python;runtime/mingw/bin;runtime/scons;<minimal system essentials>
PYTHONHOME/PYTHONPATH = runtime/python
SCONS_… flags for the platform
SCRIPT_AES256_ENCRYPTION_KEY = <key>       # see §14.3 for CI hardening
```
Path-separator (`;` vs `:`) and shell differences are localized to the builder; in Slice 0 this is
`buildWindows`, matching the future `EnvironmentBuilder` interface (§10).

### 7.2 Invocation
Headless SCons, once per target variant:
```
scons platform=windows target=template_release  <arch/flags>
scons platform=windows target=template_debug    <arch/flags>
```
Artifacts are moved into `templates/` with the canonical namespaced names
(`windows_release.exe`, `windows_debug.exe`).

### 7.3 Progress reporting
* **Slice 0:** stream raw SCons stdout/stderr to the terminal and to `logs/`.
* **`[Slice 1]` staged updates:** a line scanner maps SCons module boundaries to human stages
  (`Compiling core…`, `Compiling drivers…`, `Compiling platform/windows…`, `Linking…`) plus an
  elapsed-time counter. No fake linear percentage — a from-source compile doesn't map cleanly to
  one. The scanner is tolerant: unrecognized lines pass through at debug verbosity.

### 7.4 Idempotency hook
Before invoking SCons, consult the manifest (§9). If a prior successful build matches the **full**
input fingerprint and the artifacts exist, skip. `--force-rebuild` overrides. If `runtime/` was
pruned, re-provision first.

### 7.5 Failure handling
Non-zero SCons exit → capture the tail of the log, classify common causes (missing long-path
opt-in, out-of-disk, compiler error) into actionable messages where possible, and return
`ErrBuildFailed` with a pointer to the full log. **No config is injected** on build failure (§8.2).

---

## 8. Godot `ConfigFile` Handling & Config Injection

### 8.1 Why this is the highest-risk subsystem
`export_presets.cfg` / `export_credentials.cfg` are Godot `ConfigFile` — INI-ish but carrying typed
literals (`PackedStringArray(...)`, `Dictionary(...)`, resource references, booleans, floats, nested
quoting). **No mature Go library round-trips this format.** A generic INI/serializer that reorders
keys, strips comments, or reformats literals will silently corrupt a user's export settings.

### 8.2 Strategy: targeted minimal edits (all slices)
Do **not** parse-and-round-trip the whole file. Instead:
1. Read the file as text.
2. Locate the target `[section]` (e.g. the Windows preset, or `[preset.N.options]`).
3. Within it, find-or-insert only the specific keys being set.
4. Rewrite **only** those lines; preserve every other byte (including comments, ordering, and
   untouched literals) exactly.

A small, well-tested section/key locator handles the addressing; it understands `ConfigFile` section
headers and key boundaries but deliberately does **not** attempt to interpret arbitrary typed
values. If the structure needed for a targeted edit can't be located unambiguously, it **fails
closed** (`ErrConfigStructureUnrecognized`) rather than writing.

> A full `ConfigFile` parser is only built if a future slice genuinely needs to *read* complex typed
> values; injection does not require it.

### 8.3 Injection ordering (transactional shape)
```
assert(build_succeeded)                     # never inject before a verified compile
for each target file (presets, credentials):
    if not exists(file.bak): copy(file -> file.bak)   # backup-once (§8.4)
    new = targeted_edit(read(file))
    atomic_write(file, new)                 # temp + fsync + rename (§12)
verify(files parse-locatable & contain expected keys)
on any failure: restore(file <- file.bak) for all touched files ; return error
```
This guarantees the project is never left with new templates + broken config, or edited config +
absent templates.

### 8.4 Backup-once semantics (fixes the clobber bug)
A naive "back up on every run" overwrites the pristine original with the tool's own
already-modified file on the second run. Therefore: **create `.bak` only if it does not already
exist.** Optionally stamp tool-authored regions with a marker comment so re-runs can distinguish
their own edits. `clean`/uninstall can offer to restore from `.bak`.

### 8.5 Fields written
* `export_presets.cfg` → the Windows preset's `custom_template/release` and `custom_template/debug`
  = absolute (or project-relative, configurable) paths to the compiled templates.
* Encryption key target, **version-gated** via a `CredentialWriter` selected by version range
  `[Slice 1 for multi-era; Slice 0 implements only 4.3+]`:
  * `4.3+` → `encryption/script_encryption_key` in `.godot/export_credentials.cfg`.
  * `4.1–4.2` → `script_encryption_key` in the preset inside `export_presets.cfg`.
  * `<4.1` → error at version resolution.
  * unknown newer → **fail closed**.

### 8.6 `CredentialWriter` interface `[Slice 1]`
```go
type CredentialWriter interface {
    Supports(v ResolvedVersion) bool
    Write(files *ProjectConfigFiles, key string) error
}
// Registered in a version-range table; selection is data-driven, not an if/else ladder.
// Distinct from the platform plugin registry (§10): this varies by Godot *era*, not by build target.
```

---

## 9. Manifest, Idempotency & Caching `[Slice 1+]`

### 9.1 Schema (`manifest.json`)
```json
{
  "schemaVersion": 1,
  "godot":     { "minor": "4.3", "patch": "4.3.2", "method": "local_editor", "source": "godot --version" },
  "platform":  "windows",
  "toolchain": { "mingw": "sha256:…", "python": "sha256:…", "scons": "sha256:…", "godotSource": "sha256:…" },
  "toolVersion": "gst 0.3.1",
  "config":    { "encryptionKeyTarget": "export_credentials.cfg", "targets": ["release","debug"] },
  "build":     { "startedAt": "…", "finishedAt": "…", "status": "success" },
  "artifacts": { "windows_release.exe": "sha256:…", "windows_debug.exe": "sha256:…" }
}
```
`platform` and the toolchain map are present **from day one** even though Slice 1 only writes
`"windows"`, so caching/idempotency generalize to multi-platform without a schema change.

### 9.2 The build fingerprint (idempotency/cache key)
Critically, the key includes **more than the Godot version**:
```
fingerprint = H(godot.patch, platform, toolchain.*, toolVersion, config.*)
```
Omitting toolchain/tool identity (the flaw in the naive design) would reuse a stale artifact after a
MinGW/SCons/tool bump while the Godot version is unchanged. The manifest already records these
fields; the fingerprint simply folds them in.

### 9.3 Idempotency decision
Skip compilation iff: manifest `status == success` **and** its fingerprint == current fingerprint
**and** the referenced artifacts exist with matching hashes. Else rebuild. `--force-rebuild`
bypasses.

### 9.4 CI caching `[Slice 3]`
`manifest.json` is the natural `actions/cache` key. Restore `templates/` + `manifest.json` keyed on
the fingerprint (Godot version + platform + toolchain identity). On a hit, §9.3 skips the 20–60+ min
compile; on a miss (e.g. a version bump), a normal build runs. No new caching mechanism is
introduced.

---

## 10. Platform Plugin Framework `[Slice 2]`

### 10.1 Motivation & timing
Built **when a real second target (Linux) exists**, not speculatively for one platform. Designing
the interfaces against two concrete cases (Windows + Linux) yields a better abstraction. Because
Slice 0/1 already isolated platform work behind matching function signatures (§5.1, §7.1), this is
an **extraction**, not a rewrite.

### 10.2 Registry
```go
type PlatformID string

type Platform struct {
    ID            PlatformID
    SconsPlatform string          // "windows", "linuxbsd", "web", …
    ArtifactNames func(target string) string
    Provisioner   Provisioner
    Builder       EnvironmentBuilder
    Hosts         []HostID        // hosts this target can be built from
}

// Self-registration, database/sql-style:
func Register(p Platform)         // called from internal/platforms/<name>/init()
func Lookup(id PlatformID) (Platform, bool)
```
Core CLI code never imports a platform package directly; it only talks to the registry. Platform
packages are blank-imported to trigger `init()`.

### 10.3 Interfaces
```go
type Provisioner interface {
    // Resolve + download + checksum-verify + extract this target's toolchain.
    Components(v ResolvedVersion) ([]Artifact, error)
    // Verification is a shared step provided by the framework, not reimplemented per plugin.
}

type EnvironmentBuilder interface {
    Env(ctx *RunContext) (map[string]string, error) // isolated env incl. key handoff
    Invoke(ctx *RunContext, target BuildTarget) error // SCons with correct platform= + flags
}
```

### 10.4 Toolchain reality per target (why `Provisioner` is not "a C++ cross-compiler")
| Target | Toolchain | Notes |
|---|---|---|
| Windows | MinGW-w64 | Slice 0/1 |
| Linux (`linuxbsd`) | GCC/Clang | first extraction driver, Slice 2 |
| Web (HTML5) | **Emscripten SDK** | not a C++ cross-compiler in the MinGW/GCC sense |
| Android | Android SDK/NDK | |
| macOS / iOS | Xcode toolchain | generally requires a macOS host |

### 10.5 Host/target compatibility matrix
The registry carries which hosts can build which targets. `cmd/` checks it **before** invoking a
plugin and fails with a specific message (e.g. "Target `macos` is not buildable from a Windows host
without cross-compilation tooling not covered by this tool") instead of failing deep inside SCons.
Windows host → Windows/Linux/Web are broadly viable; Windows host → macOS/iOS is not, absent an
osxcross-style toolchain the tool does not assume.

### 10.6 Adding a platform (the payoff)
Implement `Provisioner` + `EnvironmentBuilder` under `internal/platforms/<name>/`, register the
SCons string + compatibility entries, and add a config `CredentialWriter`/metadata only if the
target needs handling beyond the generic path. **No changes** to `cmd/`, `internal/project`,
`internal/crypto`, `internal/manifest`, or the core flow.

---

## 11. CLI Surface, Flags & Exit Codes

### 11.1 Commands
| Command | Slice | Purpose |
|---|---|---|
| `create` | 0 | The full pipeline (§2.2). |
| `clean` | 1 | Remove `runtime/` (and optionally `templates/`, with confirmation). Never touches key/manifest silently. |
| `list-platforms` | 4+ | Introspect the registry. |

### 11.2 Flags
| Flag | Slice | Effect |
|---|---|---|
| `--godot-version=X.Y.Z` | 0 (required), 1 (optional override) | Pins the version; bypasses inference. |
| `--platform=<id>` | 0 (hard-wired `windows`), 3 (registry) | Selects the target plugin. |
| `--keep-runtime` | 0 (accepted no-op), 1 (active) | Skip pruning of `runtime/`. |
| `--force-rebuild` | 1 | Ignore idempotency, rebuild. |
| `--regenerate-key` / `--force` | 1 | Regenerate the AES key (guarded). |
| `--godot-editor-path=<path>` | 1 | Point at a specific editor for resolution. |
| `--github-token=<tok>` | 1 | Authenticated Releases API calls. |
| `--non-interactive` / `--yes` | 2 | Disable all prompts; fail fast instead. |
| `--json` | 2 | Structured event output. |

### 11.3 Exit codes (stable contract for CI `[Slice 3]`)
| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Generic/unexpected failure |
| 2 | Usage error (bad flags) |
| 3 | Not a Godot project / project unreadable |
| 4 | Version resolution failed |
| 5 | Integrity/checksum failure |
| 6 | Insufficient disk |
| 7 | Build failed |
| 8 | Config injection failed (rolled back) |
| 9 | Unsupported Godot version/schema (fail-closed) |
| 10 | Lock held by another run |

---

## 12. Concurrency, Atomicity & Failure Recovery

### 12.1 Run lock
An exclusive lockfile at `.gst/.lock` (advisory + PID/host stamp) is acquired for
the whole run. A second concurrent invocation exits with code 10 and a clear message rather than
racing on `runtime/`, the key, or config. Stale locks (dead PID) are detected and reclaimed.

### 12.2 Atomic file writes
Every mutation of a user file (`export_presets.cfg`, `export_credentials.cfg`, `.gitignore`,
`encryption.key`, `manifest.json`) uses **temp-file + fsync + rename** on the same volume, so a
crash mid-write cannot truncate or corrupt the original.

### 12.3 Recovery matrix
| Failure point | State left | Recovery |
|---|---|---|
| During download/extract | partial `runtime/.dl` | discarded/re-verified next run; nothing user-facing touched |
| During compile | no config changes yet | safe; re-run resumes (idempotency/reprovision) |
| During config injection | `.bak` exists, atomic write | auto-restore from `.bak`; return code 8 |
| Interrupted (Ctrl-C/kill) | lock + temp files | lock reclaimed; temps ignored; user files intact |

---

## 13. Error Taxonomy & User Messaging

### 13.1 Principles
* Every error is **typed** and maps to a **stable exit code** (§11.3) and a **specific, actionable
  message**. No bare `error` strings bubbling to the user.
* **Fail closed:** unknown version/schema/host-target → explicit error, never a guess.
* Messages state *what happened*, *why*, and *the next action*.

### 13.2 Representative errors
| Type | Exit | Example message |
|---|---|---|
| `ErrNotGodotProject` | 3 | "No `project.godot` found in <dir>. Run this from your Godot project root." |
| `ErrMinorMismatch` | 3 | "Project targets Godot 4.3 but `--godot-version=4.4.1` is on a different minor line." |
| `ErrVersionUnresolved` | 4 | "Could not determine the Godot patch version. Pass `--godot-version=X.Y.Z`." |
| `ErrVersionAPIUnreachable` | 4 | "GitHub Releases API unreachable (offline or rate-limited). Pass `--godot-version` or `--github-token`." |
| `ErrChecksumMismatch` | 5 | "Integrity check failed for `mingw` (expected …, got …). Aborting; toolchain may be tampered." |
| `ErrInsufficientDisk` | 6 | "Need ~<N> GB free on <volume>, found <M> GB." |
| `ErrBuildFailed` | 7 | "SCons build failed at stage <stage>. See `logs/<file>`. Likely cause: <hint>." |
| `ErrConfigStructureUnrecognized` | 8/9 | "Could not locate the Windows preset in `export_presets.cfg`; refusing to edit to avoid corruption." |
| `ErrUnsupportedGodot` | 9 | "Godot <v> config schema is not supported by this tool version." |
| `ErrLockHeld` | 10 | "Another run holds the lock (pid <p>). Wait for it to finish or remove `.lock` if stale." |

---

## 14. Security Model & Threat Analysis

### 14.1 Assets
1. The **AES-256 encryption key** (highest value).
2. The **integrity of the build toolchain** (a tampered toolchain could exfiltrate the key or
   backdoor the binary).
3. The user's **project config** (corruption = lost work / broken exports).

### 14.2 Toolchain supply chain
* Checksums **pinned in the tool's release**, not co-fetched with artifacts (§5.5).
* Hard abort on mismatch.
* `[Slice 4+]` signature verification when available.

### 14.3 Key exposure surface & mitigations
| Surface | Risk | Mitigation |
|---|---|---|
| On-disk `encryption.key` | Readable by other local users | `0600`/owner-only ACL; documented that gitignore ≠ secrecy |
| VCS | Accidental commit | `.gitignore` ensured before generation (defense-in-depth) |
| Env var to SCons | Visible in `ps`/`/proc`; **captured in CI logs** | `[Slice 3]` mask in all tool output; avoid echoing; prefer file/stdin handoff to the build where feasible |
| Logs/manifest | Accidental leak | Key is **never** written to logs or manifest by contract (§6.4) |

### 14.4 Isolation guarantees
No system env vars, no registry writes, no installs outside `<ProjectRoot>`. Long-path detection is
**read-only**. This bounds blast radius and makes the tool safe to run on shared/CI machines.

### 14.5 Non-goals
* Not a secrets manager — it generates and stores one local key; distributing it to teammates is the
  user's responsibility (the tool surfaces this, §8/§UX).
* Does not attempt to encrypt the key at rest (would need a second secret; out of scope).

---

## 15. Observability & Logging

* **Human logs:** streamed to the terminal; full detail persisted to `logs/<timestamp>-<stage>.log`.
* **Structured logs `[Slice 3]`:** `--json` emits one event object per stage transition
  (`{stage, status, elapsedMs, …}`) so CI can assert without scraping text.
* **Secret redaction:** a logging middleware refuses to emit the key value (matched by length/format
  and by the known variable name).
* **Manifest as audit trail:** the resolved version + method + toolchain checksums make each run
  reproducible and diagnosable after the fact.

---

## 16. Testing Strategy

### 16.1 Unit
* **`ConfigFile` targeted-edit** golden tests: a corpus of real-world `export_presets.cfg` /
  `export_credentials.cfg` files (with comments, `PackedStringArray`, `Dictionary`, odd quoting) →
  assert byte-for-byte preservation of everything except the intended keys. This is the
  highest-value test suite (§8 is the highest-risk subsystem).
* **Version parsing:** `config/features` variants; `godot --version` suffix stripping; minor-mismatch
  detection.
* **Backup-once & rollback:** simulate first/second runs and mid-injection failure; assert the
  pristine original survives and rollback restores state.
* **Fingerprint:** toolchain bump changes the fingerprint (idempotency correctness).

### 16.2 Integration (with injected `HTTP`/`Clock`/FS)
* Full pipeline against a **fake** artifact server + fixture Godot project, with SCons stubbed, to
  exercise ordering, atomicity, and error paths without a 40-minute compile.
* Concurrency: two pipelines racing on one workspace → one gets the lock, the other exits 10.

### 16.3 End-to-end (slow, gated)
* A real Windows→Windows compile on Godot 4.3.x producing runnable encrypted templates. Run in CI on
  a schedule, not per-commit, due to duration.

### 16.4 Cross-slice contract tests
* When the plugin framework lands `[Slice 2]`, the extracted `windows` plugin must pass the **same**
  integration suite the concrete Slice 0/1 code passed — proving the extraction preserved behavior.

---

## 17. Cross-Cutting Invariants (must hold in every slice)

1. Never write outside `<ProjectRoot>`; never modify system env/registry.
2. Never inject config before a **verified** successful compile.
3. Never overwrite an existing `.bak`.
4. All user-file mutations are atomic (temp + rename).
5. The key is written only to `encryption.key`(+`.bak`), with restrictive perms, and never logged.
6. Integrity-check every downloaded artifact against a pinned checksum; abort on mismatch.
7. Unknown version / schema / host-target combination → explicit fail-closed error.
8. Hold the run lock for the duration; make interruption non-corrupting.
9. Every consequential decision is echoed and (from Slice 1) recorded in the manifest.
10. Platform-specific logic stays behind the §4.6 seams so Slice 2 extraction stays mechanical.
