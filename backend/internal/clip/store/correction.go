package store

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// SaveCorrection saves an owner's correction and, in the same transaction, the
// region slots it changed (CLIP-188); regions is nil when it changed none.
func (s *Store) SaveCorrection(ctx context.Context, user, id string, revision int, raw string, regions *clip.ProjectRegions) (clip.Project, error) {
	return s.saveCorrection(ctx, user, id, "", revision, raw, regions, false)
}

// SaveRevisedPlan is the same save, performed by the revision job that wrote the
// plan: every other active job still refuses it, but the job doing the writing
// is not "busy" against itself (CLIP-131).
func (s *Store) SaveRevisedPlan(ctx context.Context, user, id, job string, revision int, raw string) (clip.Project, error) {
	return s.saveCorrection(ctx, user, id, job, revision, raw, nil, true)
}

// saveCorrection saves an owner's correction, or with `written` a revision's plan, which is a
// writer's plan and so moves the generated revision with it (CLIP-180); a revision rewrites no
// region word, so it draws the project's slots over whatever its template entries held.
func (s *Store) saveCorrection(ctx context.Context, user, id, job string, revision int, raw string, regions *clip.ProjectRegions, written bool) (clip.Project, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.Project, error) {
		p, err := getProject(ctx, q, user, id)
		if err != nil {
			return clip.Project{}, err
		}
		if p.Finalized != nil {
			return clip.Project{}, clip.ErrFinalized
		}
		busy, err := q.HasOtherActiveClipJob(ctx, sqlc.HasOtherActiveClipJobParams{ProjectID: nullable(id), JobID: job})
		if err != nil {
			return clip.Project{}, err
		}
		if busy > 0 {
			return clip.Project{}, clip.ErrBusy
		}
		if revision <= 0 || p.EditPlanRevision != revision {
			return clip.Project{}, clip.ErrPlanConflict
		}
		now := time.Now()
		if written {
			var drawn clip.ProjectRegions
			var projected bool
			if raw, drawn, projected, err = projectWrittenPlan(p, raw, nil); err != nil {
				return clip.Project{}, err
			}
			if err := saveRegionState(ctx, q, p, drawn, now, projected); err != nil {
				return clip.Project{}, err
			}
			regions = &drawn
		} else if regions != nil {
			// The owner's correction changed these slots, so a projection
			// already drew them and they are recorded whatever they were.
			if err := saveRegionState(ctx, q, p, *regions, now, true); err != nil {
				return clip.Project{}, err
			}
		}
		// Compare the semantic plan so a formatting-only save does not renew retention.
		oldPlan, oldErr := clip.DecodeEditPlan(p.EditPlan)
		newPlan, newErr := clip.DecodeEditPlan(raw)
		if newErr != nil {
			return clip.Project{}, newErr
		}
		reused, err := reuseSpeechAssets(ctx, q, user, id, &newPlan)
		if err != nil {
			return clip.Project{}, err
		}
		if reused {
			raw, err = clip.EncodeEditPlan(newPlan)
			if err != nil {
				return clip.Project{}, err
			}
		}
		if err := validateSpeechAssets(ctx, q, user, id, newPlan); err != nil {
			return clip.Project{}, err
		}
		var oldJSON, newJSON any
		if oldErr == nil && newErr == nil && reflect.DeepEqual(oldPlan, newPlan) || p.EditPlan == raw || strictJSON(p.EditPlan, &oldJSON) == nil && strictJSON(raw, &newJSON) == nil && reflect.DeepEqual(oldJSON, newJSON) {
			if regions == nil {
				return p, nil
			}
			return getProject(ctx, q, user, id)
		}

		n, err := q.SaveCorrection(ctx, sqlc.SaveCorrectionParams{EditPlanJson: nullable(raw), UpdatedAt: stamp(now), ID: id, UserID: user, EditPlanRevision: int64(revision)})
		if err != nil {
			return clip.Project{}, err
		}
		if n != 1 {
			return clip.Project{}, clip.ErrPlanConflict
		}
		if written {
			if e := affected(q.MarkGeneratedPlanRevision(ctx, sqlc.MarkGeneratedPlanRevisionParams{ID: id, UserID: user})); e != nil {
				return clip.Project{}, e
			}
		}
		if decoded, e := clip.DecodeEditPlan(raw); e == nil && decoded.Portable != nil {
			p.Composition = &clip.ProjectComposition{Snapshot: decoded.Portable.Snapshot, Inputs: decoded.Portable.Inputs}
			if e = saveComposition(ctx, q, p); e != nil {
				return clip.Project{}, e
			}
		}
		if err = renewProjectSources(ctx, q, user, id, now); err != nil {
			return p, err
		}
		return getProject(ctx, q, user, id)
	})
}
func (s *Store) SaveRender(ctx context.Context, user, id string, revision int, r clip.Result) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		p, err := getProject(ctx, q, user, id)
		if err != nil {
			return struct{}{}, err
		}
		if revision <= 0 || p.EditPlanRevision != revision {
			return struct{}{}, clip.ErrPlanConflict
		}
		if p.Result != nil {
			if err = q.EnqueueObjectDeletion(ctx, sqlc.EnqueueObjectDeletionParams{ObjectKey: p.Result.Key, CreatedAt: stamp(r.CreatedAt)}); err != nil {
				return struct{}{}, err
			}
		}
		raw := p.EditPlan
		if plan, decodeErr := clip.DecodeEditPlan(raw); decodeErr == nil {
			clip.RecomputePlanNotices(&plan, p.TargetDurationMS, 0)
			raw, err = clip.EncodeEditPlan(plan)
			if err != nil {
				return struct{}{}, err
			}
		}
		n, err := q.SaveRender(ctx, sqlc.SaveRenderParams{RenderKind: string(r.RenderKind()), EditPlanJson: nullable(raw), ResultKey: nullable(r.Key), ResultContentType: nullable(r.ContentType), ResultBytes: sql.NullInt64{Int64: r.Bytes, Valid: true}, ResultDurationMs: sql.NullInt64{Int64: int64(r.DurationMS), Valid: true}, ResultCreatedAt: nullable(stamp(r.CreatedAt)), UpdatedAt: stamp(r.CreatedAt), ID: id, UserID: user, EditPlanRevision: int64(revision)})
		if err != nil {
			return struct{}{}, err
		}
		if n != 1 {
			return struct{}{}, clip.ErrPlanConflict
		}
		return struct{}{}, nil
	})
	return err
}
