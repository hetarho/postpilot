package store

import (
	"context"
	"database/sql"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"time"
)

var _ clip.GenerationStore = (*Store)(nil)

func (s *Store) LinkSourceJob(ctx context.Context, user, batch, job string, now time.Time) error {
	return s.linkSourceJob(ctx, user, batch, job, 0, now)
}
func (s *Store) LinkRenderSourceJob(ctx context.Context, user, batch, job string, revision int, now time.Time) error {
	if revision <= 0 {
		return clip.ErrPlanConflict
	}
	return s.linkSourceJob(ctx, user, batch, job, revision, now)
}
func (s *Store) linkSourceJob(ctx context.Context, user, batch, job string, revision int, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		b, err := getSourceBatch(ctx, q, user, batch)
		if err != nil {
			return struct{}{}, err
		}
		deleting, err := q.SourceProjectWritable(ctx, sqlc.SourceProjectWritableParams{ID: b.ProjectID, UserID: user})
		if err != nil {
			return struct{}{}, err
		}
		if deleting != 0 {
			return struct{}{}, clip.ErrBusy
		}
		if revision > 0 {
			p, e := getProject(ctx, q, user, b.ProjectID)
			if e != nil {
				return struct{}{}, e
			}
			if p.EditPlanRevision != revision {
				return struct{}{}, clip.ErrPlanConflict
			}
		}
		n, err := q.LinkSourceJob(ctx, sqlc.LinkSourceJobParams{JobID: nullable(job), ID: batch, UserID: user, ExpiresAt: stamp(now)})
		if err != nil {
			return struct{}{}, err
		}
		if n != 1 {
			return struct{}{}, clip.ErrSourceState
		}
		return struct{}{}, nil
	})
	return err
}
func (s *Store) BatchForJob(ctx context.Context, user, job string) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		r, err := q.BatchForJob(ctx, sqlc.BatchForJobParams{UserID: user, JobID: nullable(job)})
		if err != nil {
			return clip.SourceBatch{}, err
		}
		return sourceBatchRow(ctx, q, r)
	})
}
func (s *Store) ListConsumingBatches(ctx context.Context) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		rows, err := q.ListConsumingBatches(ctx)
		if err != nil {
			return nil, err
		}
		return sourceRows(ctx, q, rows)
	})
}
func (s *Store) AddProxy(ctx context.Context, user, batch, key string) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		b, err := getSourceBatch(ctx, q, user, batch)
		if err != nil {
			return struct{}{}, err
		}
		if b.State != "consuming" {
			return struct{}{}, clip.ErrSourceState
		}
		return struct{}{}, q.AddProxy(ctx, sqlc.AddProxyParams{ObjectKey: key, BatchID: batch})
	})
	return err
}
func (s *Store) RemoveProxy(ctx context.Context, key string) error {
	return s.write.RemoveProxy(ctx, key)
}
func (s *Store) SaveGeneration(ctx context.Context, user, id, analysis, plan string, r clip.Result) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		old, err := getProject(ctx, q, user, id)
		if err != nil {
			return struct{}{}, err
		}
		if old.Result != nil {
			if err = q.EnqueueObjectDeletion(ctx, sqlc.EnqueueObjectDeletionParams{ObjectKey: old.Result.Key, CreatedAt: stamp(r.CreatedAt)}); err != nil {
				return struct{}{}, err
			}
		}
		n, err := q.SaveGeneration(ctx, sqlc.SaveGenerationParams{AnalysisJson: nullable(analysis), EditPlanJson: nullable(plan), ResultKey: nullable(r.Key), ResultContentType: nullable(r.ContentType), ResultBytes: sql.NullInt64{Int64: r.Bytes, Valid: true}, ResultDurationMs: sql.NullInt64{Int64: int64(r.DurationMS), Valid: true}, ResultCreatedAt: nullable(stamp(r.CreatedAt)), UpdatedAt: stamp(r.CreatedAt), UserID: user, ID: id})
		return struct{}{}, affected(n, err)
	})
	return err
}
func (s *Store) DeletionKeys(ctx context.Context) ([]string, error) { return s.read.DeletionKeys(ctx) }
func (s *Store) RemoveDeletion(ctx context.Context, key string) error {
	return s.write.RemoveDeletion(ctx, key)
}
func (s *Store) ResultKeys(ctx context.Context) ([]string, error) {
	rows, err := s.read.ResultKeys(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, key := range rows {
		keys = append(keys, key.String)
	}
	return keys, nil
}
