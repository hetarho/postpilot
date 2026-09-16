package store

import (
	"context"
	"database/sql"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"reflect"
	"time"
)

func (s *Store) SaveCorrection(ctx context.Context, user, id string, revision int, raw string) (clip.Project, error) {
	return s.saveCorrection(ctx, user, id, "", revision, raw)
}

// SaveRevisedPlan is the same save, performed by the revision job that wrote the
// plan: every other active job still refuses it, but the job doing the writing
// is not "busy" against itself (CLIP-131).
func (s *Store) SaveRevisedPlan(ctx context.Context, user, id, job string, revision int, raw string) (clip.Project, error) {
	return s.saveCorrection(ctx, user, id, job, revision, raw)
}

func (s *Store) saveCorrection(ctx context.Context, user, id, job string, revision int, raw string) (clip.Project, error) {
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
		// Compare the semantic plan so a formatting-only save does not renew retention.
		oldPlan, oldErr := clip.DecodeEditPlan(p.EditPlan)
		newPlan, newErr := clip.DecodeEditPlan(raw)
		if oldErr == nil && newErr == nil && reflect.DeepEqual(oldPlan, newPlan) {
			return p, nil
		}
		var oldJSON, newJSON any
		if p.EditPlan == raw || strictJSON(p.EditPlan, &oldJSON) == nil && strictJSON(raw, &newJSON) == nil && reflect.DeepEqual(oldJSON, newJSON) {
			return p, nil
		}
		now := time.Now()

		n, err := q.SaveCorrection(ctx, sqlc.SaveCorrectionParams{EditPlanJson: nullable(raw), UpdatedAt: stamp(now), ID: id, UserID: user, EditPlanRevision: int64(revision)})
		if err != nil {
			return clip.Project{}, err
		}
		if n != 1 {
			return clip.Project{}, clip.ErrPlanConflict
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
		n, err := q.SaveRender(ctx, sqlc.SaveRenderParams{EditPlanJson: nullable(raw), ResultKey: nullable(r.Key), ResultContentType: nullable(r.ContentType), ResultBytes: sql.NullInt64{Int64: r.Bytes, Valid: true}, ResultDurationMs: sql.NullInt64{Int64: int64(r.DurationMS), Valid: true}, ResultCreatedAt: nullable(stamp(r.CreatedAt)), UpdatedAt: stamp(r.CreatedAt), ID: id, UserID: user, EditPlanRevision: int64(revision)})
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
