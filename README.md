# Godot Secure Templater (gst)

**Secure encrypted Godot export templates, without the toolchain hassle.**

`gst` automates provisioning, compilation, and configuration of encrypted Godot export templates—all in an isolated workspace within your project. No manual C++ toolchain setup. No Python dependency hell. No SCons config. Just one command.

```bash
$ gst create --godot-version 4.3.2
✅ Success! Encrypted templates compiled and configured.
```

## What It Does

### 🔒 End-to-End Encryption
- Generates a machine-specific AES-256 encryption key
- Compiles Godot templates with `SCRIPT_AES256_ENCRYPTION_KEY` baked in
- Wires the key into your export settings
- **Result:** Your game scripts and resources are encrypted in the final binary

### 🛠️ Automated Toolchain
- Downloads and verifies Python, MinGW, SCons, and Godot source automatically
- Runs everything in `.gst/` (isolated workspace)
- Cleans up after build (optional `--keep-runtime` for debugging)
- **Result:** No manual compiler installation ever

### 🚀 One Command Per Team Member
Each team member runs the tool on their own machine. Each gets their own encryption key.

```
📋 Note for teammates:
   The encryption key in .godot/export_credentials.cfg is machine-specific.
   Each team member must run this tool locally on their machine.
   Do NOT share encryption keys between machines.
```

### 🔄 Smart Rebuilds
- Caches build fingerprint (version, checksums, platform)
- Skips rebuild if inputs haven't changed
- Force rebuild with `--force-rebuild` if needed

### 📍 Platform-Ready
- **Now:** Windows Desktop (4.3+, real SCons compilation with AES-256 encryption)
- **Soon:** Linux, Web, macOS/iOS, Android (same `gst` command)

---

## Installation

### From Source

```bash
git clone https://github.com/joemi/godot-secure-templater.git
cd godot-secure-templater
mkdir -p dist
go build -o dist/gst ./cmd/gst
sudo mv dist/gst /usr/local/bin/  # or add to PATH
```

### Cross-Compilation

Compile for different platforms using environment variables:

```bash
# Windows (64-bit)
mkdir -p dist
GOOS=windows GOARCH=amd64 go build -o dist/gst.exe ./cmd/gst

# Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -o dist/gst ./cmd/gst

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dist/gst ./cmd/gst

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dist/gst ./cmd/gst
```

The resulting binary runs on any machine with that OS/architecture—no additional dependencies needed.

### Requires
- Go 1.21+
- Windows 10/11 host (for Windows templates)
  - MinGW 14.2.0 downloaded and provisioned automatically
  - Python 3.11 embedded distribution (no system Python needed)
  - SCons 4.4.0 provisioned automatically
- Internet connection (first run downloads ~1GB of toolchain)
- 5+ GB disk space (toolchain + source + build artefacts)

---

## Quick Start

### 1. Prepare Your Godot Project

```bash
cd /path/to/your/game
```

Ensure your project has a `project.godot` file

### 2. Run gst

```bash
gst create
```

**Flags:**
- `--godot-version VERSION` (required) — Godot version to compile (must match project)
- `--verbose` — Show detailed build output
- `--keep-runtime` — Preserve toolchain after build (useful for repeated builds or debugging)
- `--force-rebuild` — Skip cache; always recompile templates
- `--regenerate-key` — Generate a new encryption key (requires confirmation)
- `--force` — Skip all confirmations (CI/automation mode)

### 3. Export Your Game

1. Open your project in Godot Editor
2. Go to **Project → Export**
3. Select the **Windows Desktop** preset
4. Click **Export Project**
5. Your game is now compiled with encrypted scripts!

---

## What Gets Created

Inside your project:

```
my-game/
├── .gst/                          # Isolated workspace (add to .gitignore)
│   ├── runtime/                   # Toolchain (Python, MinGW, SCons, Godot source)
│   ├── templates/                 # Compiled export templates (windows_release.exe, etc.)
│   ├── encryption.key             # Your machine's encryption key (owner-read-only)
│   └── manifest.json              # Build metadata for caching
├── .godot/
│   └── export_credentials.cfg     # Where the encryption key is wired
├── export_presets.cfg             # Export settings (auto-updated)
└── project.godot                  # Your project (unchanged)
```

