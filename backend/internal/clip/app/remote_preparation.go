package app

import (
	"context"
	"io"
	"reflect"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
)

func (r *generationRun) prepareRemote(revision int) error {
	s := r.s
	r.set("prepare", 0, len(r.b.Sources))
	task := clip.MediaTask{Version: clip.MediaContractVersion}
	for _, source := range r.b.Sources {
		frozen := clip.MediaTaskSource{ID: source.ID, SourceMetadata: source.SourceMetadata}
		if cached, ok := recoverySource(r.recovery, source.ID); ok {
			frozen.Info = cached.Info
			for i := 0; i < chunkCount(s.cfg.Media, cached.Info.DurationMS); i++ {
				if recoveryChunk(r.recovery, source.ID, i) != nil {
					frozen.ReusedChunks = append(frozen.ReusedChunks, i)
				}
			}
		}
		task.Sources = append(task.Sources, frozen)
	}
	stage, artifacts, err := s.remoteMedia.Request(r.ctx, MediaDispatchRequest{UserID: r.user, JobID: r.job, ProjectID: r.project, Revision: revision, Operation: clip.MediaPrepare, Task: task})
	if err != nil {
		return err
	}
	result, err := mediacodec.DecodeResult(stage.AcceptedResult)
	if err != nil {
		return err
	}
	if err = clip.ValidateMediaResult(clip.MediaPrepare, task, result, s.cfg.Media); err != nil {
		return err
	}
	bySlot := map[string]clip.MediaArtifact{}
	for _, a := range artifacts {
		if a.State != "accepted" || a.AttemptID != stage.CurrentAttemptID || bySlot[a.Slot].Slot != "" {
			return clip.ErrInvalidMedia
		}
		bySlot[a.Slot] = a
	}
	if len(bySlot) != len(result.Outputs) {
		return clip.ErrInvalidMedia
	}
	// Check every copy before a hold or provider request, one bounded stream at
	// a time. Conditional object writes keep this verdict stable across reopens.
	for _, out := range result.Outputs {
		a, ok := bySlot[out.Slot]
		if !ok || !reflect.DeepEqual(out, a.MediaOutput) {
			return clip.ErrInvalidMedia
		}
		if a.Info.ContainerDurationMS <= 0 || a.Info.ContainerDurationMS > 60000 {
			return clip.ErrInvalidMedia
		}
		if err = readPreparedArtifact(r.ctx, s.objects, a, io.Discard); err != nil {
			return err
		}
	}
	for i, source := range task.Sources {
		verified := result.Sources[i]
		input := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: source.ID, Fingerprint: source.Fingerprint, Info: verified.Info}, Filename: source.Filename}
		r.sources = append(r.sources, input)
		for index, offset := 0, 0; offset < verified.Info.DurationMS; index, offset = index+1, offset+s.cfg.Media.ChunkDurationMS {
			c := clip.AnalysisChunk{SourceID: source.ID, Fingerprint: source.Fingerprint, Index: index, OffsetMS: offset, DurationMS: min(s.cfg.Media.ChunkDurationMS, verified.Info.DurationMS-offset)}
			if clip.ValidateChunkInput(s.cfg.Analysis, clip.ChunkInput{Source: input, Index: index, OffsetMS: offset, DurationMS: c.DurationMS}) != nil {
				return clip.ErrInvalidMedia
			}
			if old := recoveryChunk(r.recovery, source.ID, index); old != nil {
				if old.Fingerprint != source.Fingerprint || old.OffsetMS != offset || old.DurationMS != c.DurationMS {
					return clip.ErrInvalidMedia
				}
				r.prepared = append(r.prepared, preparedChunk{source: i, chunk: c, reused: old})
			} else {
				a, ok := bySlot[clip.MediaAnalysisSlot(source.ID, index)]
				if !ok {
					return clip.ErrInvalidMedia
				}
				c.Info, c.Bytes = a.Info, a.Bytes
				video := artifactVideo(s.objects, a)
				r.prepared = append(r.prepared, preparedChunk{source: i, chunk: c, video: &video})
			}
		}
	}
	if len(r.prepared) > 49 {
		return clip.ErrInvalidMedia
	}
	r.set("prepare", len(r.sources), len(r.sources))
	return r.admitPrepared()
}

func (s *GenerationService) observePrepared(ctx context.Context, p preparedChunk, input clip.AnalysisSource, policy clip.GenerationPricing, language string) (clip.ChunkAnalysis, error) {
	if p.video == nil {
		return s.observe(ctx, p.chunk, input, policy.Observe, language)
	}
	out, _, err := s.planner.ObserveChunk(ctx, policy.Observe.Ref, clip.ChunkInput{Source: input, Index: p.chunk.Index, OffsetMS: p.chunk.OffsetMS, DurationMS: p.chunk.DurationMS, Video: *p.video, Policy: policy.Observe, Language: language})
	return out, err
}
