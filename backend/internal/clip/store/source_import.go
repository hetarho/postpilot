package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

var _ clip.SourceImportStore = (*Store)(nil)

func initialSourceImport(ctx context.Context, q *sqlc.Queries, user, project, batch string, now time.Time) (clip.SourceBatch, error) {
	p, err := getProject(ctx, q, user, project)
	if err != nil {
		return clip.SourceBatch{}, err
	}
	access, err := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: project, UserID: user})
	if err != nil {
		return clip.SourceBatch{}, dbError(err)
	}
	if p.Finalized != nil || p.Result != nil || p.GeneratedPlanRevision != 0 || p.EditPlanRevision != 0 || p.EditPlan != "" || p.Analysis != "" || p.Storyline != nil || access.Deleting != 0 || access.SourceAccessRevokedAt.Valid || access.SourceBatchID.String != batch {
		return clip.SourceBatch{}, clip.ErrSourceState
	}
	active, err := q.HasActiveClipJob(ctx, nullable(project))
	if err != nil {
		return clip.SourceBatch{}, err
	}
	if active > 0 {
		return clip.SourceBatch{}, clip.ErrBusy
	}
	history, err := q.HasProjectImportWork(ctx, nullable(project))
	if err != nil {
		return clip.SourceBatch{}, err
	}
	if history > 0 {
		return clip.SourceBatch{}, clip.ErrSourceState
	}
	bound, err := q.CountActiveSourceAttempts(ctx, batch)
	if err != nil {
		return clip.SourceBatch{}, err
	}
	if bound > 0 {
		return clip.SourceBatch{}, clip.ErrSourceState
	}
	b, err := getSourceBatch(ctx, q, user, batch)
	if err != nil {
		return b, err
	}
	if b.ProjectID != project || b.State != "uploading" && b.State != "ready" || !now.Before(b.UploadExpiresAt) {
		return b, clip.ErrSourceState
	}
	for _, v := range b.Sources {
		if v.CleanupPending || v.State == "ready" && !now.Before(v.ExpiresAt) {
			return b, clip.ErrSourceState
		}
	}
	b.Current = true
	return b, nil
}
func (s *Store) ImportSourceBatch(ctx context.Context, user, project, batch string, now time.Time) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		return initialSourceImport(ctx, q, user, project, batch, now)
	})
}

func (s *Store) CompleteSourceImport(ctx context.Context, user, project, batch, destination string, now time.Time) (*clip.SourceBatch, error) {
	if destination != clip.CompletedSourceImportID(user, project, batch) {
		return nil, clip.ErrInvalid
	}
	return transact(ctx, s, func(q *sqlc.Queries) (*clip.SourceBatch, error) {
		if _, err := getProject(ctx, q, user, project); err != nil {
			return nil, err
		}
		access, err := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: project, UserID: user})
		if err != nil {
			return nil, dbError(err)
		}
		if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid || access.FinalizedAt.Valid {
			return nil, clip.ErrSourceState
		}
		// Read-only replay never renews a lease, resets material or changes selection.
		if access.SourceBatchID.String == destination {
			b, e := getSourceBatch(ctx, q, user, destination)
			if e != nil {
				return nil, e
			}
			if b.ProjectID != project {
				return nil, clip.ErrSourceState
			}
			b.Current = true
			return &b, nil
		}
		// An all-ready import keeps its original batch ID. A read-only receipt
		// remains valid if the owner began work after its confirmation.
		if access.SourceBatchID.String == batch {
			ready, e := getSourceBatch(ctx, q, user, batch)
			if e != nil {
				return nil, e
			}
			if ready.ProjectID == project && (ready.State == "ready" || ready.State == "consuming") && len(ready.Sources) > 0 {
				usable := true
				for _, source := range ready.Sources {
					if source.State != "ready" || source.CleanupPending || !now.Before(source.ExpiresAt) {
						usable = false
					}
				}
				if usable {
					ready.Current = true
					return &ready, nil
				}
			}
		}

		if !access.SourceBatchID.Valid {
			old, e := getSourceBatch(ctx, q, user, batch)
			if e == nil && old.ProjectID == project && old.State == "cleanup_pending" {
				empty := true
				for _, v := range old.Sources {
					if v.State == "ready" {
						empty = false
					}
				}
				if empty {
					return nil, nil
				}
			}
		}
		b, err := initialSourceImport(ctx, q, user, project, batch, now)
		if err != nil {
			return nil, err
		}
		ready := 0
		for _, v := range b.Sources {
			if v.State == "ready" {
				ready++
			}
		}
		if ready == len(b.Sources) && ready > 0 {
			return &b, nil
		}
		// Free the unique active-project slot first. The entire transition rolls back
		// if creating the receipt or moving any lease fails.
		if err = affected(q.MarkSourceCleanup(ctx, sqlc.MarkSourceCleanupParams{ID: batch, UserID: user})); err != nil {
			return nil, err
		}
		next := nullable("")
		if ready > 0 {
			err = q.InsertSourceBatch(ctx, sqlc.InsertSourceBatchParams{ID: destination, UserID: user, ProjectID: project, State: "ready", CreatedAt: stamp(b.CreatedAt), ExpiresAt: stamp(b.UploadExpiresAt), PutExpiresAt: stamp(b.PutExpiresAt)})
			if err != nil {
				return nil, err
			}
			n, e := q.MoveConfirmedImportSources(ctx, sqlc.MoveConfirmedImportSourcesParams{DestinationID: destination, BatchID: batch, UserID: user})
			if e != nil {
				return nil, e
			}
			if n != int64(ready) {
				return nil, clip.ErrSourceState
			}
			next = nullable(destination)
		}
		if err = q.MarkBatchLeasesCleanup(ctx, batch); err != nil {
			return nil, err
		}
		if err = affected(q.SelectSourceBatch(ctx, sqlc.SelectSourceBatchParams{SourceBatchID: next, ID: project, UserID: user})); err != nil {
			return nil, err
		}
		if err = q.RefreshProjectRetention(ctx, sqlc.RefreshProjectRetentionParams{ID: project, UserID: user}); err != nil {
			return nil, err
		}
		if ready == 0 {
			return nil, nil
		}
		out, err := getSourceBatch(ctx, q, user, destination)
		out.Current = true
		return &out, err
	})
}
