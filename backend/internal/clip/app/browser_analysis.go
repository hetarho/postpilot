package app

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"io"
	"reflect"
)

func (a *AnalysisPreparations) StartBinding(ctx context.Context, user string, q clip.GenerationQuote, id string, revision int, digest string) error {
	if q.AnalysisPreparationID != id {
		return clip.ErrQuoteChanged
	}
	if id == "" {
		return nil
	}
	return WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		s, e := a.owned(ctx, p, user, id)
		if e != nil {
			return e
		}
		if s.State != "preparing" || s.ProjectID != q.ProjectID || s.BatchID != q.BatchID || s.QuoteID != q.ID || s.ExpectedRevision != revision || s.RecoveryDigest != q.Pricing.RecoveryDigest || s.OriginalDigest != digest || s.BoundQuoteDigest != q.InputDigest || q.InputDigest != clip.AnalysisBoundQuoteDigest(digest, s.QuoteManifestDigest) {
			return clip.ErrQuoteChanged
		}
		return nil
	})
}

// Bind precedes activation. No dispatcher can read a browser parent whose
// quote/source/session linkage was only partially committed.
func (a *AnalysisPreparations) Bind(ctx context.Context, user, id, parent string) error {
	return WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		s, e := a.owned(ctx, p, user, id)
		if e != nil {
			return e
		}
		j, e := p.Jobs.GetByID(ctx, parent)
		if e != nil {
			return e
		}
		if j.UserID != user || j.Subject(clip.JobSubject) != s.ProjectID || !clip.PreparesMedia(j.Kind) || j.Status != job.StatusQueued || j.DispatchReady || j.CancelRequestedAt != nil {
			return clip.ErrAnalysisPreparationState
		}
		var f clip.GenerationPayload
		if clip.StrictJSON(string(j.Payload), &f) != nil || f.AnalysisPreparationID != id || f.Approval == nil || f.Approval.QuoteID != s.QuoteID || f.Batch.ID != s.BatchID || f.ProjectID != s.ProjectID {
			return clip.ErrQuoteChanged
		}
		return p.Analysis.BindAnalysisPreparation(ctx, s, parent, a.now().UTC())
	})
}
func (a *AnalysisPreparations) forJob(ctx context.Context, user, id, parent string) (out clip.AnalysisPreparation, err error) {
	pending := false
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		var e error
		out, e = a.owned(ctx, p, user, id)
		if e != nil {
			return e
		}
		if out.ParentJobID != parent {
			return clip.ErrAnalysisPreparationState
		}
		switch out.State {
		case "preparing", "verifying":
			wait, e := p.Waits.Continuation(ctx, parent)
			if errors.Is(e, job.ErrInvalidWait) {
				if e = p.Waits.Park(ctx, parent, AnalysisPreparationWaitKey(id), job.FailOnInterrupt, a.now().UTC()); e != nil {
					return e
				}
			} else if e != nil {
				return e
			} else if wait.WaitKey != AnalysisPreparationWaitKey(id) {
				return job.ErrInvalidWait
			}
			pending = true
			return p.Waits.UpdateProgress(ctx, parent, "prepare", out.Progress*len(out.Copies)/1000, len(out.Copies), a.now().UTC())
		case "accepted", "consumed":
			if len(out.Copies) > 0 && !a.qualified(out.ProfileVersion) {
				return clip.ErrAnalysisProfileUnqualified
			}
			return nil
		default:
			return clip.ErrAnalysisPreparationState
		}
	})
	if err == nil && pending {
		err = job.ErrYield
	}
	return
}
func (r *generationRun) prepareBrowser() error {
	a := r.s.analysisPreparations
	if a == nil {
		return clip.ErrMediaUnsupported
	}
	r.set("prepare", 0, len(r.b.Sources))
	s, e := a.forJob(r.ctx, r.user, r.p.AnalysisPreparationID, r.job)
	if e != nil {
		return e
	}
	if s.ProfileVersion != clip.BrowserAnalysisProfileVersion || len(s.Sources) != len(r.b.Sources) || s.RecoveryDigest != r.pricing.RecoveryDigest || len(s.Reused) != r.pricing.ReusedChunks {
		return clip.ErrQuoteChanged
	}
	r.sources = s.Sources
	bySlot := map[string]clip.AnalysisCopy{}
	for _, c := range s.Copies {
		if c.State != "verified" || bySlot[c.Slot].Slot != "" {
			return clip.ErrInvalidMedia
		}
		bySlot[c.Slot] = c
		if e = readPreparedArtifact(r.ctx, r.s.objects, browserAnalysisArtifact(c), io.Discard); e != nil {
			return e
		}
	}
	used := 0
	for i, source := range s.Sources {
		for index, offset := 0, 0; offset < source.Info.DurationMS; index, offset = index+1, offset+r.s.cfg.Media.ChunkDurationMS {
			c := clip.AnalysisChunk{SourceID: source.ID, Fingerprint: source.Fingerprint, Index: index, OffsetMS: offset, DurationMS: min(r.s.cfg.Media.ChunkDurationMS, source.Info.DurationMS-offset)}
			if old := recoveryChunk(r.recovery, source.ID, index); old != nil {
				if old.Fingerprint != c.Fingerprint || old.OffsetMS != c.OffsetMS || old.DurationMS != c.DurationMS || !containsAnalysisReuse(s.Reused, c) {
					return clip.ErrInvalidMedia
				}
				r.prepared = append(r.prepared, preparedChunk{source: i, chunk: c, reused: old})
			} else {
				copy, ok := bySlot[clip.MediaAnalysisSlot(source.ID, index)]
				if !ok || copy.Fingerprint != source.Fingerprint || copy.OffsetMS != offset || copy.DurationMS != c.DurationMS {
					return clip.ErrInvalidMedia
				}
				c.Bytes, c.Info = copy.Bytes, copy.Info
				video := artifactVideo(r.s.objects, browserAnalysisArtifact(copy))
				r.prepared = append(r.prepared, preparedChunk{source: i, chunk: c, video: &video})
				used++
			}
		}
	}
	if used != len(bySlot) || len(r.prepared) > clip.AnalysisPreparationMaxCopies {
		return clip.ErrInvalidMedia
	}
	e = WriteTx(r.ctx, a.writer, a.bind, func(p Ports) error {
		current, e := a.owned(r.ctx, p, r.user, s.ID)
		if e != nil {
			return e
		}
		if current.ParentJobID != r.job || current.ManifestDigest != s.ManifestDigest || !reflect.DeepEqual(current.Copies, s.Copies) {
			return clip.ErrAnalysisPreparationState
		}
		return p.Analysis.ConsumeAnalysisPreparation(r.ctx, s.ID, r.job, a.now().UTC())
	})
	if e != nil {
		return e
	}
	r.set("prepare", len(r.sources), len(r.sources))
	return r.admitPrepared()
}
func containsAnalysisReuse(reuse []clip.AnalysisChunk, c clip.AnalysisChunk) bool {
	for _, v := range reuse {
		if v.SourceID == c.SourceID && v.Fingerprint == c.Fingerprint && v.Index == c.Index && v.OffsetMS == c.OffsetMS && v.DurationMS == c.DurationMS {
			return true
		}
	}
	return false
}
func browserAnalysisArtifact(c clip.AnalysisCopy) clip.MediaArtifact {
	return clip.MediaArtifact{MediaOutput: clip.MediaOutput{Slot: c.Slot, SourceID: c.SourceID, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS, Bytes: c.Bytes, ContentType: "video/mp4", Digest: c.Digest, Info: c.Info}, ObjectKey: c.ObjectKey, State: "accepted"}
}
