package internal

// ResolvedVersion represents a resolved Godot version.
type ResolvedVersion struct {
	Minor  string // e.g., "4.3"
	Patch  string // e.g., "4.3.2"
	Method string // e.g., "override"
	Source string // e.g., "command line flag"
}

// Workspace represents the tool's isolated workspace under .gst/
type Workspace struct {
	Root      string // <ProjectRoot>/.gst
	Runtime   string // Root/runtime
	Templates string // Root/templates
	Logs      string // Root/logs
	Manifest  string // Root/manifest.json
	Lock      string // Root/.lock
	KeyFile   string // Root/encryption.key
}

// Flags captures parsed CLI flags.
type Flags struct {
	GodotVersion string // --godot-version
	Platform     string // hard-wired "windows" in Slice 0
	KeepRuntime  bool   // --keep-runtime
	Interactive  bool   // negated by absence of stdin (Slice 2)
}

// RunContext threads state through the pipeline.
type RunContext struct {
	ProjectRoot string
	Workspace   *Workspace
	Godot       *ResolvedVersion
	Flags       *Flags
	Logger      Logger
}

// Logger defines the logging interface.
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Printf(format string, args ...interface{})
}

// Artifact describes a downloadable/verifiable component.
type Artifact struct {
	Name      string // e.g., "mingw", "python"
	URL       string
	SHA256    string
	ExtractTo string // subdir under runtime/
	Kind      ArchiveKind
}

// ArchiveKind enumerates supported archive formats.
type ArchiveKind int

const (
	ArchiveZip ArchiveKind = iota
	ArchiveTarXZ
	ArchiveTarGZ
	ArchiveRaw
)

// ProjectConfig represents parsed project.godot data.
type ProjectConfig struct {
	Version string // minor.patch
	Path    string
}

// ExportPresets represents the parsed export_presets.cfg.
type ExportPresets struct {
	Path     string
	RawLines []string // preserved for byte-preserving edits
}

// ExportCredentials represents the parsed .godot/export_credentials.cfg (Slice 0 targets 4.3+).
type ExportCredentials struct {
	Path     string
	RawLines []string
}
