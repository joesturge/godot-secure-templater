# Godot Secure Templater (gst)

**Secure encrypted Godot export templates without the toolchain hassle.**

<img width="735" height="342" alt="image" src="https://github.com/user-attachments/assets/3f19bdc2-1719-4cc4-a3bc-ada1d6d9cde2" />

`gst` provisions and compiles encrypted Godot export templates in an isolated workspace within your project. No manual C++ toolchain setup, Python dependency hell, or SCons config. Just one command.

```bash
$ gst create
```

## What It Does

### 🔒 Encrypted Godot Exports
- Generates an AES-256 encryption key for your project
- Compiles Godot templates with `SCRIPT_AES256_ENCRYPTION_KEY` baked in
- Prints exact setup steps for your export preset and key placement
- **Result:** Your game scripts and resources are encrypted in the final binary

### 🛠️ Automated Toolchain
- Downloads Python, MinGW, SCons, and Godot source into an isolated workspace
- Runs in `.gst/` (isolated workspace)
- Cleans up after build; use `--keep-runtime` for debugging
- **Result:** No manual compiler installation ever

### 🚀 One Command to Build and Set Up
Run `gst create` whenever you need to compile templates and print the setup steps for Godot.

Use one shared project key, distributed securely via your secret manager or CI.

```
📋 Note for teammates:
  Treat the encryption key as a project secret.
  Share it securely via team secret management or CI secret injection.
  Do not commit secret material to source control.
```

### 🔄 Smart Rebuilds
- Caches build fingerprint (version, checksums, platform)
- Skips rebuild when inputs haven't changed
- Force rebuild with `--force-rebuild` if needed

### 📍 Platform-Ready
| Host                    | Target                                | Status                   | Notes                                                                                                                               |
| ----------------------- | ------------------------------------- | ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------- |
| Windows                 | Windows                               | Supported here           | Current end-to-end path.                                                                                                            |
| Linux / POSIX           | Linux                                 | Supported here           | Uses Godot's standard `linuxbsd` SCons flow on POSIX hosts.                                                                         |
| Windows / Linux / macOS | Android export templates              | Planned here             | Godot documents Android compilation on all three hosts with the Android SDK/NDK, but this repo does not implement it yet.           |
| Windows / Linux / macOS | Web export templates                  | Planned here             | Requires the Emscripten SDK; not implemented here yet.                                                                              |
| Windows / Linux / macOS | Web local serving / browser test loop | Planned here             | Godot's Web flow also expects a local web server for testing the generated output.                                                  |
| Linux / macOS           | Windows Desktop                       | Upstream-capable example | Godot documents Windows cross-compilation from Linux/macOS via MinGW or MinGW-LLVM, but this repo does not implement that path yet. |
| Windows                 | Linux                                 | Not planned (yet)        | Godot's standard `linuxbsd` path is not a normal Windows-hosted build flow.                                                         |

---

## Installation

### From GitHub Releases (recommended)

1. Open the [latest release on GitHub](https://github.com/joesturge/godot-secure-templater/releases).
2. Download the asset for your OS and CPU.
3. Move it onto your PATH.

Common downloads:
- Windows 64-bit: `gst-windows-amd64.exe`
- Windows ARM64: `gst-windows-arm64.exe`
- Linux 64-bit: `gst-linux-amd64`
- Linux ARM64: `gst-linux-arm64`
- macOS Intel: `gst-darwin-amd64`
- macOS Apple Silicon: `gst-darwin-arm64`

Windows (PowerShell):

```powershell
# Example for v0.1.0 (update version as needed)
$version = "v0.1.0"
$url = "https://github.com/joesturge/godot-secure-templater/releases/download/$version/gst-windows-amd64.exe"
Invoke-WebRequest -Uri $url -OutFile gst.exe
```

### From Source

Clone the repository, then use the host-appropriate build command in [CONTRIBUTING.md](CONTRIBUTING.md).

### Requires
- Go 1.21+
- Windows 10/11 host for Windows templates, or Linux/POSIX host for Linux templates
  - Python 3.11 runtime provisioned automatically
  - Zig 0.16.0 provisioned automatically
  - SCons 4.4.0 provisioned automatically
- Internet connection (first run downloads ~1GB of toolchain)
- 5+ GB disk space (toolchain + source + build artefacts)

---

## Quick Start

### 1. Create an export preset

Open your project in Godot, go to **Project → Export**, create or select the preset for the target you are building, then save `export_presets.cfg`.

`gst` prints the template paths you will paste into that preset.

### 2. Run `gst create`

```bash
cd /path/to/your/game
gst create
```

`gst` will compile or reuse the target templates, generate or reuse `.gst/encryption.key`, and print the remaining setup steps.

**Flags:**
- `--godot-version VERSION` (recommended) — explicit Godot version to compile. Accepts `X.Y` or `X.Y.Z` and must match the project minor line.
- If you omit `--godot-version`, `gst` tries a local Godot editor on `PATH` (or `--godot-editor-path`), then the latest stable release for the project minor line from GitHub, then prompts interactively.
- For CI or automation, always set `--godot-version` to avoid prompts and version drift.
- `--godot-editor-path PATH` — use this specific Godot editor binary for local version detection.
- `--platform TUPLE` — target platform tuple for the build. Today, the supported tuples are `windows/amd64` on Windows hosts and `linux/amd64` on Linux/POSIX hosts. Defaults to host if not supplied.
- `--verbose` — Show detailed build output
- `--keep-runtime` — Preserve toolchain after build (useful for repeated builds or debugging)
- `--force-rebuild` — Skip cache; always recompile templates
- `--regenerate-key` — Generate a new encryption key (requires confirmation)
- `--force` — Skip all confirmations (CI/automation mode)

### 3. Export your game

Open **Project → Export** again, confirm the preset that matches the templates you built, then export.

Your build uses the custom templates and encryption key you configured.

### 4. Clean up generated files

Use `gst clean` to remove the generated `.gst/` workspace, including runtime tools, compiled templates, and key material.

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
├── export_presets.cfg             # Export settings (you configure template paths)
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
3. Run `gst create` to compile templates, then apply the printed setup steps in Godot using the shared key.

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
- **Manifest:** Records version, platform, and method (for audit trail in Slice 3+)

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

### Manual Setup Guidance

After compilation, the tool prints the exact next steps:
- Set release template path to `.gst/templates/windows_template_release.exe`
- Set debug template path to `.gst/templates/windows_template_debug.exe`
- Copy the key from `.gst/encryption.key` into your preset or credentials field for your Godot version

The current branch does not auto-edit Godot config files.

## What's Next?

### Future Releases

- **Slice 2 (Multi-Platform):** Linux on POSIX hosts, Web with Emscripten, macOS/iOS, Android (same `gst` command, within upstream Godot host/toolchain constraints)
- **Slice 3 (CI/Automation):** `--non-interactive`, `--json`, secret-safe logging

### Contributing

Want to help? See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Architecture overview
- How to add a new platform
- Testing and development workflow
- Code conventions and safety patterns
