package config

import "github.com/postpilot/backend/internal/clip"

const (
	ClipTemplateNameChars = 40
	ClipGuidanceChars     = 4000
	ClipInformationFields = 10
	ClipLabelChars        = 40
	ClipPromptChars       = 200
	ClipTitleChars        = 100
	ClipAnswerChars       = 500
	ClipMinDurationMS     = 15000
	ClipMaxDurationMS     = 90000
)

func ClipLimits() clip.Limits {
	return clip.Limits{NameChars: ClipTemplateNameChars, GuidanceChars: ClipGuidanceChars, FieldCount: ClipInformationFields, LabelChars: ClipLabelChars, PromptChars: ClipPromptChars, TitleChars: ClipTitleChars, AnswerChars: ClipAnswerChars, MinDurationMS: ClipMinDurationMS, MaxDurationMS: ClipMaxDurationMS}
}
