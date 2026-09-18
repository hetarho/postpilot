package store

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func browserRender(row sqlc.ClipBrowserRender) (clip.BrowserRender, error) {
	r := clip.BrowserRender{ID: row.ID, UserID: row.UserID, ProjectID: row.ProjectID, Revision: int(row.PlanRevision), Ratio: row.Ratio, DurationMS: int(row.DurationMs), Audio: row.HasAudio != 0, UploadBytes: row.UploadBytes}
	var err error
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err == nil && row.StoredAt.Valid {
		var at time.Time
		at, err = time.Parse(time.RFC3339Nano, row.StoredAt.String)
		r.StoredAt = &at
	}
	if err == nil && row.CancelledAt.Valid {
		var at time.Time
		at, err = time.Parse(time.RFC3339Nano, row.CancelledAt.String)
		r.CancelledAt = &at
	}
	if err == nil && row.VerdictJson.Valid {
		err = json.Unmarshal([]byte(row.VerdictJson.String), &r.Verdict)
	}
	return r, err
}

// Recheck ownership, revision and activity in the same transaction as the
// record. Layout has no write lock, so a save may win while it is being checked.
func checkBrowserProject(ctx context.Context, q *sqlc.Queries, r clip.BrowserRender) error {
	if r.CancelledAt != nil {
		return clip.ErrSourceState
	}
	p, err := getProject(ctx, q, r.UserID, r.ProjectID)
	if err != nil {
		return err
	}
	if p.Finalized != nil {
		return clip.ErrFinalized
	}
	access, err := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: r.ProjectID, UserID: r.UserID})
	if err != nil {
		return err
	}
	if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid {
		return clip.ErrSourceState
	}
	if p.EditPlanRevision != r.Revision {
		return clip.ErrPlanConflict
	}
	busy, err := q.HasActiveClipJob(ctx, nullable(r.ProjectID))
	if err != nil {
		return err
	}
	if busy != 0 {
		return clip.ErrBusy
	}
	return nil
}

func (s *Store) BeginBrowserRender(ctx context.Context, r clip.BrowserRender) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		if err := checkBrowserProject(ctx, q, r); err != nil {
			return struct{}{}, err
		}
		if err := renewProjectSources(ctx, q, r.UserID, r.ProjectID, r.CreatedAt); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, q.BeginBrowserRender(ctx, sqlc.BeginBrowserRenderParams{ID: r.ID, UserID: r.UserID, ProjectID: r.ProjectID, PlanRevision: int64(r.Revision), Ratio: r.Ratio, DurationMs: int64(r.DurationMS), HasAudio: flag(r.Audio), CreatedAt: stamp(r.CreatedAt)})
	})
	return err
}

func (s *Store) GetBrowserRender(ctx context.Context, user, id string) (clip.BrowserRender, error) {
	r, err := s.read.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: id, UserID: user})
	if err != nil {
		return clip.BrowserRender{}, dbError(err)
	}
	return browserRender(r)
}

func (s *Store) SaveBrowserRenderVerdict(ctx context.Context, user, id string, verdict clip.RenderVerdict, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		row, err := q.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: id, UserID: user})
		if err != nil {
			return struct{}{}, err
		}
		r, err := browserRender(row)
		if err != nil {
			return struct{}{}, err
		}
		if err := checkBrowserProject(ctx, q, r); err != nil {
			return struct{}{}, err
		}
		if r.Verdict != nil {
			if !reflect.DeepEqual(*r.Verdict, verdict) {
				return struct{}{}, clip.ErrPlanConflict
			}
			return struct{}{}, nil
		}
		raw, err := json.Marshal(verdict)
		if err != nil {
			return struct{}{}, err
		}
		if err := renewProjectSources(ctx, q, user, r.ProjectID, now); err != nil {
			return struct{}{}, err
		}
		n, err := q.SaveBrowserRenderVerdict(ctx, sqlc.SaveBrowserRenderVerdictParams{ID: id, UserID: user, VerdictJson: nullable(string(raw)), ReportedAt: nullable(stamp(now))})
		return struct{}{}, affected(n, err)
	})
	return err
}
