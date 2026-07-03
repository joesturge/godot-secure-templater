package longpath

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Checker inspects long-path handling capabilities on Windows.
type Checker struct {
	// Platform: "windows" or other (stub will only check Windows in real implementation)
	Platform string
}

// NewChecker creates a long-path checker for the current platform.
func NewChecker(platform string) *Checker {
	return &Checker{
		Platform: platform,
	}
}

// MaxPathLength returns the maximum path length before extended-length prefixes are needed (Windows: 260, others: varies).
func (c *Checker) MaxPathLength() int {
	switch c.Platform {
	case "windows":
		return 260 // MAX_PATH on Windows
	default:
		return 4096 // Typical on POSIX systems
	}
}

// CheckPath validates that a path is within acceptable length limits.
// On Windows, returns a warning if the path approaches MAX_PATH (260).
// Returns an error only if the path definitively exceeds limits.
func (c *Checker) CheckPath(path string) (warning string, err error) {
	length := len(path)
	max := c.MaxPathLength()

	if length > max {
		return "", fmt.Errorf(
			"path exceeds %d character limit: %d characters (%q)",
			max, length, path,
		)
	}

	// Windows-specific: warn if approaching limit (within 10%)
	if c.Platform == "windows" && length > int(float64(max)*0.9) {
		return fmt.Sprintf(
			"warning: path is %d/%d characters; nearing Windows MAX_PATH limit. "+
				"Enable Long Path support in Windows registry or move project to shorter path.",
			length, max,
		), nil
	}

	return "", nil
}

// ExtendedLengthPath returns the Windows-specific extended-length path prefix format.
// On Windows, prepends "\\?\" to allow paths up to 32,767 characters.
// On other platforms, returns the path unchanged.
func (c *Checker) ExtendedLengthPath(path string) string {
	if c.Platform != "windows" {
		return path
	}

	// Normalize to absolute path and add extended-length prefix
	// In a real implementation, this would handle UNC paths (\\server\share) specially.
	if len(path) > 1 && path[0] != '\\' {
		// Local path: prepend \\?\
		return "\\\\?\\" + path
	}

	// Already UNC or extended: return as-is
	return path
}

// DiagnosticMessage returns a user-friendly explanation of long-path requirements on Windows.
func (c *Checker) DiagnosticMessage() string {
	if c.Platform != "windows" {
		return ""
	}

	return `Long path support on Windows:
  • Paths longer than 260 characters require Windows Long Path support
  • Enable in Group Policy: gpedit.msc → Computer Config → Admin Templates → System → Filesystem
    → "Enable Win32 long paths" (Set to 'Enabled')
  • OR via Registry: HKLM\SYSTEM\CurrentControlSet\Control\FileSystem → LongPathsEnabled = 1
  • OR move your project to a shorter path (reduce directory nesting)`
}

// IsLongPathsEnabled checks the Windows registry setting LongPathsEnabled.
// Returns false,nil on non-Windows platforms.
func (c *Checker) IsLongPathsEnabled() (bool, error) {
	if c.Platform != "windows" {
		return false, nil
	}

	if runtime.GOOS != "windows" {
		return false, nil
	}

	cmd := exec.Command(
		"reg",
		"query",
		`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`,
		"/v",
		"LongPathsEnabled",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	text := strings.ToLower(string(output))
	return strings.Contains(text, "0x1") || strings.Contains(text, "0x00000001"), nil
}

// NeedsPrefixing checks if a path needs the extended-length prefix.
// Returns true if the path is on Windows and might exceed MAX_PATH after manipulation.
func (c *Checker) NeedsPrefixing(path string) bool {
	if c.Platform != "windows" {
		return false
	}

	// If path is already close to or exceeds limit, prefixing will be needed
	return len(path) > 200 // Conservative threshold (leaves room for appended filenames)
}
