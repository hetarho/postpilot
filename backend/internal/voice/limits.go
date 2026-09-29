package voice

// The projection's excerpt budget (VOICE-46): up to FewShotMax excerpts from the 학습 글, each cut
// around the target length and never past the maximum. They are code-owned: changing one changes
// product semantics.
const (
	FewShotMax                = 3
	FewShotExcerptTargetChars = 500
	FewShotExcerptMaxChars    = 800
)