**Important:** Add `.gst/` to your `.gitignore`—it's generated locally per team member.

```bash
echo ".gst/" >> .gitignore
```

The encryption key (`encryption.key`) is **machine-specific** and **owner-read-only** (permissions `0600`).

---

## Workflow for Teams

### First-Time Setup (Each Team Member)

```bash
# Developer A
$ gst create --godot-version 4.3.2
✅ Success! Templates compiled with Developer A's key.

# Developer B (different machine)
$ gst create --godot-version 4.3.2
✅ Success! Templates compiled with Developer B's key.
# Developer B's .gst/encryption.key is different from Developer A's.
```

### Repeated Builds

```bash
# Same developer, same machine, no changes to source
$ gst create --godot-version 4.3.2
✓ Cache hit! Skipping rebuild.
```

### New Encryption Key (Key Rotation)

```bash
$ gst create --godot-version 4.3.2 --regenerate-key
⚠️  Regenerating encryption key invalidates prior builds.
    Proceed? (y/n): y
✅ New key generated. Prior builds are no longer decryptable with this key.
```

---

## Troubleshooting

### "Project path exceeds Windows MAX_PATH (260 characters)"

Windows has a `MAX_PATH` limit. If your project path is too long:

```
⚠️  Project path approaching Windows MAX_PATH limit (90% = 234 chars).
    Shorten your path or enable long-path support:
    https://docs.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation
```

**Fix:**
- Shorten the project path, or
- Enable long-path support in Windows (see link above)

### "Version mismatch: project targets 4.3, supplied 4.2.1"

Your `project.godot` declares a different minor version.

```bash
# If you really want 4.2.1, update project.godot first
# Then use --force-rebuild to recompile
$ gst create --godot-version 4.2.1 --force-rebuild
```

### "Permission denied: .gst/encryption.key"

The key file has restrictive permissions (`0600`—owner-read-only). Ensure you're running `gst` as the user who owns the key.

---

## Performance

- **First run:** ~2–5 minutes (downloads toolchain, ~1GB)
- **Cached run:** <1 second (fingerprint check, exits early)
- **Fresh recompile:** ~3–5 minutes (multicore SCons compilation: auto-detects all CPU cores)
  - Parallelism: Uses N-1 cores by default (e.g., 15 cores on 16-core machine)

---

## Privacy & Security

- **Encryption key:** AES-256, generated with `crypto/rand`, stored owner-only (`0600`)
- **Scripts/resources:** Encrypted with your key in the final `.exe`
- **Build artefacts:** Automatically cleaned up (`.gst/runtime/` removed after build)
- **Manifest:** Records version, platform, and method (for audit trail in Slice 2+)

**Keys are never shared, never transmitted, never stored in version control.**

---

## How It Works Under the Hood

### Toolchain Provisioning (Automated)

1. **Python 3.11 (Embedded)** — Self-contained, no system Python required
2. **MinGW 14.2.0** — Modern GCC toolchain for Windows compilation
3. **SCons 4.4.0** — Build system that compiles Godot templates
4. **Godot Source** — Downloaded and extracted per version

All downloaded, verified (SHA-256), and extracted to `.gst/runtime/` in one step.

### SCons Compilation with Encryption

```bash
python /path/to/scons/__main__.py \
  platform=windows target=release dev_build=no optimize=speed d3d12=no \
  SCRIPT_AES256_ENCRYPTION_KEY=your-hex-key
```

- **d3d12=no:** Disables D3D12 driver to avoid Windows SDK dependency
- **Multicore:** SCons auto-detects CPU cores and compiles in parallel
- **Encryption:** Key baked into binary during compilation

### Configuration Injection

After compilation, the tool injects your encryption key into:
- **Godot 4.3+:** `.godot/export_credentials.cfg` (dedicated file)
- **Godot 4.1–4.2:** `export_presets.cfg` (embedded key line)

**Byte-preserving:** File formatting and comments are preserved.

## What's Next?

### Future Releases

- **Slice 2 (CI/Automation):** `--non-interactive`, `--json`, secret-safe logging
- **Slice 3+ (Multi-Platform):** Linux, Web, macOS/iOS, Android (same `gst` command)

### Contributing

Want to help? See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Architecture overview
- How to add a new platform
- Testing and development workflow
- Code conventions and safety patterns
