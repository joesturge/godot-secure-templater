package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/builder"
	"github.com/joemi/godot-secure-templater/internal/cleanup"
	"github.com/joemi/godot-secure-templater/internal/config"
	"github.com/joemi/godot-secure-templater/internal/crypto"
	"github.com/joemi/godot-secure-templater/internal/manifest"
	"github.com/joemi/godot-secure-templater/internal/pipeline"
	"github.com/joemi/godot-secure-templater/internal/project"
	"github.com/joemi/godot-secure-templater/internal/toolchain"
)

const toolVersion = "dev"

var (
	rootCmd = &cobra.Command{
		Use:   "gst",
		Short: "Godot Secure Templater - provision secure export templates with encryption",
		Long: `Godot Secure Templater (gst) automates provisioning, compilation, and configuration of
secure (encrypted) Godot export templates natively inside a user's project directory.

Instead of manually installing a C++ toolchain, Python, SCons, and the Godot source tree,
this tool handles everything in an isolated .gst/ workspace.`,
	}

	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create and wire in encrypted export templates",
		Long: `Compile encrypted Godot export templates from source and wire them into the project's
export configuration. Requires --godot-version in Slice 0.`,
		RunE: runCreate,
	}

	cleanCmd = &cobra.Command{
		Use:   "clean",
		Short: "Remove .gst workspace artifacts",
		Long:  "Remove the generated .gst workspace, including runtime, templates, and key material.",
		RunE:  runClean,
	}

	// Flags.
	flagGodotVersion  string
	flagGodotEditorPath string
	flagKeepRuntime   bool
	flagForceRebuild  bool
	flagRegenerateKey bool
	flagForce         bool
	flagVerbose       bool
)

func init() {
	createCmd.Flags().StringVar(&flagGodotVersion, "godot-version", "", "Godot version (required, e.g., 4.3.2)")
	createCmd.Flags().StringVar(&flagGodotEditorPath, "godot-editor-path", "", "Path to Godot editor binary used for local version resolution")
	createCmd.Flags().BoolVar(&flagKeepRuntime, "keep-runtime", false, "Keep toolchain runtime after successful build")
	createCmd.Flags().BoolVar(&flagForceRebuild, "force-rebuild", false, "Skip idempotency check; always rebuild")
	createCmd.Flags().BoolVar(&flagRegenerateKey, "regenerate-key", false, "Generate new encryption key (requires confirmation unless --force)")
	createCmd.Flags().BoolVar(&flagForce, "force", false, "Skip all confirmations (for automation/CI)")
	createCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Verbose output")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(cleanCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Extract exit code if it's a typed error.
		if e, ok := err.(*internal.Error); ok {
			if e != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", e.Error())
				os.Exit(int(e.Code))
			}
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(int(internal.ExitGenericFailure))
	}
}

