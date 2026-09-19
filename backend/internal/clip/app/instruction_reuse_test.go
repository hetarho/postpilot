package app

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// The owner's instruction is writer input a generation freezes (CLIP-69), so a
// changed one leaves no candidate plan to reuse and no quote to carry over
// (CLIP-93). Observations are untouched by it and stay reusable.
func TestInstructionChangeInvalidatesPlanAndQuote(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"added", "", "강조: 보습력"},
		{"edited", "강조: 보습력", "강조: 발림성"},
		{"cleared", "강조: 보습력", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, after := clip.GenerationPayload{Language: "ko", Instruction: tc.before}, clip.GenerationPayload{Language: "ko", Instruction: tc.after}
			if planRecoveryDigest(before) == planRecoveryDigest(after) {
				t.Error("an instruction change reused the plan")
			}
			p, q := clip.Project{Language: "ko", Instruction: tc.before}, clip.Project{Language: "ko", Instruction: tc.after}
			if clip.QuoteInputDigest(p, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}) == clip.QuoteInputDigest(q, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}) {
				t.Error("an instruction change reused the quote")
			}
		})
	}
	if planRecoveryDigest(clip.GenerationPayload{Language: "ko"}) != planRecoveryDigest(clip.GenerationPayload{Language: "ko"}) {
		t.Fatal("an absent instruction moved the digest")
	}
	if clip.QuoteInputDigest(clip.Project{Language: "ko"}, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}) != clip.QuoteInputDigest(clip.Project{Language: "ko"}, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}) {
		t.Fatal("an absent instruction moved the quote digest")
	}
}
