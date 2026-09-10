package config

import (
	"github.com/postpilot/backend/internal/clip"
	"time"
)

func ClipMedia(cfg *Config) clip.MediaConfig {
	return clip.MediaConfig{
		WorkRoot: cfg.ClipWorkRoot, FFmpegPath: cfg.ClipFFmpegPath, FFprobePath: cfg.ClipFFprobePath,
		StaleAge: cfg.ClipWorkStaleAge, OperationTimeout: cfg.ClipMediaTimeout, WaitDelay: 2 * time.Second,
		ChunkDurationMS: 60000, LongEdge: 720, FPS: 15, Threads: 1, CRF: 28, AudioRate: 48000, AudioBitrate: 64000,
		StdoutLimit: 64 * 1024, StderrLimit: 8 * 1024, MaxStreams: 64, MaxDimension: 16384, DurationToleranceMS: 1000,
		AnalysisMaxBytes: 8 << 20, PreparedMaxBytes: 512 << 20, WorkspaceMaxBytes: 8 << 30,
		VideoMaxRate: 900000, VideoBufferSize: 1800000, RetryMaxRate: 650000, RetryBufferSize: 1300000,
		DiskCheckInterval: 100 * time.Millisecond,
		Sources:           ClipSourceLimits(cfg.ClipSourceBatchTTL, cfg.PresignPutTTL),
	}
}

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
