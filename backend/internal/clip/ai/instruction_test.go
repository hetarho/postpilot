package ai_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
)

// The instruction is the ONE thing it changes in a writing request, and a
// project that carries none leaves no residue of one.
func TestAnInstructionChangesOnlyItsOwnValue(t *testing.T) {
	l, fade := clip.DefaultCompositionLimits(), 200
	base := flowInput()
	systemBefore, userBefore := ai.BuildFlowPrompt(base, fade, l)

	written := base
	written.Instruction = "고기 굽는 소리를 살려 주세요."
	systemWith, userWith := ai.BuildFlowPrompt(written, fade, l)
	if systemWith != systemBefore || userWith == userBefore {
		t.Fatal("the instruction moved the contract instead of its own payload value")
	}

	cleared := written
	cleared.Instruction = ""
	systemAfter, userAfter := ai.BuildFlowPrompt(cleared, fade, l)
	if systemAfter != systemBefore || userAfter != userBefore {
		t.Fatal("a cleared instruction did not restore the original request")
	}
	// The contract states the instruction's authority whether or not this
	// project wrote one, and the payload always carries the key.
	if !strings.Contains(systemBefore, "project_instruction is the CONTENT authority") || !strings.Contains(userBefore, `"project_instruction":""`) {
		t.Fatal("the uninstructed request lost the instruction's place")
	}
}

// The instruction counts toward the frozen input allowance, so a project whose
// instruction overflows it is refused before any paid work, with its measured
// size (CLIP-90).
func TestOverlongInstructionIsRefusedBeforePaidWork(t *testing.T) {
	l := clip.DefaultCompositionLimits()
	base := flowInput()
	base.Analyses = nil
	withInstruction := base
	withInstruction.Instruction = strings.Repeat("가", clip.InstructionChars)
	// An allowance that exactly admits the LARGER of the two writing requests
	// without the instruction: the instruction is then the only thing that can
	// overflow it (CLIP-90).
	cfg := ai.DefaultConfig(clip.Environment{})
	system, user := ai.BuildNarrationPrompt(clip.NarrationInput{PlanningInput: base, Flow: ai.WidestFlow(cfg, base)}, l)
	allowance := ai.PromptBytes(system, user, ai.NarrationSchema()) + 2048

	s, models, _ := newService(t, defaultFlow(), true)
	sources := []clip.AnalysisSource{flowInput().Analyses[0].Source}
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
