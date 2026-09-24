package clip

import (
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"strings"
)

func MediaAnalysisSlot(source string, index int) string {
	return fmt.Sprintf("analysis/%s/%d", source, index)
}

func ValidateMediaTask(op MediaOperation, task MediaTask, cfg MediaConfig) error {
	if task.Version != MediaContractVersion {
		return ErrMediaIncompatible
	}
	if len(task.Sources) == 0 || len(task.Sources) > cfg.Sources.MaxCount {
		return ErrInvalid
	}
	seen := map[string]bool{}
	var totalBytes int64
	var duration int
	for _, s := range task.Sources {
		if !ValidMediaLabel(s.ID) || seen[s.ID] || !ValidMediaLabel(s.Fingerprint) || s.Bytes <= 0 || s.Bytes > cfg.Sources.MaxFileBytes || s.Bytes > cfg.Sources.MaxBatchBytes-totalBytes || s.DurationMS <= 0 || s.DurationMS > cfg.Sources.MaxDurationMS-duration || s.Width <= 0 || s.Height <= 0 || s.Width > cfg.MaxDimension || s.Height > cfg.MaxDimension {
			return ErrInvalid
		}
		seen[s.ID] = true
		totalBytes += s.Bytes
		duration += s.DurationMS
		reuseDuration := s.DurationMS
		if len(s.ReusedChunks) > 0 {
			if _, err := ValidateProbedSources(cfg, []ProbedSource{{Metadata: s.SourceMetadata, Info: s.Info}}); err != nil {
				return err
			}
			reuseDuration = s.Info.DurationMS
		}
		previous := -1
		for _, index := range s.ReusedChunks {
			if index <= previous || index > (reuseDuration-1)/cfg.ChunkDurationMS {
				return ErrInvalid
			}
			previous = index
		}
	}
	switch op {
	case MediaPrepare:
		if task.Plan != "" {
			return ErrInvalid
		}
	case MediaRender:
		plan, err := DecodeEditPlan(task.Plan)
		if err != nil {
			return err
		}
		plan = task.Render.Apply(plan)
		plan.HideDisclosure = task.HideDisclosure
		sources := make([]RenderSource, len(task.Sources))
		for i, s := range task.Sources {
			sources[i] = RenderSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info}
		}
		if err := ValidateEditPlan(DefaultRenderConfig(Environment{}), plan, sources); err != nil {
			return err
		}
	default:
		return ErrInvalid
	}
	return nil
}

