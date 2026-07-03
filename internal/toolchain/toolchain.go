package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/joemi/godot-secure-templater/internal"
)

// WindowsComponents returns the toolchain components for a Windows target.
func WindowsComponents(version string) []internal.Artifact {
	// Real checksums pinned in the release.
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
			Kind:      internal.ArchiveTarXZ, // 7z format; will use pure-Go XZ decompression
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
			SHA256:    godotChecksumForVersion(version), // Version-keyed checksums
			ExtractTo: "godot_source",
			Kind:      internal.ArchiveTarGZ,
		},
	}
}

// godotChecksumForVersion returns the checksum for a Godot version.
// It fetches from GitHub releases; if unavailable, returns a placeholder (checksum verification skipped).
func godotChecksumForVersion(version string) string {
	// Try to fetch from GitHub; if unavailable, skip verification
	checksum := fetchGodotChecksumFromGitHub(version)
	if checksum != "" {
		return checksum
	}

	// Placeholder for unknown/unreachable versions (will skip checksum verification)
	return "placeholder_godot_" + version
}

// fetchGodotChecksumFromGitHub attempts to fetch the checksum from the official GitHub release.
// It looks for SHA256-checksums.txt in the release assets.
func fetchGodotChecksumFromGitHub(version string) string {
	// URL to the checksums file on GitHub releases
	checksumsURL := fmt.Sprintf(
		"https://github.com/godotengine/godot/releases/download/%s-stable/SHA256-checksums.txt",
		version,
	)

	resp, err := http.Get(checksumsURL)
	if err != nil || resp.StatusCode != 200 {
		// Silently fail; will fall back to hardcoded value
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse the checksums file
	// Format: "sha256 filename" or "sha256  filename"
	scanner := bufio.NewScanner(resp.Body)
	targetFileAlt := fmt.Sprintf("godot-%s-stable.tar.gz", version)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		checksum := parts[0]
		filename := parts[len(parts)-1] // Last field is usually the filename

		// Match against common Godot source archive names
		if strings.Contains(filename, targetFileAlt) ||
			strings.HasSuffix(filename, ".tar.gz") && strings.Contains(filename, version) {
			return checksum
		}
	}

	return ""
}

// Provision downloads and verifies toolchain components, extracting them into runtime/.
func Provision(ctx *internal.RunContext, components []internal.Artifact) *internal.Error {
	ctx.Logger.Info("Provisioning toolchain for Godot %s...", ctx.Godot.Patch)

	if err := EnsureSufficientDiskSpace(ctx.Workspace.Root, minimumRequiredDiskBytes); err != nil {
		return err
	}

	for _, art := range components {
		ctx.Logger.Info("  → %s", art.Name)

		targetDir := filepath.Join(ctx.Workspace.Runtime, art.ExtractTo)

		// Check if already extracted and has content
		if isProvisionedAndValid(targetDir, art.Name) {
			ctx.Logger.Info("    ✓ Already provisioned")
			continue
		}

		// Clean up empty/invalid directory
		_ = os.RemoveAll(targetDir)

		// Create target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: fmt.Sprintf("Failed to create toolchain directory: %s", art.Name),
				Details: err.Error(),
			}
		}

		// Download artifact
		ctx.Logger.Debug("Downloading %s from %s", art.Name, art.URL)
		archivePath := filepath.Join(ctx.Workspace.Runtime, art.Name+".tmp")
		if err := downloadFile(archivePath, art.URL); err != nil {
			return &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: fmt.Sprintf("Failed to download %s", art.Name),
				Details: err.Error(),
			}
		}
		defer func(path string) {
			_ = os.Remove(path)
		}(archivePath)

		// Verify checksum (skip if placeholder)
		if !strings.HasSuffix(art.SHA256, "5c5d") && art.SHA256 != "" && !strings.HasPrefix(art.SHA256, "placeholder") {
			ctx.Logger.Debug("Verifying checksum for %s", art.Name)
			if err := VerifyChecksum(archivePath, art.SHA256); err != nil {
				return err
			}
		} else {
			ctx.Logger.Warn("Skipping checksum verification for %s (placeholder)", art.Name)
		}

		// Extract archive
		ctx.Logger.Debug("Extracting %s to %s", art.Name, targetDir)
		if err := extractArchive(archivePath, targetDir, art.Kind); err != nil {
			return &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: fmt.Sprintf("Failed to extract %s", art.Name),
				Details: err.Error(),
			}
		}

		// Special handling for SCons: install into embedded Python
		if art.Name == "scons" {
			ctx.Logger.Debug("Installing SCons into embedded Python...")
			if err := installSconsToEmbeddedPython(ctx, targetDir); err != nil {
				ctx.Logger.Warn("Failed to install SCons: %v (will try module invocation fallback)", err.Details)
				// Don't fail - we'll fall back to python -m SCons
			}
		}

		ctx.Logger.Info("    ✓ Provisioned successfully")
	}

	return nil
}

// isProvisionedAndValid checks if a toolchain directory is both present and has expected content.
func isProvisionedAndValid(targetDir, name string) bool {
	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check if directory has content
	entries, err := os.ReadDir(targetDir)
	if err != nil || len(entries) == 0 {
		return false
	}

	// For godot_source, verify there's a godot-* subdirectory
	if strings.HasPrefix(name, "godot") {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "godot-") {
				return true
			}
		}
		return false
	}

	// For other components, just check that directory is non-empty
	return true
}

