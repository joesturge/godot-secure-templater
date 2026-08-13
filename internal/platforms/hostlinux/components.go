package hostlinux

import (
	"fmt"

	"github.com/joemi/godot-secure-templater/internal"
	platformhelpers "github.com/joemi/godot-secure-templater/internal/platforms"
)

// pkgConfigStub is a minimal pkg-config wrapper written into the provisioned
// toolchain bin directory so that Godot's linuxbsd SCons checks never fall
// back to the host's pkg-config binary. It reports all packages as NOT present
// (exit 1) so that optional system integrations (fontconfig, PulseAudio, udev,
// X11, Wayland) are automatically disabled by SCons rather than enabled with
// missing compile/link flags that would break the actual compile.
const pkgConfigStub = `#!/bin/sh
# Provisioned pkg-config stub – part of the gst toolchain.
# Reports all packages as not found so that Godot's linuxbsd SCons feature
# checks disable optional system integrations for a self-contained Zig build.
if [ "$1" = "--version" ]; then
	echo "0.0.0"
	exit 0
fi

if [ "$1" = "--help" ]; then
	exit 0
fi

exit 1
`

// Components returns the toolchain components for a Linux target on a Linux host.
func Components(version string) []internal.Artifact {
	releaseTag := platformhelpers.GodotReleaseTagForVersion(version)
	return []internal.Artifact{
		{
			Name:      "pkg-config",
			ExtractTo: "bin",
			Kind:      internal.ArchiveScript,
			Content:   pkgConfigStub,
		},
		{
			Name:      "python",
			URL:       "https://github.com/astral-sh/python-build-standalone/releases/download/20260623/cpython-3.11.15%2B20260623-x86_64-unknown-linux-gnu-install_only.tar.gz",
			SHA256:    "60295e3e703b48c270e8d8c685195b8d5c2f0b8a596c1a910d7e24a2cc55afdd",
			ExtractTo: "python",
			Kind:      internal.ArchiveTarGZ,
		},
		{
			Name:      "zig",
			URL:       "https://ziglang.org/download/0.16.0/zig-x86_64-linux-0.16.0.tar.xz",
			SHA256:    "70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00",
			ExtractTo: "zig",
			Kind:      internal.ArchiveTarXZ,
		},
		{
			Name:      "scons",
			URL:       "https://github.com/SCons/scons/releases/download/4.4.0/scons-4.4.0.tar.gz",
			SHA256:    "7703c4e9d2200b4854a31800c1dbd4587e1fa86e75f58795c740bcfa7eca7eaa",
			ExtractTo: "scons",
			Kind:      internal.ArchiveTarGZ,
		},
		{
			Name:      "godot_source",
			URL:       fmt.Sprintf("https://github.com/godotengine/godot/archive/refs/tags/%s.tar.gz", releaseTag),
			SHA256:    "",
			ExtractTo: "godot_source",
			Kind:      internal.ArchiveTarGZ,
		},
	}
}
