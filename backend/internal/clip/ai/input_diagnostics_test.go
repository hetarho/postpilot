package ai_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func TestObservedPromptOverflowKeepsPrivateBoundedMeasurements(t *testing.T) {
	s, models, _ := newService(t, "", true)
	in := nativeInput()
	in.Policy.InputTokens = llm.ClipPlanInputUnits
	// Valid individual fields/observations whose complete aggregate exceeds 64k.
	a := in.Analyses[0]
	a.Segments[0].Event = strings.Repeat("private-canary ", 100)
	a.Segments[1].Event = strings.Repeat("private-canary ", 100)
	in.Analyses = nil
	for i := range 20 {
		next := a
		next.Source.ID = fmt.Sprintf("source-%d", i)
		next.Source.Fingerprint = fmt.Sprintf("fingerprint-%d", i)
		in.Analyses = append(in.Analyses, next)
	}
	_, usage, err := s.Flow(t.Context(), testRef(), in)
	d, ok := clip.DiagnosticFromError(err)
	if !errors.Is(err, clip.ErrInputTooLarge) || !ok || d.Check != "input_prompt_limit" || d.Phase != "input" || d.Values["input_bytes"] <= d.Values["input_limit_bytes"] || d.Values["input_limit_bytes"] != 61952 || len(models.calls) != 0 || usage != (llm.Usage{}) {
		t.Fatalf("oversized aggregate not refused before dispatch: %v %+v", err, d)
	}
	if len(d.Values) != 5 || strings.Contains(err.Error(), "canary") || clip.SafeAttemptCheck(d.Check) != d.Check || clip.SafeAttemptPhase(d.Phase) != d.Phase {
		t.Fatal("unsafe diagnostic")
	}
}
