package clip

import "testing"

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
			before, after := generationPayload{Language: "ko", Instruction: tc.before}, generationPayload{Language: "ko", Instruction: tc.after}
			if planRecoveryDigest(before) == planRecoveryDigest(after) {
				t.Error("an instruction change reused the plan")
			}
			p, q := Project{Language: "ko", Instruction: tc.before}, Project{Language: "ko", Instruction: tc.after}
			if QuoteInputDigest(p, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) == QuoteInputDigest(q, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) {
				t.Error("an instruction change reused the quote")
			}
		})
	}
	if planRecoveryDigest(generationPayload{Language: "ko"}) != planRecoveryDigest(generationPayload{Language: "ko"}) {
		t.Fatal("an absent instruction moved the digest")
	}
	if QuoteInputDigest(Project{Language: "ko"}, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) != QuoteInputDigest(Project{Language: "ko"}, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) {
		t.Fatal("an absent instruction moved the quote digest")
	}
}
