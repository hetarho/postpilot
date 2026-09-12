package store

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"github.com/postpilot/backend/internal/platform/config"
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
	out := clip.SourceBatch{ID: r.ID, UserID: r.UserID, ProjectID: r.ProjectID, State: r.State, JobID: r.JobID.String, CreatedAt: created, ExpiresAt: expires, UploadExpiresAt: expires}
	out.PutExpiresAt, err = time.Parse(time.RFC3339Nano, r.PutExpiresAt)
	if err != nil {
		return out, err
	}
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
		out.Sources = append(out.Sources, clip.SourceLease{ID: v.CanonicalID, Key: v.ObjectKey, State: v.State, CleanupPending: v.CleanupPending != 0, ActualBytes: v.ActualBytes, SourceMetadata: clip.SourceMetadata{Filename: v.Filename, ContentType: v.ContentType, Fingerprint: v.Fingerprint, Bytes: v.DeclaredBytes, DurationMS: int(v.DurationMs), Width: int(v.Width), Height: int(v.Height)}})

		if v.RetentionExpiresAt.Valid {
			at, e := time.Parse(time.RFC3339Nano, v.RetentionExpiresAt.String)
			if e != nil {
				return out, e
			}
			out.Sources[len(out.Sources)-1].ExpiresAt = at
		}
	}
	if out.State == "ready" || out.State == "consuming" {
		out.ExpiresAt = time.Time{}
		for _, source := range out.Sources {
			if out.ExpiresAt.IsZero() || source.ExpiresAt.Before(out.ExpiresAt) {
				out.ExpiresAt = source.ExpiresAt
			}
		}
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
		p, err := getProject(ctx, q, b.UserID, b.ProjectID)
		if err != nil {
			return nil, err
		}
		canonical := map[string]string{}
		if refs, e := clip.RetainedSources(p); e == nil {
			for _, ref := range refs {
				canonical[ref.Fingerprint] = ref.ID
			}
		}
		if err := q.InsertSourceBatch(ctx, sqlc.InsertSourceBatchParams{ID: b.ID, UserID: b.UserID, ProjectID: b.ProjectID, State: b.State, CreatedAt: stamp(b.CreatedAt), ExpiresAt: stamp(b.ExpiresAt), PutExpiresAt: stamp(b.PutExpiresAt)}); err != nil {
			return nil, err
		}
		for i, v := range b.Sources {
			id := v.ID
			if prior := canonical[v.Fingerprint]; prior != "" {
				id = prior
			}
			if err := q.InsertSourceLease(ctx, sqlc.InsertSourceLeaseParams{ID: v.ID, CanonicalID: id, BatchID: b.ID, UserID: b.UserID, ObjectKey: v.Key, Filename: v.Filename, ContentType: v.ContentType, Fingerprint: v.Fingerprint, DeclaredBytes: v.Bytes, DurationMs: int64(v.DurationMS), Width: int64(v.Width), Height: int64(v.Height), State: v.State, Ordinal: int64(i)}); err != nil {
				return nil, err
			}
		}
		return nil, affected(q.SelectSourceBatch(ctx, sqlc.SelectSourceBatchParams{SourceBatchID: nullable(b.ID), ID: b.ProjectID, UserID: b.UserID}))
	})
}
func (s *Store) GetSourceBatch(ctx context.Context, user, id string) (clip.SourceBatch, error) {
	// One read transaction keeps the batch state and its leases from different revisions.
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		b, e := getSourceBatch(ctx, q, user, id)
		if e != nil {
			return b, e
		}
		access, e := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: b.ProjectID, UserID: user})
		if e != nil {
			b.AccessDenied = true
			return b, nil
		}
		b.Current = access.SourceBatchID.String == b.ID
		b.AccessDenied = access.Deleting != 0 || access.SourceAccessRevokedAt.Valid || !b.Current
		return b, nil
	})
}
func (s *Store) ConfirmSourceLease(ctx context.Context, user, batch, id string, actual int64, now time.Time) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		b, err := getSourceBatch(ctx, q, user, batch)
		if err != nil {
			return b, err
		}
		access, e := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: b.ProjectID, UserID: user})
		if e != nil {
			return b, dbError(e)
		}
		if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid || access.SourceBatchID.String != batch || (b.State != "uploading" && b.State != "ready") {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		found := false
		for _, v := range b.Sources {
			if v.ID == id {
				found = true
				if v.CleanupPending {
					return b, clip.ErrSourceState
				}
				if v.State == "ready" {
					if !now.Before(v.ExpiresAt) {
						return b, clip.ErrSourceState
					}
					return b, nil
				}
				if !now.Before(b.UploadExpiresAt) {
					return b, clip.ErrSourceState
				}
				if actual != v.Bytes {
					return clip.SourceBatch{}, clip.ErrInvalid
				}
			}
		}
		if !found {
			return clip.SourceBatch{}, clip.ErrNotFound
		}
		if err := affected(q.SetSourceLeaseReady(ctx, sqlc.SetSourceLeaseReadyParams{ActualBytes: actual, RetentionExpiresAt: nullable(stamp(now.Add(config.ClipOriginalRetention))), CanonicalID: id, BatchID: batch, UserID: user})); err != nil {
			return clip.SourceBatch{}, err
		}
		if err := q.MarkSourceBatchReady(ctx, sqlc.MarkSourceBatchReadyParams{ID: batch, UserID: user}); err != nil {
			return clip.SourceBatch{}, err
		}
		if err := renewProjectSources(ctx, q, user, b.ProjectID, now); err != nil {
			return b, err
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
		if b.State == "consuming" {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		n, e := q.CountActiveSourceAttempts(ctx, b.ID)
		if e != nil {
			return b, e
		}
		if n != 0 {
			return b, clip.ErrSourceState
		}
		p, e := getProject(ctx, q, user, b.ProjectID)
		if e == nil {
			if refs, err := clip.RetainedSources(p); err == nil {
				for _, ref := range refs {
					for _, v := range b.Sources {
						if v.State == "ready" && !v.CleanupPending && ref.ID == v.ID && ref.Fingerprint == v.Fingerprint {
							return b, clip.ErrSourceState
						}
					}
				}
			}
		}

		if err := affected(q.MarkSourceCleanup(ctx, sqlc.MarkSourceCleanupParams{ID: id, UserID: user})); err != nil {
			return clip.SourceBatch{}, err
		}
		if err := q.MarkBatchLeasesCleanup(ctx, b.ID); err != nil {
			return b, err
		}
		b.State = "cleanup_pending"
		return b, nil
	})
}
func (s *Store) BeginProjectSourceCleanup(ctx context.Context, user, project string) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		if err := affected(q.FenceSourceAccess(ctx, sqlc.FenceSourceAccessParams{SourceAccessRevokedAt: nullable(stamp(time.Now())), ID: project, UserID: user})); err != nil {
			return nil, err
		}
		if err := affected(q.FenceSourceProjectDeletion(ctx, sqlc.FenceSourceProjectDeletionParams{ID: project, UserID: user})); err != nil {
			return nil, err
		}
		return markProjectSources(ctx, q, user, project)
	})
}
func (s *Store) ReapSourceBatches(ctx context.Context, now time.Time) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		if err := q.MarkExpiredSourceLeases(ctx, nullable(stamp(now))); err != nil {
			return nil, err
		}
		if err := q.MarkExpiredSources(ctx, stamp(now)); err != nil {
			return nil, err
		}
		rows, err := q.ListSourceCleanup(ctx)
		if err != nil {
			return nil, err
		}
		partial, err := q.ListPartialSourceCleanup(ctx)
		if err != nil {
			return nil, err
		}
		rows = append(rows, partial...)
		return sourceRows(ctx, q, rows)
	})
}
func (s *Store) RemoveSourceBatch(ctx context.Context, user, id string, now time.Time) error {
	return s.write.RemoveSourceBatch(ctx, sqlc.RemoveSourceBatchParams{ID: id, UserID: user, PutExpiresAt: stamp(now)})
}
func (s *Store) SourceKeyExists(ctx context.Context, key string) (bool, error) {
	return s.read.SourceKeyExists(ctx, key)
}
