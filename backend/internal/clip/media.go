package clip

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var ErrInvalidMedia = errors.New("clip source media is invalid or unsupported")

// Media owns local processing only; it never sees a signed URL or an object store.
// Workspaces and chunk paths are callback-scoped and must not be persisted.
type Media interface {
	WithWorkspace(context.Context, string, func(MediaWorkspace) error) error
	Probe(context.Context, MediaWorkspace, string) (MediaInfo, error)
	PrepareAnalysisChunks(context.Context, MediaWorkspace, MediaSource, func(AnalysisChunk) error) error
	CleanupStale(context.Context, time.Time) error
}

type MediaWorkspace struct{ Path string }
type MediaStream struct {
	Index       int
	Kind, Codec string
}
type MediaInfo struct {
	PixelFormat, SampleAspectRatio           string
	DurationMS, Width, Height, Rotation      int
	FrameRateNumerator, FrameRateDenominator int
	HasAudio                                 bool
	Streams                                  []MediaStream
}
type ProbedSource struct {
	Metadata SourceMetadata
	Info     MediaInfo
}
type MediaSource struct {
	Path, SourceID, Fingerprint string
	Info                        MediaInfo
}
type AnalysisChunk struct {
	Path, SourceID, Fingerprint string
	Index, OffsetMS, DurationMS int
}
type MediaConfig struct {
	WorkRoot, FFmpegPath, FFprobePath                                     string
	StaleAge, OperationTimeout, WaitDelay                                 time.Duration
	ChunkDurationMS, LongEdge, FPS, Threads, CRF, AudioRate, AudioBitrate int
	StdoutLimit, StderrLimit, MaxStreams, MaxDimension                    int
	DurationToleranceMS                                                   int
	Sources                                                               SourceConfig
}

// ValidateProbedSources is called after every source has been independently probed
// and before the first model call. Callers may probe/download one source at a time.
func ValidateProbedSources(cfg MediaConfig, sources []ProbedSource) (int, error) {
	if len(sources) == 0 || len(sources) > cfg.Sources.MaxCount || cfg.ChunkDurationMS <= 0 {
		return 0, ErrInvalidMedia
	}
	total, chunks := 0, 0
	var bytes int64
	seen := map[string]bool{}
	for _, source := range sources {
		m, p := source.Metadata, source.Info
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(m.Filename), "."))
		if !slices.Contains(cfg.Sources.Containers[ext], m.ContentType) || p.DurationMS <= 0 || p.DurationMS > cfg.Sources.MaxDurationMS-total || p.Width <= 0 || p.Height <= 0 || p.Width != m.Width || p.Height != m.Height || math.Abs(float64(p.DurationMS)-float64(m.DurationMS)) > float64(cfg.DurationToleranceMS) || m.Bytes <= 0 || m.Bytes > cfg.Sources.MaxFileBytes || m.Bytes > cfg.Sources.MaxBatchBytes-bytes || m.Fingerprint == "" || seen[m.Fingerprint] {
			return 0, ErrInvalidMedia
		}
		total += p.DurationMS
		bytes += m.Bytes
		seen[m.Fingerprint] = true
		chunks += (p.DurationMS + cfg.ChunkDurationMS - 1) / cfg.ChunkDurationMS
	}
	return chunks, nil
}
