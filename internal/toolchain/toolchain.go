package toolchain

import (
	"archive/tar"
	"archive/zip"
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
	"time"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/ulikunitz/xz"
)

const artifactDownloadTimeout = 20 * time.Minute

// Provision downloads and verifies toolchain components, extracting them into runtime/.
func Provision(ctx *internal.RunContext, components []internal.Artifact) *internal.Error {
	ctx.Logger.Info("Provisioning toolchain for Godot %s...", ctx.Godot.Patch)

	if err := EnsureSufficientDiskSpace(ctx.Workspace.Root, minimumRequiredDiskBytes); err != nil {
		return err
	}

	for _, art := range components {
		ctx.Logger.Info("  → %s", art.Name)

		targetDir := filepath.Join(ctx.Workspace.Runtime, art.ExtractTo)

		// Script artifacts are written directly to disk without downloading.
		if art.Kind == internal.ArchiveScript {
			if err := provisionScript(targetDir, art.Name, art.Content); err != nil {
				return &internal.Error{
					Code:    internal.ExitGenericFailure,
					Message: fmt.Sprintf("Failed to write script artifact: %s", art.Name),
					Details: err.Error(),
				}
			}
			ctx.Logger.Info("    ✓ Provisioned successfully")
			continue
		}

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
		ctx.Logger.Info("    Downloading archive...")
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

		if art.SHA256 == "" {
			if art.Name == "godot_source" {
				ctx.Logger.Debug("Skipping checksum verification for %s (checksum not provided)", art.Name)
			} else {
				return &internal.Error{
					Code:    internal.ExitChecksumMismatch,
					Message: fmt.Sprintf("No checksum available for %s", art.Name),
					Details: "Failed to resolve checksum metadata for this artifact. Aborting to avoid unverified downloads.",
				}
			}
		} else {
			ctx.Logger.Debug("Verifying checksum for %s", art.Name)
			if err := VerifyChecksum(archivePath, art.SHA256); err != nil {
				return err
			}
		}

		// Extract archive
		ctx.Logger.Info("    Extracting archive...")
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

		// Special handling for Python: enable PYTHONPATH support in the embedded distribution.
		// The Windows embedded zip ships a python*._pth file that disables PYTHONPATH; without
		// patching it, subprocesses like em++.py cannot import sibling modules such as emcc.
		if art.Name == "python" {
			if patchErr := enableSiteForEmbeddedPython(targetDir); patchErr != nil {
				ctx.Logger.Warn("Failed to patch Python path config to enable PYTHONPATH: %v", patchErr)
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
		// Support both extracted layouts:
		// 1) top-level godot-* folder
		// 2) stripped archive root containing source tree files directly
		knownRootMarkers := map[string]bool{
			"SConstruct": true,
			"version.py": true,
			"core":       true,
			"platform":   true,
		}

		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "godot-") {
				return true
			}
			if knownRootMarkers[entry.Name()] {
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
	client := &http.Client{Timeout: artifactDownloadTimeout}

	resp, err := client.Get(url)
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
		return extractTarXZ(archivePath, targetDir)
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

// extractTarXZ extracts a tar.xz archive in a single pass.
//
// Unlike tar.gz extraction, this path intentionally does not strip a single top-level
// directory because doing so requires a second full decompression pass, which is costly
// for larger tar.xz payloads such as Zig toolchains.
func extractTarXZ(xzPath, targetDir string) error {
	file, err := os.Open(xzPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	xzr, err := xz.NewReader(file)
	if err != nil {
		return err
	}

	tr := tar.NewReader(xzr)
	cleanTargetDir := filepath.Clean(targetDir)
	type pendingSymlink struct {
		path     string
		linkname string
	}
	var symlinks []pendingSymlink
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fpath := filepath.Clean(filepath.Join(cleanTargetDir, header.Name))
		if !strings.HasPrefix(fpath, cleanTargetDir+string(os.PathSeparator)) {
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
		case tar.TypeSymlink:
			if filepath.IsAbs(header.Linkname) {
				continue
			}
			linkTarget := filepath.Clean(filepath.Join(filepath.Dir(fpath), header.Linkname))
			if !pathWithinDirectory(cleanTargetDir, linkTarget) {
				continue
			}
			symlinks = append(symlinks, pendingSymlink{path: fpath, linkname: header.Linkname})
		case tar.TypeLink:
			continue
		}
	}

	for _, symlink := range symlinks {
		linkTarget := filepath.Clean(filepath.Join(filepath.Dir(symlink.path), symlink.linkname))
		resolvedTarget, err := filepath.EvalSymlinks(linkTarget)
		if err != nil || !pathWithinDirectory(cleanTargetDir, resolvedTarget) {
			continue
		}
		if _, err := os.Lstat(symlink.path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(symlink.path), os.ModePerm); err != nil {
			return err
		}
		if err := os.Symlink(symlink.linkname, symlink.path); err != nil {
			return err
		}
	}

	return nil
}

func pathWithinDirectory(directory string, path string) bool {
	cleanDirectory := filepath.Clean(directory)
	cleanPath := filepath.Clean(path)
	return cleanPath != cleanDirectory && strings.HasPrefix(cleanPath, cleanDirectory+string(os.PathSeparator))
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
	pythonCandidates := []string{
		filepath.Join(ctx.Workspace.Runtime, "python", "python.exe"),
		filepath.Join(ctx.Workspace.Runtime, "python", "python"),
		filepath.Join(ctx.Workspace.Runtime, "python", "bin", "python"),
		filepath.Join(ctx.Workspace.Runtime, "python", "bin", "python3"),
	}
	var pythonExe string
	for _, candidate := range pythonCandidates {
		if _, err := os.Stat(candidate); err == nil {
			pythonExe = candidate
			break
		}
	}
	// Fallback: glob for versioned python3.* binaries (e.g. python3.11) in bin/.
	// python-build-standalone symlinks (python → python3.11) are not materialised during
	// extraction, so the versioned binary may be the only executable present.
	if pythonExe == "" {
		matches, _ := filepath.Glob(filepath.Join(ctx.Workspace.Runtime, "python", "bin", "python3*"))
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				pythonExe = match
				break
			}
		}
	}

	// Verify python exists
	if pythonExe == "" {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Python not found for SCons installation",
			Details: fmt.Sprintf("no python executable found under %s", filepath.Join(ctx.Workspace.Runtime, "python")),
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

// enableSiteForEmbeddedPython patches any python*._pth files found in pythonDir to uncomment
// "#import site", enabling PYTHONPATH support in the Windows embedded Python distribution.
// Without this, the embedded Python ignores PYTHONPATH entirely, causing em++.py to fail with
// "ModuleNotFoundError: No module named 'emcc'" when SCons invokes it as a subprocess.
// This is a no-op if no ._pth files exist (e.g. on POSIX hosts).
func enableSiteForEmbeddedPython(pythonDir string) error {
	matches, err := filepath.Glob(filepath.Join(pythonDir, "python*._pth"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	for _, pthFile := range matches {
		data, err := os.ReadFile(pthFile)
		if err != nil {
			continue
		}
		patched := strings.ReplaceAll(string(data), "#import site", "import site")
		if patched == string(data) {
			continue
		}
		if writeErr := os.WriteFile(pthFile, []byte(patched), 0644); writeErr != nil {
			return fmt.Errorf("failed to patch %s to enable PYTHONPATH: %w", pthFile, writeErr)
		}
	}
	return nil
}

// provisionScript writes a script artifact directly to targetDir/<name> with 0755 permissions.
func provisionScript(targetDir, name, content string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}
	dest := filepath.Join(targetDir, name)
	if err := os.WriteFile(dest, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write script %s: %w", dest, err)
	}
	// Explicitly chmod to ensure the executable bit is set even when the file
	// already existed with non-executable permissions before this call.
	if err := os.Chmod(dest, 0755); err != nil {
		return fmt.Errorf("failed to set permissions on script %s: %w", dest, err)
	}
	return nil
}
