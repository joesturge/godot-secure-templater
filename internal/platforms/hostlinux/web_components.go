package hostlinux

import (
	"fmt"

	"github.com/joemi/godot-secure-templater/internal"
	platformhelpers "github.com/joemi/godot-secure-templater/internal/platforms"
)

func webComponents(version string) []internal.Artifact {
	releaseTag := platformhelpers.GodotReleaseTagForVersion(version)
	return []internal.Artifact{
		{
			Name:      "emsdk",
			URL:       "https://github.com/emscripten-core/emsdk/archive/refs/tags/4.0.0.tar.gz",
			SHA256:    "c916f67615cb08db6e5a9a6c8a198bf6c41613036119db2cd961ad0ac45d7b9c",
			ExtractTo: "emsdk",
			Kind:      internal.ArchiveTarGZ,
		},
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
			Name:      "scons",
			URL:       "https://github.com/SCons/scons/releases/download/4.4.0/scons-4.4.0.tar.gz",
			SHA256:    "7703c4e9d2200b4854a31800c1dbd4587e1fa86e75f58795c740bcfa7eca7eaa",
			ExtractTo: "scons",
			Kind:      internal.ArchiveTarGZ,
		},
		{
			Name:      "godot_source",
			URL:       fmt.Sprintf("https://github.com/godotengine/godot/archive/refs/tags/%s.tar.gz", releaseTag),
			ExtractTo: "godot_source",
			Kind:      internal.ArchiveTarGZ,
		},
	}
}
