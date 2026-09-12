package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"github.com/postpilot/backend/internal/platform/config"
)

func renewProjectSources(ctx context.Context, q *sqlc.Queries, user, project string, now time.Time) error {
	if err := q.SetSourceRetention(ctx, sqlc.SetSourceRetentionParams{UserID: user, ProjectID: project, Now: nullable(stamp(now)), ExpiresAt: nullable(stamp(now.Add(config.ClipOriginalRetention)))}); err != nil {
		return err
	}
	return q.RefreshProjectRetention(ctx, sqlc.RefreshProjectRetentionParams{ID: project, UserID: user})
}

func (s *Store) RevokeProjectSources(ctx context.Context, user, project string, now time.Time) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		if err := affected(q.FenceSourceAccess(ctx, sqlc.FenceSourceAccessParams{ID: project, UserID: user, SourceAccessRevokedAt: nullable(stamp(now))})); err != nil {
			return nil, err
		}
		rows, err := q.ListProjectSourceBatches(ctx, sqlc.ListProjectSourceBatchesParams{ProjectID: project, UserID: user})
		if err != nil {
			return nil, err
		}
		var ready []sqlc.ClipSourceBatch
		for _, r := range rows {
			if err := affected(q.MarkSourceCleanup(ctx, sqlc.MarkSourceCleanupParams{ID: r.ID, UserID: user})); err != nil {
				return nil, err
			}
			active, err := q.CountActiveSourceAttempts(ctx, r.ID)
			if err != nil {
				return nil, err
			}
			if active == 0 {
				r.State = "cleanup_pending"
				ready = append(ready, r)
			}
		}
		return sourceRows(ctx, q, ready)
	})
}

func bindSourceAttempt(ctx context.Context, q *sqlc.Queries, user, batch, job string, now time.Time) error {
	b, err := getSourceBatch(ctx, q, user, batch)
	if err != nil {
		return err
	}
	n, err := q.LinkSourceJob(ctx, sqlc.LinkSourceJobParams{JobID: nullable(job), ID: batch, UserID: user, Now: nullable(stamp(now))})
	if err != nil {
		return err
	}
	if n != 1 {
		return clip.ErrSourceState
	}
	raw, err := json.Marshal(clip.SourceManifest(b.Sources))
	if err != nil {
		return err
	}
	if err = q.InsertSourceAttempt(ctx, sqlc.InsertSourceAttemptParams{JobID: job, UserID: user, ProjectID: b.ProjectID, BatchID: batch, ManifestJson: string(raw), BoundAt: stamp(now)}); err != nil {
		return err
	}
	if err = q.SetBoundSourceRetention(ctx, sqlc.SetBoundSourceRetentionParams{BatchID: batch, ExpiresAt: stamp(now.Add(config.ClipOriginalRetention))}); err != nil {
		return err
	}
	return q.RefreshProjectRetention(ctx, sqlc.RefreshProjectRetentionParams{ID: b.ProjectID, UserID: user})
}

func (s *Store) ReleaseSourceAttempt(ctx context.Context, user, job string, terminal time.Time) error {
	if terminal.IsZero() {
		return clip.ErrSourceState
	}
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		a, e := q.GetSourceAttempt(ctx, sqlc.GetSourceAttemptParams{UserID: user, JobID: job})
		if errors.Is(e, sql.ErrNoRows) {
			return struct{}{}, nil
		}
		if e != nil {
			return struct{}{}, e
		}
		if a.ReleasedAt.Valid {
			return struct{}{}, nil
		}
		bound, e := time.Parse(time.RFC3339Nano, a.BoundAt)
		if e != nil {
			return struct{}{}, e
		}
		if terminal.Before(bound) {
			return struct{}{}, clip.ErrSourceState
		}
		if e = q.SetBoundSourceRetention(ctx, sqlc.SetBoundSourceRetentionParams{BatchID: a.BatchID, ExpiresAt: stamp(terminal.Add(config.ClipOriginalRetention))}); e != nil {
			return struct{}{}, e
		}
		if e = affected(q.ReleaseSourceAttempt(ctx, sqlc.ReleaseSourceAttemptParams{UserID: user, JobID: job, ReleasedAt: nullable(stamp(terminal))})); e != nil {
			return struct{}{}, e
		}
		if e = q.RestoreSourceBatch(ctx, a.BatchID); e != nil {
			return struct{}{}, e
		}
		return struct{}{}, q.RefreshProjectRetention(ctx, sqlc.RefreshProjectRetentionParams{ID: a.ProjectID, UserID: user})
	})
	return err
}

// ProjectSourceBatches exposes the selected manifest plus originals still referenced
// by the saved plan. Superseded unrelated selections never become playback capabilities.
func (s *Store) ProjectSourceBatches(ctx context.Context, user, project string) ([]clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) ([]clip.SourceBatch, error) {
		access, e := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: project, UserID: user})
		if e != nil {
			return nil, dbError(e)
		}
		if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid {
			return nil, clip.ErrSourceState
		}
		p, e := getProject(ctx, q, user, project)
		if e != nil {
			return nil, e
		}
		refs := map[string]string{}
		if sources, err := clip.RetainedSources(p); err == nil {
			for _, v := range sources {
				refs[v.ID] = v.Fingerprint
			}
		}
		rows, e := q.ListProjectSourceBatches(ctx, sqlc.ListProjectSourceBatchesParams{ProjectID: project, UserID: user})
		if e != nil {
			return nil, e
		}
		var out []clip.SourceBatch
		for _, r := range rows {
			b, err := sourceBatchRow(ctx, q, r)
			if err != nil {
				return nil, err
			}
			if b.ID == access.SourceBatchID.String {
				b.Current = true
				out = append([]clip.SourceBatch{b}, out...)
				continue
			}
			var retained []clip.SourceLease
			for _, v := range b.Sources {
				if refs[v.ID] == v.Fingerprint {
					retained = append(retained, v)
				}
			}
			if len(retained) > 0 {
				b.Sources = retained
				out = append(out, b)
			}
		}
		return out, nil
	})
}