func runCreate(cmd *cobra.Command, args []string) error {
	logger := internal.NewSimpleLogger(flagVerbose)

	// ============================================================================
	// PREFLIGHT: Detect project, validate version, init workspace
	// ============================================================================

	logger.Info("Detecting Godot project...")
	cwd, err := os.Getwd()
	if err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Could not get working directory."}
	}

	projectRoot, projectErr := project.Detect(cwd)
	if projectErr != nil {
		logger.Error("Detect failed: %v", projectErr)
		return projectErr
	}
	logger.Info("Project root: %s", projectRoot)

	// Read the project's declared Godot version.
	logger.Info("Reading project version...")
	projectMinor, versionErr := project.ReadVersion(projectRoot)
	if versionErr != nil {
		logger.Error("ReadVersion failed: %v", versionErr)
		return versionErr
	}
	logger.Info("Project targets Godot %s", projectMinor)

	// Ensure .gst/ workspace exists.
	logger.Info("Initializing workspace...")
	workspace, wsErr := project.InitWorkspace(projectRoot)
	if wsErr != nil {
		logger.Error("InitWorkspace failed: %v", wsErr)
		return wsErr
	}

	// Ensure .gitignore includes .gst/.
	if err := project.EnsureGitignore(projectRoot); err != nil {
		logger.Warn("Could not update .gitignore: %v", err)
	}

	// Acquire lock to prevent concurrent runs mutating the same workspace.
	releaseLock, lockErr := acquireRunLock(workspace.Lock)
	if lockErr != nil {
		logger.Error("Failed to acquire run lock: %v", lockErr)
		return lockErr
	}
	defer releaseLock()

	// ============================================================================
	// BUILD PIPELINE ORCHESTRATOR
	// ============================================================================

	logger.Info("Building pipeline orchestrator...")
	opts := &pipeline.Options{
		ProjectRoot:   projectRoot,
		GodotVersion:  flagGodotVersion,
		GodotEditorPath: flagGodotEditorPath,
		ProjectMinor:  projectMinor,
		Platform:      "windows", // Hard-wired in Slice 0; Slice 3 will extend.
		KeepRuntime:   flagKeepRuntime,
		ForceRebuild:  flagForceRebuild,
		RegenerateKey: flagRegenerateKey,
		Force:         flagForce,
		Verbose:       flagVerbose,
	}
	orch := pipeline.NewOrchestrator(opts)

	// ============================================================================
	// PHASE 1: CHECK LONG PATHS (Fail Fast)
	// ============================================================================

	logger.Info("Checking Windows path length limits...")
	warnings, err := orch.CheckLongPaths()
	if err != nil {
		logger.Error("Path check failed: %v", err)
		return err
	}
	for _, w := range warnings {
		logger.Warn(w)
	}

	// ============================================================================
	// PHASE 2: RESOLVE VERSION (Strategy Chain)
	// ============================================================================

	logger.Info("Resolving version...")
	resolution, err := orch.ResolveVersion()
	if err != nil {
		logger.Error("Version resolution failed: %v", err)
		return err
	}
	logger.Info("Resolved version %s via %s (%s)", resolution.Version, resolution.Method, resolution.Source)

	if err := project.ValidateMinorLine(projectMinor, resolution.Version); err != nil {
		return err
	}
	logger.Info("Version validated against project minor line: %s", resolution.Version)

	// ============================================================================
	// PHASE 3: DETERMINE CONFIG ERA
	// ============================================================================

	logger.Info("Determining configuration era...")
	era, err := orch.DetermineConfigEra(resolution.Version)
	if err != nil {
		logger.Error("Era determination failed: %v", err)
		return err
	}
	logger.Info("Configuration era: %v", era)

	components := toolchain.WindowsComponents(resolution.Version)
	toolchainChecksums := buildToolchainChecksums(components)

	// ============================================================================
	// PHASE 4: CHECK IDEMPOTENCY
	// ============================================================================

	canSkip := false
	if !flagForceRebuild {
		canSkip = orch.CheckIdempotency(resolution, toolchainChecksums, toolVersion)
		if canSkip {
			logger.Info("Cache hit! Skipping rebuild.")
			logger.Info(orch.GetTeammateMessage())
			return nil
		}
		logger.Info("No matching manifest cache key found; continuing with rebuild")
	} else {
		logger.Info("Force rebuild requested; skipping cache check")
	}

	// ============================================================================
	// Build RunContext (Legacy for Slice 0 components)
	// ============================================================================

	ctx := &internal.RunContext{
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Godot: &internal.ResolvedVersion{
			Patch:  resolution.Version,
			Minor:  projectMinor,
			Method: string(resolution.Method),
			Source: resolution.Source,
		},
		Flags: &internal.Flags{
			GodotVersion: flagGodotVersion,
			Platform:     "windows",
			KeepRuntime:  flagKeepRuntime,
			Interactive:  !flagForce,
		},
		Logger: logger,
		Clock:  nil, // [TODO] Inject for testability.
		HTTP:   nil, // [TODO] Inject for testability.
	}

	// ============================================================================
	// PHASE 5: KEY REGENERATION & CRYPTO
	// ============================================================================

	logger.Info("Managing encryption key...")
	if flagRegenerateKey {
		if !flagForce {
			logger.Warn("Regenerating encryption key invalidates prior builds that embedded the old key.")
			logger.Warn("This key is MACHINE-SPECIFIC; each teammate must regenerate on their machine.")
			confirmed, confirmErr := confirmRegenerateKey()
			if confirmErr != nil {
				return &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to read confirmation input.", Details: confirmErr.Error()}
			}
			if !confirmed {
				return &internal.Error{Code: internal.ExitGenericFailure, Message: "Key regeneration cancelled by user."}
			}
		}
		logger.Info("Removing old key to trigger regeneration...")
		if removeErr := os.Remove(workspace.KeyFile); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warn("Could not remove old key file %s: %v", workspace.KeyFile, removeErr)
		}
	}

	key, keyErr := crypto.EnsureKey(workspace.KeyFile)
	if keyErr != nil {
		logger.Error("EnsureKey failed: %v", keyErr)
		return keyErr
	}
	logger.Info("Encryption key ready (reusing existing or generated new)")

	// ============================================================================
	// PHASE 6: TOOLCHAIN PROVISIONING
	// ============================================================================

	logger.Info("Provisioning toolchain...")
	if err := toolchain.Provision(ctx, components); err != nil {
		return err
	}

	// ============================================================================
	// PHASE 7: COMPILATION
	// ============================================================================

	logger.Info("Compiling templates...")
	if err := builder.CompileTemplates(ctx, key); err != nil {
		return err
	}

	// ============================================================================
	// PHASE 8: CONFIG INJECTION
	// ============================================================================

	logger.Info("Injecting configuration...")

	presetsPath := filepath.Join(projectRoot, "export_presets.cfg")
	if err := config.InjectWindowsTemplate(presetsPath,
		filepath.Join(workspace.Templates, "windows_template_release.exe"),
		filepath.Join(workspace.Templates, "windows_template_debug.exe")); err != nil {
		logger.Error("Config injection failed: %v", err)
		return err
	}

	credsPath, credPathErr := config.CredentialPath(projectRoot, era)
	if credPathErr != nil {
		return &internal.Error{Code: internal.ExitUnsupportedGodot, Message: "Could not determine credential path for this Godot version.", Details: credPathErr.Error()}
	}
	if err := config.InjectEncryptionKey(credsPath, key); err != nil {
		logger.Error("Credential injection failed: %v", err)
		return err
	}

	// ============================================================================
	// PHASE 9: WRITE MANIFEST & CLEANUP
	// ============================================================================

	logger.Info("Recording build manifest...")
	releaseTemplatePath := filepath.Join(workspace.Templates, "windows_template_release.exe")
	debugTemplatePath := filepath.Join(workspace.Templates, "windows_template_debug.exe")
	releaseHash, releaseHashErr := manifest.ComputeFileHash(releaseTemplatePath)
	if releaseHashErr != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to hash release template for manifest.", Details: releaseHashErr.Error()}
	}
	debugHash, debugHashErr := manifest.ComputeFileHash(debugTemplatePath)
	if debugHashErr != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to hash debug template for manifest.", Details: debugHashErr.Error()}
	}

	if err := orch.WriteManifest(
		resolution,
		opts.Platform,
		toolchainChecksums,
		toolVersion,
		true,
		releaseHash,
		debugHash,
	); err != nil {
		logger.Error("Manifest write failed: %v", err)
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to write build manifest.", Details: err.Error()}
	}

	logger.Info("Cleaning up build artifacts...")
	if err := orch.CleanupAfterSuccess(); err != nil {
		logger.Error("Cleanup failed: %v", err)
		return err
	}

	// ============================================================================
	// SUCCESS & TEAMMATE MESSAGE
	// ============================================================================

	logger.Info("Success! Encrypted templates compiled and configured.")
	logger.Info("")
	logger.Info(orch.GetTeammateMessage())
	logger.Info("")
	logger.Info("Next steps:")
	logger.Info("  1. Open your project in the Godot Editor")
	logger.Info("  2. Go to Project → Export")
	logger.Info("  3. Export your game using the Windows preset")
	logger.Info("")

	return nil
}

