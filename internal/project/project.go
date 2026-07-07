package project

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joemi/godot-secure-templater/internal"
)

// Detect locates a Godot project by finding project.godot in the given directory.
func Detect(dir string) (string, *internal.Error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", &internal.Error{
			Code:    internal.ExitNotGodotProject,
			Message: "Could not resolve project path.",
			Details: err.Error(),
		}
	}

	projectPath := filepath.Join(absDir, "project.godot")
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return "", internal.ErrNotGodotProject
	}

	return absDir, nil
}

// ReadVersion reads the Godot version from project.godot's config/features line.
// It looks for PackedStringArray("4.3", ...) or similar and extracts the first semantic version.
func ReadVersion(projectPath string) (string, *internal.Error) {
	file, err := os.Open(filepath.Join(projectPath, "project.godot"))
	if err != nil {
		return "", &internal.Error{
			Code:    internal.ExitNotGodotProject,
			Message: "Could not open project.godot.",
			Details: err.Error(),
		}
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "config/features") {
			// Try to extract version from PackedStringArray("4.3", ...)
			re := regexp.MustCompile(`"(\d+\.\d+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	return "", internal.ErrProjectVersionUnreadable
}

// ValidateMinorLine ensures the supplied version's minor line matches the project's.
func ValidateMinorLine(projectMinor, suppliedVersion string) *internal.Error {
	// Extract minor line from supplied version (X.Y or X.Y.Z).
	parts := strings.Split(suppliedVersion, ".")
	if len(parts) < 2 {
		return &internal.Error{
			Code:    internal.ExitVersionResolution,
			Message: fmt.Sprintf("Invalid version format: %s (expected X.Y.Z)", suppliedVersion),
		}
	}
	suppliedMinor := parts[0] + "." + parts[1]

	if suppliedMinor != projectMinor {
		return internal.ErrMinorMismatch(projectMinor, suppliedMinor)
	}

	return nil
}

// EnsureGitignore ensures .gst/ is in .gitignore.
func EnsureGitignore(projectPath string) *internal.Error {
	gitignorePath := filepath.Join(projectPath, ".gitignore")
	toolDirEntry := ".gst/"

	// Read existing gitignore or create empty.
	var content []byte
	if info, err := os.Stat(gitignorePath); err == nil && !info.IsDir() {
		var err error
		content, err = os.ReadFile(gitignorePath)
		if err != nil {
			return &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: "Could not read .gitignore.",
				Details: err.Error(),
			}
		}
	}

	// Check if entry exists.
	if strings.Contains(string(content), toolDirEntry) {
		return nil
	}

	// Append.
	f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: "Could not write to .gitignore.",
			Details: err.Error(),
		}
	}
	defer func() {
		_ = f.Close()
	}()

	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return &internal.Error{Code: internal.ExitGenericFailure, Message: "Write failed."}
		}
	}

	if _, err := f.WriteString(toolDirEntry + "\n"); err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Write failed."}
	}

	return nil
}

// InitWorkspace creates the .gst/ structure.
func InitWorkspace(projectPath string) (*internal.Workspace, *internal.Error) {
	wsRoot := filepath.Join(projectPath, ".gst")

	dirs := []string{
		wsRoot,
		filepath.Join(wsRoot, "runtime"),
		filepath.Join(wsRoot, "templates"),
		filepath.Join(wsRoot, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, &internal.Error{
				Code:    internal.ExitGenericFailure,
				Message: fmt.Sprintf("Could not create workspace directory: %s", dir),
				Details: err.Error(),
			}
		}
	}

	return &internal.Workspace{
		Root:      wsRoot,
		Runtime:   filepath.Join(wsRoot, "runtime"),
		Templates: filepath.Join(wsRoot, "templates"),
		Logs:      filepath.Join(wsRoot, "logs"),
		Manifest:  filepath.Join(wsRoot, "manifest.json"),
		Lock:      filepath.Join(wsRoot, ".lock"),
		KeyFile:   filepath.Join(wsRoot, "encryption.key"),
	}, nil
}
