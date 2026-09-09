package store

import (
	"context"
	"database/sql"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"time"
)

func (s *Store) SaveCorrection(ctx context.Context, user, id string, revision int, raw string) (clip.Project, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.Project, error) {
		p, err := getProject(ctx, q, user, id)
		if err != nil {
			return clip.Project{}, err
		}
		busy, err := q.HasActiveClipJob(ctx, nullable(id))
		if err != nil {
			return clip.Project{}, err
		}
		if busy > 0 {
			return clip.Project{}, clip.ErrBusy
		}
		if revision <= 0 || p.EditPlanRevision != revision {
			return clip.Project{}, clip.ErrPlanConflict
		}
		n, err := q.SaveCorrection(ctx, sqlc.SaveCorrectionParams{EditPlanJson: nullable(raw), UpdatedAt: stamp(time.Now()), ID: id, UserID: user, EditPlanRevision: int64(revision)})
		if err != nil {
			return clip.Project{}, err
		}
		if n != 1 {
			return clip.Project{}, clip.ErrPlanConflict
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
		n, err := q.SaveRender(ctx, sqlc.SaveRenderParams{ResultKey: nullable(r.Key), ResultContentType: nullable(r.ContentType), ResultBytes: sql.NullInt64{Int64: r.Bytes, Valid: true}, ResultDurationMs: sql.NullInt64{Int64: int64(r.DurationMS), Valid: true}, ResultCreatedAt: nullable(stamp(r.CreatedAt)), UpdatedAt: stamp(r.CreatedAt), ID: id, UserID: user, EditPlanRevision: int64(revision)})
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