func ValidateMediaOutput(op MediaOperation, task MediaTask, out MediaOutput, cfg MediaConfig) error {
	digest, err := hex.DecodeString(out.Digest)
	if err != nil || len(digest) != 32 || out.Digest != strings.ToLower(out.Digest) || out.ContentType != "video/mp4" || out.Bytes <= 0 || out.Info.Width <= 0 || out.Info.Height <= 0 || out.Info.DurationMS <= 0 || len(out.Info.Streams) > cfg.MaxStreams {
		return ErrInvalid
	}
	if out.Info.Width > cfg.MaxDimension || out.Info.Height > cfg.MaxDimension || out.Info.FrameRateNumerator < 0 || out.Info.FrameRateNumerator > 1000000 || out.Info.FrameRateDenominator < 0 || out.Info.FrameRateDenominator > 1000000 {
		return ErrInvalid
	}
	if len(out.Info.PixelFormat) > 64 || len(out.Info.SampleAspectRatio) > 64 {
		return ErrInvalid
	}
	for _, stream := range out.Info.Streams {
		if len(stream.Kind) > 64 || len(stream.Codec) > 64 || len(stream.Profile) > 64 {
			return ErrInvalid
		}
	}
	if op == MediaRender {
		if out.Slot != "result" || out.SourceID != "" || out.Index != 0 || out.OffsetMS != 0 || out.Bytes > cfg.Sources.MaxFileBytes {
			return ErrInvalid
		}
		plan, err := DecodeEditPlan(task.Plan)
		if err != nil {
			return err
		}
		canvas, err := ClipCanvas(plan.Ratio)
		if err != nil {
			return err
		}
		render := DefaultRenderConfig(Environment{})
		if out.Info.Width != canvas.Width || out.Info.Height != canvas.Height || out.Info.FrameRateDenominator <= 0 || out.Info.FrameRateNumerator != render.FPS*out.Info.FrameRateDenominator || out.DurationMS != out.Info.DurationMS || math.Abs(float64(out.DurationMS-plan.DurationMS)) > float64(cfg.DurationToleranceMS) {
			return ErrInvalidMedia
		}
		if out.Info.Rotation != 0 || out.Info.PixelFormat != "yuv420p" || out.Info.SampleAspectRatio != "1:1" || out.Info.DecodedFrames <= 0 || out.Info.DecodedFrames > 1000000 || math.Abs(float64(out.Info.DecodedFrames)*1000/float64(render.FPS)-float64(plan.DurationMS)) > 1000/float64(render.FPS) {
			return ErrInvalidMedia
		}
		audio := false
		for _, cut := range plan.Cuts {
			for _, s := range task.Sources {
				if s.ID == cut.SourceID {
					audio = audio || plan.RetainsOriginalAudio(cut) && s.Info.HasAudio
				}
			}
		}
		if audio != out.Info.HasAudio || audio && out.Info.AudioRate != render.AudioRate {
			return ErrInvalidMedia
		}
		videoCount, audioCount := 0, 0
		for _, s := range out.Info.Streams {
			switch s.Kind {
			case "video":
				videoCount++
				if s.Codec != "h264" || s.Profile != "High" {
					return ErrInvalidMedia
				}
			case "audio":
				audioCount++
				if s.Codec != "aac" {
					return ErrInvalidMedia
				}
			default:
				return ErrInvalidMedia
			}
		}
		if videoCount != 1 || audio && audioCount != 1 || !audio && audioCount != 0 {
			return ErrInvalidMedia
		}
		return nil
	}
	if op != MediaPrepare || out.Bytes > cfg.AnalysisMaxBytes || max(out.Info.Width, out.Info.Height) > cfg.LongEdge || out.Index < 0 || out.DurationMS <= 0 || out.DurationMS > cfg.ChunkDurationMS {
		return ErrInvalid
	}
	for _, s := range task.Sources {
		if s.ID != out.SourceID {
			continue
		}
		if slices.Contains(s.ReusedChunks, out.Index) || out.Index > (s.DurationMS+cfg.DurationToleranceMS-1)/cfg.ChunkDurationMS || out.OffsetMS != out.Index*cfg.ChunkDurationMS || out.Slot != MediaAnalysisSlot(s.ID, out.Index) || math.Abs(float64(out.Info.DurationMS-out.DurationMS)) > float64(cfg.DurationToleranceMS) {
			return ErrInvalid
		}
		return nil
	}
	return ErrInvalid
}

// ValidateMediaResult checks ordering and complete coverage before a receipt can
// release the parent to its first paid call. Reservations alone prove no coverage.
func ValidateMediaResult(op MediaOperation, task MediaTask, result MediaResult, cfg MediaConfig) error {
	if result.Version != MediaContractVersion || len(result.Sources) != len(task.Sources) {
		return ErrInvalid
	}
	verified := make([]ProbedSource, len(task.Sources))
	for i, s := range task.Sources {
		v := result.Sources[i]
		if v.ID != s.ID || v.Fingerprint != s.Fingerprint {
			return ErrInvalid
		}
		verified[i] = ProbedSource{Metadata: s.SourceMetadata, Info: v.Info}
	}
	if _, err := ValidateProbedSources(cfg, verified); err != nil {
		return err
	}
	if op == MediaRender {
		if len(result.Outputs) != 1 {
			return ErrInvalid
		}
		return ValidateMediaOutput(op, task, result.Outputs[0], cfg)
	}
	if result.Plan != "" {
		return ErrInvalid
	}
	var bytes int64
	cursor := 0
	for i, s := range task.Sources {
		duration := result.Sources[i].Info.DurationMS
		for offset, index := 0, 0; offset < duration; offset, index = offset+cfg.ChunkDurationMS, index+1 {
			if slices.Contains(s.ReusedChunks, index) {
				continue
			}
			if cursor >= len(result.Outputs) {
				return ErrInvalid
			}
			out := result.Outputs[cursor]
			if out.SourceID != s.ID || out.Index != index || out.OffsetMS != offset || out.DurationMS != min(cfg.ChunkDurationMS, duration-offset) {
				return ErrInvalid
			}
			if err := ValidateMediaOutput(op, task, out, cfg); err != nil {
				return err
			}
			if out.Bytes > cfg.PreparedMaxBytes-bytes {
				return ErrAnalysisTooLarge
			}
			bytes += out.Bytes
			cursor++
		}
		for _, index := range s.ReusedChunks {
			if index > (duration-1)/cfg.ChunkDurationMS {
				return ErrInvalid
			}
		}
	}
	if cursor != len(result.Outputs) {
		return ErrInvalid
	}
	return nil
}
