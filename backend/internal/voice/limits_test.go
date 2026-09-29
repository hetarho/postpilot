package voice_test

import (
	"testing"

	"github.com/postpilot/backend/internal/voice"
)

// VOICE-46: VOICE_FEW_SHOT_MAX 3, excerpts cut around 500 and never past 800 characters.
func TestTheExcerptBudgetIsTheSpecifiedOne(t *testing.T) {
	if voice.FewShotMax != 3 || voice.FewShotExcerptTargetChars != 500 || voice.FewShotExcerptMaxChars != 800 {
		t.Fatalf("excerpt budget = %d / %d / %d", voice.FewShotMax, voice.FewShotExcerptTargetChars, voice.FewShotExcerptMaxChars)
	}
}
