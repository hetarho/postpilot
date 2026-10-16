package ai_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func TestLinkedPostCountsTowardAllowance(t *testing.T) {
	base := flowInput()
	base.Analyses = nil
	cfg := ai.DefaultConfig(clip.Environment{})
	system, user := ai.BuildNarrationPrompt(clip.NarrationInput{PlanningInput: base, Flow: ai.WidestFlow(cfg, base)}, clip.DefaultCompositionLimits())
	base.Policy.InputTokens = ai.PromptBytes(system, user, ai.NarrationSchema()) + 4096
	s, models := newService(t, defaultFlow(), true)
	if err := s.ValidatePreparation(testRef(), base, []clip.AnalysisSource{flowInput().Analyses[0].Source}); err != nil {
		t.Fatal("baseline refused", err)
	}
	material := strings.Repeat("가", base.Policy.InputTokens+10000)
	base.LinkedPost = &clip.LinkedPostText{Slug: "post", Title: "owner", Sections: []clip.LinkedPostSection{{Text: material}}}
	if err := s.ValidatePreparation(testRef(), base, []clip.AnalysisSource{flowInput().Analyses[0].Source}); !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatal("linked material escaped preparation bound", err)
	}
	base.Analyses = flowInput().Analyses
	if err := s.ValidateStorylineRequest(testRef(), clip.StorylineInput{PlanningInput: base}); !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatal("storyline request escaped bound", err)
	}
	revision := clip.RevisionInput{PlanningInput: base, Current: ai.WidestFlow(cfg, base), Request: "短く", Target: clip.RevisionBoth}
	if err := s.ValidateRevision(testRef(), revision); !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatal("revision escaped bound", err)
	}
	upload := clip.UploadTextInput{LinkedPost: base.LinkedPost, Language: "ko", Channel: clip.ChannelNaver, Policy: base.Policy}
	upload.Policy = testPolicy("write")
	upload.Policy.InputTokens = llm.ClipInputUnits
	upload.Policy.ResponseRetries = 0
	upload.LinkedPost = &clip.LinkedPostText{Slug: "post", Sections: []clip.LinkedPostSection{{Text: strings.Repeat("가", llm.ClipPlanInputUnits)}}}
	if err := s.ValidateUploadText(t.Context(), testRef(), upload); !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatal("upload text escaped bound", err)
	}
	if len(models.calls) != 0 || base.LinkedPost.Sections[0].Text != material {
		t.Fatal("refused input was paid for or shortened")
	}
}
