package toolchain

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
)

// GodotChecksumForVersion returns a checksum for a Godot version from GitHub release metadata.
func GodotChecksumForVersion(version string) string {
	return fetchGodotChecksumFromGitHub(version)
}

func fetchGodotChecksumFromGitHub(version string) string {
	checksumsURL := fmt.Sprintf(
		"https://github.com/godotengine/godot/releases/download/%s-stable/SHA256-checksums.txt",
		version,
	)

	return fetchGodotChecksumFromURL(checksumsURL, version)
}

func fetchGodotChecksumFromURL(checksumsURL, version string) string {
	resp, err := http.Get(checksumsURL)
	if err != nil {
		return ""
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	targetName := fmt.Sprintf("godot-%s-stable.tar.gz", version)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		filename := parts[len(parts)-1]
		if filename == targetName {
			return parts[0]
		}
	}

	return ""
}
