package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// SetSourceOriginalAudio records one owner 원본 소리 유지 decision.
//
// Everything the decision touches happens on the serialized writer in ONE
// transaction: the lease that is the owner-facing authority, the saved plan's
// snapshot of it, the plan revision and the retention renewal. A lease that
// disagreed with its plan for even a moment could render audio the owner never
// asked for, which is the one outcome CDS-6 does not allow.
//
// No media or provider work happens here, so the transaction is never held
// across an external call (ARCH-10).
func (s *Store) SetSourceOriginalAudio(ctx context.Context, user string, change clip.SourceAudioChange) (clip.SourceBatch, clip.Project, error) {
	type result struct {
		batch   clip.SourceBatch
		project clip.Project
	}
	out, err := transact(ctx, s, func(q *sqlc.Queries) (result, error) {
		p, err := getProject(ctx, q, user, change.ProjectID)
		if err != nil {
			return result{}, err
		}
		if p.Finalized != nil {
			return result{}, clip.ErrFinalized
		}
		b, err := getSourceBatch(ctx, q, user, change.BatchID)
		if err != nil {
			return result{}, err
		}
		access, err := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: change.ProjectID, UserID: user})
		if err != nil {
			return result{}, dbError(err)
		}
		// A replaced batch, a deleted project and a project whose source access
		// was revoked all keep their own leases; none of them is editable.
		if b.ProjectID != change.ProjectID || access.Deleting != 0 || access.SourceAccessRevokedAt.Valid ||
			access.SourceBatchID.String != change.BatchID || b.State == "cleanup_pending" {
			return result{}, clip.ErrSourceState
		}
		busy, err := q.HasActiveClipJob(ctx, nullable(change.ProjectID))
		if err != nil {
			return result{}, err
		}
		if busy > 0 {
			return result{}, clip.ErrBusy
		}
		found := false
		current := false
		for _, v := range b.Sources {
			if v.ID != change.SourceID || v.Fingerprint != change.Fingerprint {
				continue
			}
			found = true
			if v.CleanupPending || v.State != "ready" {
				return result{}, clip.ErrSourceState
			}
			current = v.RetainOriginalAudio
		}
		if !found {
			return result{}, clip.ErrNotFound
		}
		// The plan the owner was looking at. Zero is the honest expectation
		// before generation has produced one.
		if p.EditPlan == "" {
			if change.ExpectedRevision != 0 {
				return result{}, clip.ErrPlanConflict
			}
		} else if change.ExpectedRevision <= 0 || change.ExpectedRevision != p.EditPlanRevision {
			return result{}, clip.ErrPlanConflict
		}
		// A retried or repeated request changes nothing: not the lease, not the
		// revision, and not the retention window.
		if current == change.RetainOriginal {
			b.Current, b.AccessDenied = true, false
			return result{b, p}, nil
		}
		if err = affected(q.SetSourceOriginalAudio(ctx, sqlc.SetSourceOriginalAudioParams{
			RetainOriginalAudio: flag(change.RetainOriginal), CanonicalID: change.SourceID,
			Fingerprint: change.Fingerprint, BatchID: change.BatchID, UserID: user,
		})); err != nil {
			return result{}, err
		}
		now := time.Now()
		if p.EditPlan != "" {
			plan, styles, e := clip.DecodeEditPlan(p.EditPlan)
			if e != nil {
				return result{}, e
			}
			raw, e := clip.EncodeEditPlan(clip.ApplySourceAudio(plan, change.SourceID, change.Fingerprint, change.RetainOriginal), styles)
			if e != nil {
				return result{}, e
			}
			n, e := q.SaveCorrection(ctx, sqlc.SaveCorrectionParams{EditPlanJson: nullable(raw), UpdatedAt: stamp(now), ID: change.ProjectID, UserID: user, EditPlanRevision: int64(change.ExpectedRevision)})
			if e != nil {
				return result{}, e
			}
			// SaveCorrection advances edit_plan_revision by exactly one and
			// leaves rendered_plan_revision alone, which is what marks the
			// existing result stale for a credit-free rerender (CLIP-20).
			if n != 1 {
				return result{}, clip.ErrPlanConflict
			}
		}
		if err = renewProjectSources(ctx, q, user, change.ProjectID, now); err != nil {
			return result{}, err
		}
		batch, err := getSourceBatch(ctx, q, user, change.BatchID)
		if err != nil {
			return result{}, err
		}
		batch.Current = true
		project, err := getProject(ctx, q, user, change.ProjectID)
		return result{batch, project}, err
	})
	return out.batch, out.project, err
}