func runClean(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Could not get working directory."}
	}

	projectRoot, projectErr := project.Detect(cwd)
	if projectErr != nil {
		return projectErr
	}

	pruner := cleanup.NewPruner(projectRoot, false)
	if err := pruner.PruneManual(); err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to clean .gst workspace.", Details: err.Error()}
	}

	return nil
}

func buildToolchainChecksums(components []internal.Artifact) map[string]string {
	checksums := make(map[string]string)
	for _, component := range components {
		checksums[component.Name] = component.SHA256
	}
	return checksums
}

func confirmRegenerateKey() (bool, error) {
	if _, err := fmt.Fprint(os.Stdout, "Proceed with key regeneration? (y/N): "); err != nil {
		return false, err
	}
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	trimmed := strings.ToLower(strings.TrimSpace(input))
	return trimmed == "y" || trimmed == "yes", nil
}

func acquireRunLock(lockPath string) (func(), *internal.Error) {
	hostname, hostErr := os.Hostname()
	if hostErr != nil {
		hostname = "unknown"
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			pid := "unknown"
			host := "unknown"
			if data, readErr := os.ReadFile(lockPath); readErr == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "pid=") {
						pid = strings.TrimPrefix(line, "pid=")
					}
					if strings.HasPrefix(line, "host=") {
						host = strings.TrimPrefix(line, "host=")
					}
				}
			}
			return nil, internal.ErrLockHeld(pid, host)
		}

		return nil, &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to create run lock.", Details: err.Error()}
	}

	lockContents := fmt.Sprintf("pid=%d\nhost=%s\n", os.Getpid(), hostname)
	if _, writeErr := lockFile.WriteString(lockContents); writeErr != nil {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
		return nil, &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to write run lock metadata.", Details: writeErr.Error()}
	}

	release := func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}

	return release, nil
}
