package voice

// PersonalizationThresholds are the product thresholds for a voice's projection (ARCH-21).
// They are deliberately code-owned: changing one changes product semantics. No interval
// exists because personalization never runs on a clock — all evaluation is request-time
// and user-initiated.
func PersonalizationThresholds() PersonalizationConfig {
	return PersonalizationConfig{
		FewShotMax:                3,
		FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800,
		EndingMaxConsecutive: 2,
	}
}
