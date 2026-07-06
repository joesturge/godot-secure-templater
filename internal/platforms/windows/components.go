package windows

import (
	"fmt"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/toolchain"
)

var resolveGodotChecksum = toolchain.GodotChecksumForVersion

// Components returns the toolchain components for a Windows target.
func Components(version string) []internal.Artifact {
	return []internal.Artifact{
		{
			Name:      "python",
			URL:       "https://www.python.org/ftp/python/3.11.0/python-3.11.0-embed-amd64.zip",
			SHA256:    "68fb03784e8545c35bcb5f240b696e6e676ca3e5fb90926ed0673d564299fb94",
			ExtractTo: "python",
			Kind:      internal.ArchiveZip,
		},
		{
			Name:      "mingw",
			URL:       "https://github.com/niXman/mingw-builds-binaries/releases/download/14.2.0-rt_v12-rev0/x86_64-14.2.0-release-posix-seh-ucrt-rt_v12-rev0.7z",
			SHA256:    "0f1afc3b48f66dda68fbfb7b8b0f1d22b831396fbe1e3dea776745f32d930b24",
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
			URL:       fmt.Sprintf("https://github.com/godotengine/godot/archive/refs/tags/%s-stable.tar.gz", version),
			SHA256:    resolveGodotChecksum(version),
			ExtractTo: "godot_source",
			Kind:      internal.ArchiveTarGZ,
		},
	}
}
