package progress

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Stage represents a build stage (e.g., "Compiling core", "Linking").
type Stage string

const (
	StagePreparing Stage = "Preparing"
	StageCompiling Stage = "Compiling"
	StageLinking   Stage = "Linking"
	StageFinishing Stage = "Finishing"
	StageUnknown   Stage = "Building"
)

// Parser reads SCons build output and extracts staged progress.
type Parser struct {
	// lastStage tracks the most recent recognized stage to provide continuity.
	lastStage Stage

	// patterns map regex patterns to Stage identifiers.
	patterns map[*regexp.Regexp]Stage
}

// NewParser creates a progress parser with standard SCons patterns.
func NewParser() *Parser {
	p := &Parser{
		lastStage: StageUnknown,
	}

	p.patterns = map[*regexp.Regexp]Stage{
		regexp.MustCompile(`(?i)compiling.*core`):   StageCompiling,
		regexp.MustCompile(`(?i)compiling.*driver`): StageCompiling,
		regexp.MustCompile(`(?i)compiling.*module`): StageCompiling,
		regexp.MustCompile(`(?i)linking`):           StageLinking,
		regexp.MustCompile(`(?i)^scons: Finished`):  StageFinishing,
		regexp.MustCompile(`(?i)generating`):        StagePreparing,
	}

	return p
}

// ParseLine examines a single line of output and returns the detected stage (or the last known stage if no match).
func (p *Parser) ParseLine(line string) Stage {
	for pattern, stage := range p.patterns {
		if pattern.MatchString(line) {
			p.lastStage = stage
			return stage
		}
	}

	// No match; return the last known stage
	return p.lastStage
}

// ParseOutput reads a full output stream line-by-line and returns a summary of detected stages.
func (p *Parser) ParseOutput(output io.Reader) ([]Stage, error) {
	scanner := bufio.NewScanner(output)
	var stages []Stage
	lastStage := StageUnknown

	for scanner.Scan() {
		line := scanner.Text()
		stage := p.ParseLine(line)

		// Record unique stage transitions
		if stage != lastStage {
			stages = append(stages, stage)
			lastStage = stage
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error parsing output: %w", err)
	}

	return stages, nil
}

// FormatStageUpdate formats a user-friendly progress message.
func FormatStageUpdate(stage Stage) string {
	switch stage {
	case StagePreparing:
		return "⏳ Preparing build environment…"
	case StageCompiling:
		return "🔨 Compiling Godot…"
	case StageLinking:
		return "🔗 Linking…"
	case StageFinishing:
		return "✓ Finishing…"
	default:
		return "⚙ Building…"
	}
}

// SummarizeStages returns a human-readable summary of the stages detected.
func SummarizeStages(stages []Stage) string {
	if len(stages) == 0 {
		return "Build completed"
	}

	var summaryParts []string
	for _, stage := range stages {
		summaryParts = append(summaryParts, string(stage))
	}

	return fmt.Sprintf("Completed: %s", strings.Join(summaryParts, " → "))
}
