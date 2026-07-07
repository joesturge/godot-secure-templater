package toolchain

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var checksumHTTPClient = &http.Client{Timeout: 15 * time.Second}

// GodotReleaseTagForVersion returns the GitHub release tag for a Godot version.
// Feature releases use tags like 4.7-stable; maintenance releases use 4.6.3-stable.
func GodotReleaseTagForVersion(version string) string {
	version = strings.TrimSpace(version)
	parts := strings.Split(version, ".")
	if len(parts) >= 3 && parts[2] == "0" {
		return fmt.Sprintf("%s.%s-stable", parts[0], parts[1])
	}
	return fmt.Sprintf("%s-stable", version)
}

// GodotChecksumForVersion returns a checksum for a Godot version from GitHub release metadata.
func GodotChecksumForVersion(version string) string {
	return fetchGodotChecksumFromGitHub(version)
}

func fetchGodotChecksumFromGitHub(version string) string {
	releaseTag := GodotReleaseTagForVersion(version)
	checksumsURL := fmt.Sprintf("https://github.com/godotengine/godot/releases/download/%s/SHA256-checksums.txt", releaseTag)

	return fetchGodotChecksumFromURL(checksumsURL, releaseTag)
}

func fetchGodotChecksumFromURL(checksumsURL, releaseTag string) string {
	resp, err := checksumHTTPClient.Get(checksumsURL)
	if err != nil {
		return ""
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	targetName := fmt.Sprintf("godot-%s.tar.gz", releaseTag)
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

	if scanner.Err() != nil {
		return ""
	}

	return ""
}
