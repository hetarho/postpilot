package ai_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/platform/config"
)

// A project with no instruction produces exactly the request it produced before
// the instruction existed: the original authority line, and no key for a value
// it does not have.
func TestNoInstructionLeavesTheRequestByteIdentical(t *testing.T) {
	l, fade := config.ClipCompositionLimits(), 200
	base := nativeInput()
	systemBefore, userBefore := ai.BuildPlanPrompt(base, fade, l)

	written := base
	written.Instruction = "고기 굽는 소리를 살려 주세요."
	systemWith, userWith := ai.BuildPlanPrompt(written, fade, l)
	if systemWith == systemBefore || userWith == userBefore {
		t.Fatal("an instruction changed nothing in the request")
	}

	// Clearing it returns the original bytes, so the instruction path leaves no
	// residue on a project that carries none.
	cleared := written
	cleared.Instruction = ""
	systemAfter, userAfter := ai.BuildPlanPrompt(cleared, fade, l)
	if systemAfter != systemBefore || userAfter != userBefore {
		t.Fatal("a cleared instruction did not restore the original request")
	}
	if !strings.Contains(systemBefore, "Frozen XML is the content authority") || strings.Contains(systemBefore, "project_instruction") {
		t.Fatal("the uninstructed contract changed")
	}
	if strings.Contains(userBefore, "project_instruction") {
		t.Fatal("the uninstructed payload carries an instruction key")
	}
}

// The instruction counts toward the frozen input allowance, so a project whose
// instruction overflows it is refused before any paid work, with its measured
// size (CLIP-90).
func TestOverlongInstructionIsRefusedBeforePaidWork(t *testing.T) {
	l, fade := config.ClipCompositionLimits(), 200
	base := nativeInput()
	base.Analyses = nil
	withInstruction := base
	withInstruction.Instruction = strings.Repeat("가", config.ClipInstructionChars)
	// An allowance that exactly admits the request without the instruction: the
	// instruction is then the only thing that can overflow it.
	system, user := ai.BuildPlanPrompt(base, fade, l)
	allowance := ai.PromptBytes(system, user, ai.CompositionPlanSchema()) + 2048

	s, models, _ := newService(t, raw(nativePlan()), true)
	sources := []clip.AnalysisSource{nativeInput().Analyses[0].Source}
	base.Policy.InputTokens, withInstruction.Policy.InputTokens = allowance, allowance
	if err := s.ValidatePreparation(testRef(), base, sources); err != nil {
		t.Fatalf("the same request without an instruction was refused: %v", err)
	}
	err := s.ValidatePreparation(testRef(), withInstruction, sources)
	if !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatalf("an overflowing instruction was not refused: %v", err)
	}
	if len(models.calls) != 0 {
		t.Fatal("a refused input still paid for a call")
	}
	d, ok := clip.DiagnosticFromError(err)
	if !ok || d.Check != "input_prompt_limit" || d.Values["input_bytes"] <= d.Values["input_limit_bytes"] {
		t.Fatalf("the refusal lost its measurements: %+v", d)
	}
}
