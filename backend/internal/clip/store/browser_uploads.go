package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// Cancellation and promotion serialize on the same writer. Once cancellation
// wins, no delayed upload/report/completion can change the project's result.
func (s *Store) CancelBrowserRender(ctx context.Context, user, id string, now time.Time) (bool, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (bool, error) {
		row, err := q.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: id, UserID: user})
		if err != nil {
			return false, err
		}
		if row.StoredAt.Valid {
			return false, nil
		}
		if row.CancelledAt.Valid {
			return true, nil
		}
		n, err := q.CancelBrowserRender(ctx, sqlc.CancelBrowserRenderParams{ID: id, UserID: user, CancelledAt: nullable(stamp(now))})
		return n == 1, err
	})
}

func (s *Store) ReserveBrowserRenderUpload(ctx context.Context, user, id string, bytes int64, now time.Time) error {
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
		if bytes <= 0 || r.StoredAt != nil || r.Verdict != nil {
			return struct{}{}, clip.ErrSourceState
		}
		if r.UploadBytes != 0 {
			if r.UploadBytes != bytes {
				return struct{}{}, clip.ErrPlanConflict
			}
			return struct{}{}, nil
		}
		if err := renewProjectSources(ctx, q, user, r.ProjectID, now); err != nil {
			return struct{}{}, err
		}
		n, err := q.ReserveBrowserRenderUpload(ctx, sqlc.ReserveBrowserRenderUploadParams{ID: id, UserID: user, UploadBytes: bytes})
		return struct{}{}, affected(n, err)
	})
	return err
}

// Promotion and its idempotence marker share the writer transaction. A failed
// HEAD, verdict, concurrent edit or transaction leaves the prior result intact.
func (s *Store) CompleteBrowserRender(ctx context.Context, user, id string, now time.Time) (string, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (string, error) {
		row, err := q.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: id, UserID: user})
		if err != nil {
			return "", err
		}
		r, err := browserRender(row)
		if err != nil {
			return "", err
		}
		if r.StoredAt != nil {
			return r.ProjectID, nil
		}
		if err := checkBrowserProject(ctx, q, r); err != nil {
			return "", err
		}
		if r.UploadBytes <= 0 || r.Verdict == nil || !r.Verdict.Passed {
			return "", clip.ErrSourceState
		}
		if err := renewProjectSources(ctx, q, user, r.ProjectID, now); err != nil {
			return "", err
		}
		m := r.Verdict.Measurements
		result := clip.Result{Kind: clip.RenderBrowser, Key: r.ResultKey(), ContentType: "video/mp4", Bytes: r.UploadBytes, DurationMS: int((int64(m.VideoFrames)*1000*int64(m.FrameRateDenominator) + int64(m.FrameRateNumerator)/2) / int64(m.FrameRateNumerator)), CreatedAt: now}
		bound := &Store{read: q, write: q}
		if err := bound.SaveRender(ctx, user, r.ProjectID, r.Revision, result); err != nil {
			return "", err
		}
		n, err := q.CompleteBrowserRender(ctx, sqlc.CompleteBrowserRenderParams{ID: id, UserID: user, StoredAt: nullable(stamp(now))})
		if err := affected(n, err); err != nil {
			return "", err
		}
		return r.ProjectID, nil
	})
}
