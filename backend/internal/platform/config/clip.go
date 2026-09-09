package config

import (
	"github.com/postpilot/backend/internal/clip"
	"time"
)

const (
	ClipTemplateNameChars       = 40
	ClipGuidanceChars           = 4000
	ClipInformationFields       = 10
	ClipLabelChars              = 40
	ClipPromptChars             = 200
	ClipTitleChars              = 100
	ClipAnswerChars             = 500
	ClipMinDurationMS           = 15000
	ClipMaxDurationMS           = 90000
	ClipSourceCount             = 20
	ClipSourceDurationMS        = 30 * 60 * 1000
	ClipSourceFileBytes   int64 = 2 * 1024 * 1024 * 1024
	ClipSourceBatchBytes  int64 = 8 * 1024 * 1024 * 1024
)

func ClipSourceLimits(batchTTL, putTTL time.Duration) clip.SourceConfig {
	return clip.SourceConfig{MaxCount: ClipSourceCount, MaxDurationMS: ClipSourceDurationMS, MaxFilenameChars: 255, MaxFileBytes: ClipSourceFileBytes, MaxBatchBytes: ClipSourceBatchBytes, BatchTTL: batchTTL, PutTTL: putTTL, Containers: map[string][]string{
		"mp4": {"video/mp4"}, "mov": {"video/quicktime"}, "m4v": {"video/x-m4v", "video/mp4"}, "webm": {"video/webm"},
	}}
}

func ClipLimits() clip.Limits {
	return clip.Limits{NameChars: ClipTemplateNameChars, GuidanceChars: ClipGuidanceChars, FieldCount: ClipInformationFields, LabelChars: ClipLabelChars, PromptChars: ClipPromptChars, TitleChars: ClipTitleChars, AnswerChars: ClipAnswerChars, MinDurationMS: ClipMinDurationMS, MaxDurationMS: ClipMaxDurationMS}
}
