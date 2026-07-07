package platforms

import (
	"fmt"
	"strings"
)

// GodotReleaseTagForVersion maps a semantic version to the upstream Godot release tag.
func GodotReleaseTagForVersion(version string) string {
	version = strings.TrimSpace(version)
	parts := strings.Split(version, ".")
	if len(parts) >= 3 && parts[2] == "0" {
		return fmt.Sprintf("%s.%s-stable", parts[0], parts[1])
	}
	return fmt.Sprintf("%s-stable", version)
}
