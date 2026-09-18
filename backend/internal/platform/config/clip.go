package config

import (
	"github.com/postpilot/backend/internal/clip"
	"time"
)

func ClipMedia(cfg *Config) clip.MediaConfig {
	return clip.MediaConfig{
		WorkRoot: cfg.ClipWorkRoot, FFmpegPath: cfg.ClipFFmpegPath, FFprobePath: cfg.ClipFFprobePath,
		StaleAge: cfg.ClipWorkStaleAge, OperationTimeout: cfg.ClipMediaTimeout, WaitDelay: 2 * time.Second,
		ChunkDurationMS: 60000, LongEdge: 720, FPS: 15, DecodeThreads: 2, EncodeThreads: 1, CRF: 28, AudioRate: 48000, AudioBitrate: 64000,
		StdoutLimit: 64 * 1024, StderrLimit: 16 * 1024, MaxStreams: 64, MaxDimension: 16384, DurationToleranceMS: 1000,
		AnalysisMaxBytes: 8 << 20, PreparedMaxBytes: 512 << 20, WorkspaceMaxBytes: 8 << 30,
		VideoMaxRate: 900000, VideoBufferSize: 1800000, RetryMaxRate: 650000, RetryBufferSize: 1300000,
		DiskCheckInterval: 100 * time.Millisecond,
		Sources:           ClipSourceLimits(cfg.ClipSourceBatchTTL, cfg.PresignPutTTL),
	}
}

const (
	ClipTemplateNameChars = 40
	ClipGuidanceChars     = 4000
	ClipInformationFields = 10
	// How many named composition stages one template may carry (CLIP-141). A
	// stage is a movement of the whole clip, so a body listing more than this is
	// scripting the footage rather than guiding the flow.
	ClipCompositionStages = 8
	// What one sequence-rendered caption frame costs to draw (CDS-81). It is the
	// number the approval surface multiplies the frames by, so it is measured
	// rather than assumed: 2026-09-17, the bundled resvg over the real frames
	// five sequence styles produce, 6 ms (word-pop) to 37 ms (neon) per frame on
	// a dev Mac, the spread coming from the painted crop rather than the style.
	// This is the upper-mid of that measured range, an estimate rather than a
	// wall-clock guarantee. CLIP-145 quotes the longest permitted frame count
	// with it and imposes no ceiling on sequence captions.
	ClipSequenceFrameCostMS       = 30
	ClipLabelChars                = 40
	ClipPromptChars               = 200
	ClipTitleChars                = 100
	ClipAnswerChars               = 500
	ClipInstructionChars          = 1000
	ClipMinDurationMS             = 15000
	ClipMaxDurationMS             = 90000
	ClipSourceCount               = 20
	ClipSourceDurationMS          = 30 * 60 * 1000
	ClipSourceFileBytes     int64 = 2 * 1024 * 1024 * 1024
	ClipSourceBatchBytes    int64 = 8 * 1024 * 1024 * 1024
)

func ClipSourceLimits(batchTTL, putTTL time.Duration) clip.SourceConfig {
	return clip.SourceConfig{RetentionTTL: ClipOriginalRetention, PlaybackTTL: 5 * time.Minute, MaxCount: ClipSourceCount, MaxDurationMS: ClipSourceDurationMS, MaxFilenameChars: 255, MaxFileBytes: ClipSourceFileBytes, MaxBatchBytes: ClipSourceBatchBytes, BatchTTL: batchTTL, PutTTL: putTTL, Containers: map[string][]string{
		"mp4": {"video/mp4"}, "mov": {"video/quicktime"}, "m4v": {"video/x-m4v", "video/mp4"}, "webm": {"video/webm"},
	}}
}

// Confirmed originals follow product policy independently of incomplete uploads.
const ClipOriginalRetention = 24 * time.Hour

func ClipLimits() clip.Limits {
	return clip.Limits{Composition: ClipCompositionLimits(), NameChars: ClipTemplateNameChars, GuidanceChars: ClipGuidanceChars, FieldCount: ClipInformationFields, LabelChars: ClipLabelChars, PromptChars: ClipPromptChars, TitleChars: ClipTitleChars, AnswerChars: ClipAnswerChars, InstructionChars: ClipInstructionChars, MinDurationMS: ClipMinDurationMS, MaxDurationMS: ClipMaxDurationMS}
}
