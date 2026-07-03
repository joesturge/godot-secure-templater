# Godot Secure Templater (gst)

**Secure encrypted Godot export templates, without the toolchain hassle.**

<img width="735" height="342" alt="image" src="https://github.com/user-attachments/assets/3f19bdc2-1719-4cc4-a3bc-ada1d6d9cde2" />

`gst` automates provisioning, compilation, and configuration of encrypted Godot export templates—all in an isolated workspace within your project. No manual C++ toolchain setup. No Python dependency hell. No SCons config. Just one command.

```bash
$ gst create
```

## What It Does

### 🔒 Encrypted Godot Exports
- Generates an AES-256 encryption key for your project
- Compiles Godot templates with `SCRIPT_AES256_ENCRYPTION_KEY` baked in
- Wires the key into your export settings
- **Result:** Your game scripts and resources are encrypted in the final binary

### 🛠️ Automated Toolchain
- Downloads and verifies Python, MinGW, SCons, and Godot source automatically
- Runs everything in `.gst/` (isolated workspace)
- Cleans up after build (optional `--keep-runtime` for debugging)
- **Result:** No manual compiler installation ever

### 🚀 One Command to Wire Project Secrets
Run `gst create` whenever you need to compile templates or re-apply export wiring.

Use one shared project key distributed securely (for example via your secret manager or CI).

```
📋 Note for teammates:
  Treat the encryption key as a project secret.
  Share it securely via team secret management or CI secret injection.
  Do not commit secret material to source control.
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

### From GitHub Releases (recommended)

1. Open the [latest release on GitHub](https://github.com/joesturge/godot-secure-templater/releases).
2. Download the asset for your OS/architecture.
3. Rename or move it to a location on your PATH.

Release asset names:
- `gst-windows-amd64.exe`
- `gst-windows-arm64.exe`
- `gst-linux-amd64`
- `gst-linux-arm64`
- `gst-darwin-amd64`
- `gst-darwin-arm64`

Which one should I download?
- Most modern Windows PCs (Intel/AMD 64-bit): `gst-windows-amd64.exe`
- Windows on ARM devices: `gst-windows-arm64.exe`
- Most modern Linux PCs (Intel/AMD 64-bit): `gst-linux-amd64`
- Linux on ARM64 devices: `gst-linux-arm64`
- macOS on Apple Silicon (M1/M2/M3): `gst-darwin-arm64`
- macOS on older Intel Macs: `gst-darwin-amd64`

Windows (PowerShell):

```powershell
# Example for v0.1.0 (update version as needed)
$version = "v0.1.0"
$url = "https://github.com/joesturge/godot-secure-templater/releases/download/$version/gst-windows-amd64.exe"
Invoke-WebRequest -Uri $url -OutFile gst.exe
```

### From Source

```bash
git clone https://github.com/joesturge/godot-secure-templater.git
cd godot-secure-templater
mkdir -p dist
go build -o dist/gst ./cmd/gst
sudo mv dist/gst /usr/local/bin/  # or add to PATH
```

For contributor and release build commands, see [CONTRIBUTING.md](CONTRIBUTING.md).

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

### 1. Open your project and create an export preset in Godot

1. Open your project in Godot Editor.
2. Go to **Project → Export**.
3. Create or select a **Windows Desktop** preset.
4. Save and close the export dialogue.

This creates/updates `export_presets.cfg`, which `gst` will wire automatically.

### 2. Run gst to populate templates and encryption key

```bash
cd /path/to/your/game
gst create
```

`gst` will:
- compile (or cache-hit) encrypted Windows templates
- populate custom template paths in `export_presets.cfg`
- write your project encryption key to `.godot/export_credentials.cfg` (Godot 4.3+)

**Flags:**
- `--godot-version VERSION` (required) — Godot version to compile (must match project)
- `--verbose` — Show detailed build output
- `--keep-runtime` — Preserve toolchain after build (useful for repeated builds or debugging)
- `--force-rebuild` — Skip cache; always recompile templates
- `--regenerate-key` — Generate a new encryption key (requires confirmation)
- `--force` — Skip all confirmations (CI/automation mode)

### 3. Export your game

1. Open **Project → Export** again.
2. Confirm the **Windows Desktop** preset is selected.
3. Export the project.
4. Your build uses the injected custom templates and encryption key.

---

## What Gets Created

Inside your project:

```
my-game/
├── .gst/                          # Isolated workspace (add to .gitignore)
│   ├── runtime/                   # Toolchain (Python, MinGW, SCons, Godot source)
│   ├── templates/                 # Compiled export templates (windows_release.exe, etc.)
│   ├── encryption.key             # Project encryption key (owner-read-only)
│   └── manifest.json              # Build metadata for caching
├── .godot/
│   └── export_credentials.cfg     # Where the encryption key is wired
├── export_presets.cfg             # Export settings (auto-updated)
└── project.godot                  # Your project (unchanged)
```

**Important:** Add `.gst/` to your `.gitignore`—it is generated locally and should not be committed.

```bash
echo ".gst/" >> .gitignore
```

The encryption key (`encryption.key`) is **owner-read-only** (permissions `0600`) and should be treated as a project secret.

---

## Workflow for Teams

### Shared Key Setup

1. Generate and store a single project key in your team's secret manager.
2. Ensure each development environment and CI job receives that same key securely.
3. Run `gst create` to compile templates and wire export settings using the shared key.

### Import the shared key on another developer machine

1. Retrieve the project key from your team's secret manager.
2. Write it to `.gst/encryption.key` in the game project root.
3. Run `gst create` (without `--regenerate-key`).

Windows PowerShell:

```powershell
New-Item -ItemType Directory -Force .gst | Out-Null
Set-Content -Path .gst/encryption.key -Value $env:GST_ENCRYPTION_KEY -NoNewline
./gst.exe create --verbose
```

Linux/macOS:

```bash
mkdir -p .gst
printf '%s' "$GST_ENCRYPTION_KEY" > .gst/encryption.key
gst create --verbose
```

Notes:
- Keep the key byte-for-byte identical across dev and CI.
- Avoid trailing newline changes when writing the file.
- Never commit `.gst/encryption.key`.

### CI usage

In CI, inject the same project key from your secret store and write `.gst/encryption.key` before running `gst create`.

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
✅ New key generated. Rotate and redistribute the new key securely before the next team/CI export.
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

**Keys must be shared only through secure secret-management channels, never via source control.**

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
