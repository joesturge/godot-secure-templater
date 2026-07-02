package progress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParserParseLine(t *testing.T) {
	// GIVEN various lines and patterns
	tests := []struct {
		name      string
		line      string
		wantStage Stage
	}{
		{
			name:      "compiling core module",
			line:      "Compiling core modules...",
			wantStage: StageCompiling,
		},
		{
			name:      "compiling drivers",
			line:      "Compiling drivers...",
			wantStage: StageCompiling,
		},
		{
			name:      "linking",
			line:      "Linking bin/godot.windows.opt.exe",
			wantStage: StageLinking,
		},
		{
			name:      "finishing",
			line:      "scons: Finished building targets.",
			wantStage: StageFinishing,
		},
		{
			name:      "generating",
			line:      "Generating cpp_bindings...",
			wantStage: StagePreparing,
		},
		{
			name:      "unrecognized line",
			line:      "some random output",
			wantStage: StageUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a fresh parser for each test (isolate state)
			parser := NewParser()

			// WHEN parsing the line
			got := parser.ParseLine(tt.line)

			// THEN the detected stage should match
			assert.Equal(t, tt.wantStage, got)
		})
	}
}

func TestParserStateTransition(t *testing.T) {
	// GIVEN a parser
	parser := NewParser()

	// WHEN parsing sequential lines
	stage1 := parser.ParseLine("Compiling core modules...")
	stage2 := parser.ParseLine("some intermediate output")
	stage3 := parser.ParseLine("Linking bin/godot.exe")

	// THEN stages should transition correctly
	assert.Equal(t, StageCompiling, stage1, "first line: compiling")
	assert.Equal(t, StageCompiling, stage2, "unrecognized: retain last stage")
	assert.Equal(t, StageLinking, stage3, "recognized: new stage")
}

func TestParserParseOutput(t *testing.T) {
	// GIVEN a full build output
	output := `Generating C++ bindings...
Compiling core modules...
core/main.cpp
core/config.cpp
Compiling drivers...
drivers/windows/opengl_context_wgl.cpp
Linking bin/godot.windows.opt.exe
scons: Finished building targets.`

	// WHEN parsing the full output
	parser := NewParser()
	stages, err := parser.ParseOutput(strings.NewReader(output))

	// THEN no error should occur
	assert.Nil(t, err)

	// AND stage transitions should be detected
	// (exact stages depend on patterns, but should have multiple stages)
	assert.Greater(t, len(stages), 0, "should detect at least one stage")
	assert.Contains(t, stages, StageCompiling, "should detect compiling stage")
	assert.Contains(t, stages, StageLinking, "should detect linking stage")
}

func TestFormatStageUpdate(t *testing.T) {
	// GIVEN various stages
	tests := []struct {
		name     string
		stage    Stage
		wantText string
	}{
		{
			name:     "preparing stage",
			stage:    StagePreparing,
			wantText: "Preparing",
		},
		{
			name:     "compiling stage",
			stage:    StageCompiling,
			wantText: "Compiling",
		},
		{
			name:     "linking stage",
			stage:    StageLinking,
			wantText: "Linking",
		},
		{
			name:     "finishing stage",
			stage:    StageFinishing,
			wantText: "Finishing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WHEN formatting the stage
			got := FormatStageUpdate(tt.stage)

			// THEN output should contain stage name
			assert.Contains(t, got, tt.wantText)
		})
	}
}

func TestSummarizeStagesEmpty(t *testing.T) {
	// GIVEN empty stages
	// WHEN summarizing
	got := SummarizeStages([]Stage{})

	// THEN should return completion message
	assert.Contains(t, got, "completed")
}

func TestSummarizeStagesMultiple(t *testing.T) {
	// GIVEN multiple stages
	stages := []Stage{StagePreparing, StageCompiling, StageLinking}

	// WHEN summarizing
	got := SummarizeStages(stages)

	// THEN should include all stages
	assert.Contains(t, got, "Preparing")
	assert.Contains(t, got, "Compiling")
	assert.Contains(t, got, "Linking")
}

func TestParserCaseInsensitive(t *testing.T) {
	// GIVEN a parser
	parser := NewParser()

	// WHEN parsing uppercase variants
	stage1 := parser.ParseLine("COMPILING CORE MODULES...")
	stage2 := parser.ParseLine("LINKING bin/godot.exe")
	stage3 := parser.ParseLine("scons: FINISHED building targets.")

	// THEN patterns should match regardless of case
	assert.Equal(t, StageCompiling, stage1)
	assert.Equal(t, StageLinking, stage2)
	assert.Equal(t, StageFinishing, stage3)
}

func TestParserEmptyOutput(t *testing.T) {
	// GIVEN empty output
	// WHEN parsing
	parser := NewParser()
	stages, err := parser.ParseOutput(strings.NewReader(""))

	// THEN no error should occur
	assert.Nil(t, err)

	// AND stages should be empty
	assert.Equal(t, 0, len(stages))
}
