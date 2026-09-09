package store

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"time"
)

var _ clip.SourceStore = (*Store)(nil)

func sourceBatchRow(ctx context.Context, q *sqlc.Queries, r sqlc.ClipSourceBatch) (clip.SourceBatch, error) {
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return clip.SourceBatch{}, err
	}
	expires, err := time.Parse(time.RFC3339Nano, r.ExpiresAt)
	if err != nil {
		return clip.SourceBatch{}, err
	}
	out := clip.SourceBatch{ID: r.ID, UserID: r.UserID, ProjectID: r.ProjectID, State: r.State, JobID: r.JobID.String, CreatedAt: created, ExpiresAt: expires}
	proxies, err := q.ListBatchProxies(ctx, r.ID)
	if err != nil {
		return out, err
	}
	out.ProxyKeys = proxies
	rows, err := q.ListSourceLeases(ctx, sqlc.ListSourceLeasesParams{BatchID: r.ID, UserID: r.UserID})
	if err != nil {
		return out, err
	}
	for _, v := range rows {
		out.Sources = append(out.Sources, clip.SourceLease{ID: v.ID, Key: v.ObjectKey, State: v.State, ActualBytes: v.ActualBytes, SourceMetadata: clip.SourceMetadata{Filename: v.Filename, ContentType: v.ContentType, Fingerprint: v.Fingerprint, Bytes: v.DeclaredBytes, DurationMS: int(v.DurationMs), Width: int(v.Width), Height: int(v.Height)}})
	}
	return out, nil
}
func getSourceBatch(ctx context.Context, q *sqlc.Queries, user, id string) (clip.SourceBatch, error) {
	r, err := q.GetSourceBatch(ctx, sqlc.GetSourceBatchParams{ID: id, UserID: user})
	if err != nil {
		return clip.SourceBatch{}, dbError(err)
	}
	return sourceBatchRow(ctx, q, r)
}
func sourceRows(ctx context.Context, q *sqlc.Queries, rows []sqlc.ClipSourceBatch) ([]clip.SourceBatch, error) {
	out := make([]clip.SourceBatch, 0, len(rows))
	for _, row := range rows {
		v, err := sourceBatchRow(ctx, q, row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func markProjectSources(ctx context.Context, q *sqlc.Queries, user, project string) ([]clip.SourceBatch, error) {
	rows, err := q.ListProjectSourceBatches(ctx, sqlc.ListProjectSourceBatchesParams{ProjectID: project, UserID: user})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.State == "consuming" {
			return nil, clip.ErrSourceState
		}
	}
	for i := range rows {
		if err := affected(q.MarkSourceCleanup(ctx, sqlc.MarkSourceCleanupParams{ID: rows[i].ID, UserID: user})); err != nil {
			return nil, err
		}
		rows[i].State = "cleanup_pending"
	}
	return sourceRows(ctx, q, rows)
}
func (s *Store) ReplaceSourceBatch(ctx context.Context, b clip.SourceBatch) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		deleting, err := q.SourceProjectWritable(ctx, sqlc.SourceProjectWritableParams{ID: b.ProjectID, UserID: b.UserID})
		if err != nil {
			return nil, dbError(err)
		}
		if deleting != 0 {
			return nil, clip.ErrSourceState
		}
		old, err := markProjectSources(ctx, q, b.UserID, b.ProjectID)
		if err != nil {
			return nil, err
		}
		if err := q.InsertSourceBatch(ctx, sqlc.InsertSourceBatchParams{ID: b.ID, UserID: b.UserID, ProjectID: b.ProjectID, State: b.State, CreatedAt: stamp(b.CreatedAt), ExpiresAt: stamp(b.ExpiresAt)}); err != nil {
			return nil, err
		}
		for i, v := range b.Sources {
			if err := q.InsertSourceLease(ctx, sqlc.InsertSourceLeaseParams{ID: v.ID, BatchID: b.ID, UserID: b.UserID, ObjectKey: v.Key, Filename: v.Filename, ContentType: v.ContentType, Fingerprint: v.Fingerprint, DeclaredBytes: v.Bytes, DurationMs: int64(v.DurationMS), Width: int64(v.Width), Height: int64(v.Height), State: v.State, Ordinal: int64(i)}); err != nil {
				return nil, err
			}
		}
		return old, nil
	})
}
func (s *Store) GetSourceBatch(ctx context.Context, user, id string) (clip.SourceBatch, error) {
	// One read transaction keeps the batch state and its leases from different revisions.
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) { return getSourceBatch(ctx, q, user, id) })
}
func (s *Store) ConfirmSourceLease(ctx context.Context, user, batch, id string, actual int64, now time.Time) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		b, err := getSourceBatch(ctx, q, user, batch)
		if err != nil {
			return b, err
		}
		if (b.State != "uploading" && b.State != "ready") || !now.Before(b.ExpiresAt) {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		found := false
		for _, v := range b.Sources {
			if v.ID == id {
				found = true
				if actual != v.Bytes {
					return clip.SourceBatch{}, clip.ErrInvalid
				}
			}
		}
		if !found {
			return clip.SourceBatch{}, clip.ErrNotFound
		}
		if err := affected(q.SetSourceLeaseReady(ctx, sqlc.SetSourceLeaseReadyParams{ActualBytes: actual, ID: id, BatchID: batch, UserID: user})); err != nil {
			return clip.SourceBatch{}, err
		}
		if err := q.MarkSourceBatchReady(ctx, sqlc.MarkSourceBatchReadyParams{ID: batch, UserID: user}); err != nil {
			return clip.SourceBatch{}, err
		}
		return getSourceBatch(ctx, q, user, batch)
	})
}
func (s *Store) MarkSourceCleanup(ctx context.Context, user, id string, allowConsuming bool) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		b, err := getSourceBatch(ctx, q, user, id)
		if err != nil {
			return b, err
		}
		if b.State == "consuming" && !allowConsuming {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		if err := affected(q.MarkSourceCleanup(ctx, sqlc.MarkSourceCleanupParams{ID: id, UserID: user})); err != nil {
			return clip.SourceBatch{}, err
		}
		b.State = "cleanup_pending"
		return b, nil
	})
}
func (s *Store) BeginProjectSourceCleanup(ctx context.Context, user, project string) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		if err := affected(q.FenceSourceProjectDeletion(ctx, sqlc.FenceSourceProjectDeletionParams{ID: project, UserID: user})); err != nil {
			return nil, err
		}
		return markProjectSources(ctx, q, user, project)
	})
}
func (s *Store) ReapSourceBatches(ctx context.Context, now time.Time) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		if err := q.MarkExpiredSources(ctx, stamp(now)); err != nil {
			return nil, err
		}
		rows, err := q.ListSourceCleanup(ctx)
		if err != nil {
			return nil, err
		}
		return sourceRows(ctx, q, rows)
	})
}
func (s *Store) RemoveSourceBatch(ctx context.Context, user, id string) error {
	return s.write.RemoveSourceBatch(ctx, sqlc.RemoveSourceBatchParams{ID: id, UserID: user})
}
func (s *Store) SourceKeyExists(ctx context.Context, key string) (bool, error) {
	return s.read.SourceKeyExists(ctx, key)
}
