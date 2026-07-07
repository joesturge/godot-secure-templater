package hostwindows

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
			Name:      "zig",
			URL:       "https://ziglang.org/download/0.16.0/zig-x86_64-windows-0.16.0.zip",
			SHA256:    "68659eb5f1e4eb1437a722f1dd889c5a322c9954607f5edcf337bc3684a75a7e",
			ExtractTo: "zig",
			Kind:      internal.ArchiveZip,
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
