package config

import "github.com/postpilot/backend/internal/clip/composition"

func ClipCompositionLimits() composition.Limits {
	return composition.Limits{
		SourceChars: 16000, Nodes: 200, Fields: ClipInformationFields, Items: 20, Cuts: 100, Cues: 2400,
		LabelChars: ClipLabelChars, PromptChars: ClipPromptChars, AnswerChars: ClipAnswerChars,
		CopyChars: 500, GuideChars: ClipGuidanceChars, MaxDurationMS: ClipMaxDurationMS, AutoInsetMS: 120,
	}
}
