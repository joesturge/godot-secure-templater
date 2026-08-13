package hostlinux

import (
	"fmt"

	"github.com/joemi/godot-secure-templater/internal"
	platformhelpers "github.com/joemi/godot-secure-templater/internal/platforms"
)

// windowsCrossComponents returns the toolchain components for building Windows
// templates from a Linux host using the provisioned MinGW toolchain.
func windowsCrossComponents(version string) []internal.Artifact {
	releaseTag := platformhelpers.GodotReleaseTagForVersion(version)
	return []internal.Artifact{
		{
			Name:      "python",
			URL:       "https://github.com/astral-sh/python-build-standalone/releases/download/20260623/cpython-3.11.15%2B20260623-x86_64-unknown-linux-gnu-install_only.tar.gz",
			SHA256:    "60295e3e703b48c270e8d8c685195b8d5c2f0b8a596c1a910d7e24a2cc55afdd",
			ExtractTo: "python",
			Kind:      internal.ArchiveTarGZ,
		},
		{
			Name:      "mingw",
			URL:       "https://github.com/mstorsjo/llvm-mingw/releases/download/20260616/llvm-mingw-20260616-ucrt-ubuntu-22.04-x86_64.tar.xz",
			SHA256:    "534b92e067b22a6b4441f48ae9240a3341b17825d04d577eab0cf85c44b4deda",
			ExtractTo: "mingw",
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
