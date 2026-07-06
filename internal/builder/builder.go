package builder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joemi/godot-secure-templater/internal"
	"github.com/joemi/godot-secure-templater/internal/progress"
)

// BuildTarget represents a build variant.
type BuildTarget string

const (
	BuildRelease BuildTarget = "template_release"
	BuildDebug   BuildTarget = "template_debug"
)

// CompileTemplates compiles both release and debug templates using a platform-provided command.
func CompileTemplates(
	ctx *internal.RunContext,
	key string,
	buildCommand func(ctx *internal.RunContext, target BuildTarget, key string) (*exec.Cmd, *internal.Error),
	sourceTemplateName func(target BuildTarget) string,
	destinationTemplateName func(target BuildTarget) string,
) *internal.Error {
	if buildCommand == nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "compile spec is missing build command"}
	}
	if sourceTemplateName == nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "compile spec is missing source template naming"}
	}
	if destinationTemplateName == nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "compile spec is missing destination template naming"}
	}

	ctx.Logger.Info("Compiling Godot templates...")

	targets := []BuildTarget{BuildRelease, BuildDebug}
	for _, target := range targets {
		if err := compileSingle(ctx, key, target, buildCommand, sourceTemplateName, destinationTemplateName); err != nil {
			return err
		}
	}

	ctx.Logger.Info("Templates compiled successfully.")
	return nil
}

// compileSingle compiles a single template variant.
func compileSingle(
	ctx *internal.RunContext,
	key string,
	target BuildTarget,
	buildCommand func(ctx *internal.RunContext, target BuildTarget, key string) (*exec.Cmd, *internal.Error),
	sourceTemplateName func(target BuildTarget) string,
	destinationTemplateName func(target BuildTarget) string,
) *internal.Error {
	ctx.Logger.Info("  → Compiling %s...", target)

	cmd, err := buildCommand(ctx, target, key)
	if err != nil {
		return err
	}
	if cmd == nil || cmd.Dir == "" {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "compile command did not set working directory"}
	}

	stdout, stdoutErr := cmd.StdoutPipe()
	if stdoutErr != nil {
		return &internal.Error{Code: internal.ExitBuildFailed, Message: "Failed to setup stdout pipe", Details: stdoutErr.Error()}
	}

	stderr, stderrErr := cmd.StderrPipe()
	if stderrErr != nil {
		return &internal.Error{Code: internal.ExitBuildFailed, Message: "Failed to setup stderr pipe", Details: stderrErr.Error()}
	}

	if err := cmd.Start(); err != nil {
		return &internal.Error{Code: internal.ExitBuildFailed, Message: "Failed to start SCons build", Details: err.Error()}
	}

	go streamOutput(ctx.Logger, stdout, false)
	go streamOutput(ctx.Logger, stderr, true)

	if err := cmd.Wait(); err != nil {
		return &internal.Error{Code: internal.ExitBuildFailed, Message: fmt.Sprintf("SCons build failed for %s", target), Details: err.Error()}
	}

	ctx.Logger.Info("    ✓ %s compiled successfully", target)

	if err := moveTemplate(ctx, cmd.Dir, target, sourceTemplateName, destinationTemplateName); err != nil {
		return err
	}

	return nil
}

// streamOutput reads from a pipe and logs each line.
func streamOutput(logger interface{ Info(string, ...interface{}) }, reader io.ReadCloser, isError bool) {
	scanner := bufio.NewScanner(reader)
	parser := progress.NewParser()
	lastStage := progress.StageUnknown
	start := time.Now()

	for scanner.Scan() {
		line := scanner.Text()

		if !isError {
			stage := parser.ParseLine(line)
			if stage != lastStage {
				logger.Info("%s (elapsed: %s)", progress.FormatStageUpdate(stage), time.Since(start).Round(time.Second))
				lastStage = stage
			}
		}

		if isError {
			if loggerWithWarn, ok := logger.(interface{ Warn(string, ...interface{}) }); ok {
				loggerWithWarn.Warn(line)
			} else {
				logger.Info(line)
			}
		} else {
			logger.Info(line)
		}
	}
}

// moveTemplate moves the compiled template from the Godot build directory to templates/.
func moveTemplate(
	ctx *internal.RunContext,
	godotSrc string,
	target BuildTarget,
	sourceTemplateName func(target BuildTarget) string,
	destinationTemplateName func(target BuildTarget) string,
) *internal.Error {
	binDir := filepath.Join(godotSrc, "bin")
	srcName := sourceTemplateName(target)
	srcPath := filepath.Join(binDir, srcName)

	if _, err := os.Stat(srcPath); err != nil {
		var actualFiles []string
		if entries, dirErr := os.ReadDir(binDir); dirErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.Contains(entry.Name(), "template") {
					actualFiles = append(actualFiles, entry.Name())
				}
			}
		}

		return &internal.Error{
			Code:    internal.ExitGenericFailure,
			Message: fmt.Sprintf("Compiled template not found at %s", srcPath),
			Details: fmt.Sprintf("Expected: %s\nFound templates in bin/: %v", srcName, actualFiles),
		}
	}

	dstName := destinationTemplateName(target)
	dstPath := filepath.Join(ctx.Workspace.Templates, dstName)

	if err := os.MkdirAll(ctx.Workspace.Templates, 0755); err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: "Failed to create templates directory", Details: err.Error()}
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: fmt.Sprintf("Failed to open source template %s", srcName), Details: err.Error()}
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(dstPath)
	if err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: fmt.Sprintf("Failed to create destination template %s", dstName), Details: err.Error()}
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return &internal.Error{Code: internal.ExitGenericFailure, Message: fmt.Sprintf("Failed to copy template to %s", dstName), Details: err.Error()}
	}

	ctx.Logger.Info("    ✓ Copied %s to templates/", srcName)
	ctx.Logger.Debug("Template copied from %s to %s", srcPath, dstPath)
	return nil
}
