package voice

import "time"

// PersonalizationThresholds are the product thresholds for progressive learning
// (ARCH-21). They are deliberately code-owned: changing one changes product
// semantics. No interval exists because personalization never runs on a clock —
// all evaluation is request-time and user-initiated.
func PersonalizationThresholds() PersonalizationConfig {
	return PersonalizationConfig{
		FewShotTargetCount: 2, FewShotMax: 3,
		FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800,
		EmbeddingSwitchPosts: 50, DiffMaxRules: 3, DiffMinPatternEdits: 2,
		RuleActivationEvidence: 3, RuleRetireAfter: 180 * 24 * time.Hour,
		ValidationPostCount: 3, EndingMaxConsecutive: 2,
	}
}