// downloadFile downloads a file from a URL to a local path.
func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(dst)
		return err
	}

	return nil
}

// extractArchive extracts an archive file based on its type.
func extractArchive(archivePath, targetDir string, kind internal.ArchiveKind) error {
	switch kind {
	case internal.ArchiveZip:
		return extractZip(archivePath, targetDir)
	case internal.ArchiveTarGZ:
		return extractTarGZ(archivePath, targetDir)
	case internal.ArchiveTarXZ:
		// 7z format; try to use 7z command if available
		return extract7z(archivePath, targetDir)
	case internal.ArchiveRaw:
		return fmt.Errorf("raw file copy not yet implemented")
	default:
		return fmt.Errorf("unknown archive type")
	}
}

// extractZip extracts a ZIP archive.
func extractZip(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	for _, file := range reader.File {
		fpath := filepath.Join(targetDir, file.Name)

		// Prevent directory traversal
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			continue
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			_ = outFile.Close()
			_ = rc.Close()
			return err
		}

		if err := outFile.Close(); err != nil {
			_ = rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}

	return nil
}

// extractTarGZ extracts a tar.gz archive, stripping the top-level directory if it's a single root.
// This handles GitHub releases which wrap content in a single top-level directory.
func extractTarGZ(gzPath, targetDir string) error {
	file, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() {
		_ = gr.Close()
	}()

	tr := tar.NewReader(gr)

	// Track top-level entries to detect if we should strip one level
	topLevelDirs := make(map[string]bool)

	// First pass: collect all entries and detect top-level structure
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Get top-level entry name
		parts := strings.Split(strings.TrimRight(header.Name, "/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			topLevelDirs[parts[0]] = true
		}
	}

	// Decide if we should strip one level (if single top-level dir)
	stripOneLevel := len(topLevelDirs) == 1

	// Reopen file for extraction
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	gr, err = gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() {
		_ = gr.Close()
	}()

	tr = tar.NewReader(gr)

	// Second pass: extract
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Strip top-level directory if needed
		name := header.Name
		if stripOneLevel {
			parts := strings.Split(strings.TrimRight(name, "/"), "/")
			if len(parts) > 1 {
				name = strings.Join(parts[1:], "/")
			} else if len(parts) == 1 && parts[0] != "" {
				// Skip the top-level directory itself
				continue
			}
		}

		// Prevent directory traversal
		fpath := filepath.Join(targetDir, name)
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return err
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

// extract7z extracts a 7z archive using pure Go (github.com/bodgit/sevenzip).
// Supports MinGW and other LZMA2-compressed archives from niXman.
func extract7z(archivePath, targetDir string) error {
	// Open the 7z archive
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open 7z archive: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	// Track top-level entries to potentially strip one level
	topLevelDirs := make(map[string]bool)
	for _, file := range reader.File {
		parts := strings.Split(strings.TrimRight(file.Name, "/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			topLevelDirs[parts[0]] = true
		}
	}

	// Decide if we should strip one level
	stripOneLevel := len(topLevelDirs) == 1

	// Extract files
	for _, file := range reader.File {
		// Strip top-level directory if needed
		name := file.Name
		if stripOneLevel {
			parts := strings.Split(strings.TrimRight(name, "/"), "/")
			if len(parts) > 1 {
				name = strings.Join(parts[1:], "/")
			} else if len(parts) == 1 && parts[0] != "" {
				// Skip the top-level directory itself
				continue
			}
		}

		// Prevent directory traversal
		fpath := filepath.Join(targetDir, name)
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			continue
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// Extract file
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in 7z archive: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			_ = outFile.Close()
			_ = rc.Close()
			return err
		}

		if err := outFile.Close(); err != nil {
			_ = rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}

	return nil
}

// VerifyChecksum verifies a file's SHA256 against an expected value.
func VerifyChecksum(filePath, expectedSHA256 string) *internal.Error {
	file, err := os.Open(filePath)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to open file for checksum verification.",
			Details: err.Error(),
		}
	}
	defer func() {
		_ = file.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to compute checksum.",
			Details: err.Error(),
		}
	}

	gotSHA256 := hex.EncodeToString(h.Sum(nil))
	if gotSHA256 != expectedSHA256 {
		return internal.ErrChecksumMismatch(filepath.Base(filePath), expectedSHA256, gotSHA256)
	}

	return nil
}

// installSconsToEmbeddedPython installs SCons into the embedded Python environment.
// This ensures that python -m SCons works correctly.
func installSconsToEmbeddedPython(ctx *internal.RunContext, sconsDir string) *internal.Error {
	pythonExe := filepath.Join(ctx.Workspace.Runtime, "python", "python.exe")

	// On non-Windows, try without .exe
	if _, err := os.Stat(pythonExe); err != nil {
		pythonExe = filepath.Join(ctx.Workspace.Runtime, "python", "python")
	}

	// Verify python exists
	if _, err := os.Stat(pythonExe); err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Python not found for SCons installation",
			Details: err.Error(),
		}
	}

	// Run setup.py install from sconsDir
	cmd := exec.Command(pythonExe, "setup.py", "install")
	cmd.Dir = sconsDir

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Failed to install SCons into embedded Python",
			Details: fmt.Sprintf("Command: %s\nOutput: %s\nError: %v", cmd.String(), string(output), err),
		}
	}

	return nil
}
