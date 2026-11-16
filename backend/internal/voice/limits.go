package voice

// The projection's excerpt budget (VOICE-46): up to FewShotMax excerpts from the 학습 글, each cut
// around the target length and never past the maximum. They are code-owned: changing one changes
// product semantics.
const (
	FewShotMax                = 3
	FewShotExcerptTargetChars = 500
	FewShotExcerptMaxChars    = 800
)

// CompareMinExpected is VOICE_COMPARE_MIN_EXPECTED (VOICE-62): a habit the voice uses so rarely
// that a text of this length would carry fewer occurrences than this is shown without a verdict.
const CompareMinExpected = 2
