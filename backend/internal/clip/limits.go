package clip

import (
	"time"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// Product limits and code-owned budgets of the clip context (ARCH-21). They live
// here rather than in platform/config because they are product rules, not
// deployment settings: platform parses env, and Environment below is the only
// part of a clip configuration a deployment may move.

const (
	TemplateNameChars = 40
	GuidanceChars     = 4000
	InformationFields = 10
	// CompositionStages is how many named composition stages one template may
	// carry (CLIP-141). A stage is a movement of the whole clip, so a body
	// listing more than this is scripting the footage rather than guiding the
	// flow.
	CompositionStages = 8
	// SequenceFrameCostMS is what one sequence-rendered caption frame costs to
	// draw (CDS-81). It is the number the approval surface multiplies the frames
	// by, so it is measured rather than assumed: 2026-09-17, the bundled resvg
	// over the real frames five sequence styles produce, 6 ms (word-pop) to
	// 37 ms (neon) per frame on a dev Mac, the spread coming from the painted
	// crop rather than the style. This is the upper-mid of that measured range,
	// an estimate rather than a wall-clock guarantee. CLIP-145 quotes the longest
	// permitted frame count with it and imposes no ceiling on sequence captions.
	SequenceFrameCostMS = 30
	LabelChars          = 40
	PromptChars         = 200
	TitleChars          = 100
	AnswerChars         = 500
	InstructionChars    = 1000
	MinDurationMS       = 15000
	MaxDurationMS       = 90000
	// MediaThreadMax bounds deployment tuning for both ffmpeg roles. Production
	// keeps the single encoder thread default for reproducibility; local dev may
	// spend more cores without turning a typo into an unbounded process.
	MediaThreadMax         = 8
	SourceCount            = 20
	SourceDurationMS       = 30 * 60 * 1000
	SourceFileBytes  int64 = 2 * 1024 * 1024 * 1024
	SourceBatchBytes int64 = 8 * 1024 * 1024 * 1024
)

// OriginalRetention is how long a confirmed original is kept. Confirmed
// originals follow product policy independently of incomplete uploads.
const OriginalRetention = 24 * time.Hour

const (
	MediaLeaseTTL         = time.Minute
	MediaHeartbeatHint    = 15 * time.Second
	MediaWaitTimeout      = 30 * time.Minute
	MediaStageTimeout     = 2 * time.Hour
	MediaStageTimeoutMax  = 6 * time.Hour
	MediaAttempts         = 3
	MediaAttemptsMax      = 5
	MediaPayloadMaxBytes  = 2 * 1024 * 1024
	MediaManifestMaxBytes = 16 * 1024
	MediaLabelMaxBytes    = 256
	MediaProgressMax      = 1000
)

func DefaultMediaStageLimits(env Environment) MediaStageLimits {
	l := MediaStageLimits{LeaseTTL: MediaLeaseTTL, WaitTimeout: MediaWaitTimeout, StageTimeout: MediaStageTimeout, MaxAttempts: MediaAttempts}
	if env.MediaLeaseTTL != 0 {
		l.LeaseTTL = env.MediaLeaseTTL
	}
	if env.MediaWaitTimeout != 0 {
		l.WaitTimeout = env.MediaWaitTimeout
	}
	if env.MediaStageTimeout != 0 {
		l.StageTimeout = env.MediaStageTimeout
	}
	if env.MediaMaxAttempts != 0 {
		l.MaxAttempts = env.MediaMaxAttempts
	}
	return l
}

// Environment is the deployment-owned half of a clip configuration: paths the
// image lays down, timeouts an operator may tune, and the presign windows the
// platform owns. cmd/api fills it from the parsed env and the Default*
// constructors below merge it into the product limits.
type Environment struct {
	MediaLeaseTTL, MediaWaitTimeout, MediaStageTimeout time.Duration
	MediaMaxAttempts                                   int
	WorkRoot                                           string
	FFmpegPath                                         string
	FFprobePath                                        string
	ResvgPath                                          string
	OverlayDir                                         string
	FontPaths                                          map[string]string

	WorkStaleAge  time.Duration
	MediaTimeout  time.Duration
	EncodeThreads int
	DecodeThreads int

	SourceBatchTTL time.Duration
	PutTTL         time.Duration
	GetTTL         time.Duration
	OrphanMinAge   time.Duration
	QuoteTTL       time.Duration
}

// DefaultLimits is the template-authoring surface CLIP validates against.
func DefaultLimits() Limits {
	return Limits{Composition: DefaultCompositionLimits(), NameChars: TemplateNameChars, GuidanceChars: GuidanceChars, FieldCount: InformationFields, LabelChars: LabelChars, PromptChars: PromptChars, TitleChars: TitleChars, AnswerChars: AnswerChars, InstructionChars: InstructionChars, MinDurationMS: MinDurationMS, MaxDurationMS: MaxDurationMS}
}

// DefaultCompositionLimits bounds one composition document.
func DefaultCompositionLimits() composition.Limits {
	return composition.Limits{
		SourceChars: 16000, Nodes: 200, Fields: InformationFields, Items: 20, Cuts: 100, Cues: 2400, Stages: CompositionStages,
		LabelChars: LabelChars, PromptChars: PromptChars, AnswerChars: AnswerChars,
		CopyChars: 500, GuideChars: GuidanceChars, MaxDurationMS: MaxDurationMS, AutoInsetMS: 120,
	}
}

// DefaultSourceLimits bounds what one upload batch may carry.
func DefaultSourceLimits(env Environment) SourceConfig {
	return SourceConfig{RetentionTTL: OriginalRetention, PlaybackTTL: 5 * time.Minute, MaxCount: SourceCount, MaxDurationMS: SourceDurationMS, MaxFilenameChars: 255, MaxFileBytes: SourceFileBytes, MaxBatchBytes: SourceBatchBytes, BatchTTL: env.SourceBatchTTL, PutTTL: env.PutTTL, Containers: map[string][]string{
		"mp4": {"video/mp4"}, "mov": {"video/quicktime"}, "m4v": {"video/x-m4v", "video/mp4"}, "webm": {"video/webm"},
	}}
}

// DefaultMediaConfig is what the media toolchain is allowed to spend.
func DefaultMediaConfig(env Environment) MediaConfig {
	return MediaConfig{
		WorkRoot: env.WorkRoot, FFmpegPath: env.FFmpegPath, FFprobePath: env.FFprobePath,
		StaleAge: env.WorkStaleAge, OperationTimeout: env.MediaTimeout, WaitDelay: 2 * time.Second,
		ChunkDurationMS: 60000, LongEdge: 720, FPS: 15, DecodeThreads: threads(env.DecodeThreads, 2), EncodeThreads: threads(env.EncodeThreads, 1), CRF: 28, AudioRate: 48000, AudioBitrate: 64000,
		StdoutLimit: 64 * 1024, StderrLimit: 16 * 1024, MaxStreams: 64, MaxDimension: 16384, DurationToleranceMS: 1000,
		AnalysisMaxBytes: 8 << 20, PreparedMaxBytes: 512 << 20, WorkspaceMaxBytes: 8 << 30,
		VideoMaxRate: 900000, VideoBufferSize: 1800000, RetryMaxRate: 650000, RetryBufferSize: 1300000,
		DiskCheckInterval: 100 * time.Millisecond,
		Sources:           DefaultSourceLimits(env),
	}
}

// threads keeps an Environment built by hand — tests and tools that never go
// through config.Load — on the production pinning. Load refuses a non-positive
// value, so a zero here can only mean nobody set one.
func threads(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// DefaultRenderConfig is the render stage's shape budget.
//
// FadeMS is the design system's transition constant (CDS-36), not a setting: the
// renderer's constructor refuses a configuration that disagrees with it.
func DefaultRenderConfig(env Environment) RenderConfig {
	return RenderConfig{Composition: DefaultCompositionLimits(), OverlayBatchSize: 8, ResvgPath: env.ResvgPath, FontPaths: env.FontPaths, OverlayDir: env.OverlayDir, MaxCuts: 100, MaxCopyRunes: 500, MergeInputs: 6, SequenceFrameCostMS: SequenceFrameCostMS, SampleBatch: 24, FadeMS: design.Transition.FadeMS, FPS: 30, CRF: 20, AudioRate: 48000, AudioBitrate: 192000, MinDurationMS: MinDurationMS, MaxDurationMS: MaxDurationMS}
}

// DefaultAnalysisLimits bounds the observation pass. At most 60 segments per
// 60-second chunk keeps one chunk's output shape bounded.
func DefaultAnalysisLimits() AnalysisLimits {
	return AnalysisLimits{ChunkMS: 60000, MaxSources: SourceCount, MaxSourceDurationMS: SourceDurationMS, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}
}

// DefaultGenerationConfig is what one generation run is allowed to spend.
func DefaultGenerationConfig(env Environment) GenerationConfig {
	return GenerationConfig{Preview: PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 * 1024, MaxResponseBytes: 4 * 1024 * 1024, MaxFrameCells: 30, MaxSheetPixels: 4096, Timeout: 5 * time.Second}, Render: DefaultRenderConfig(env), Media: DefaultMediaConfig(env), Analysis: DefaultAnalysisLimits(), ReadTTL: env.GetTTL, CleanupTimeout: 30 * time.Second, OrphanMinAge: env.OrphanMinAge, QuoteTTL: env.QuoteTTL}
}
