package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// RecordFinalization is bound to the coordinator's job-exclusion transaction.
func (s *Store) RecordFinalization(ctx context.Context, req clip.FinalizationRequest, now time.Time) (clip.Project, error) {
	if s.writer != nil {
		return clip.Project{}, errors.New("clip finalization requires a coordinated transaction")
	}
	p, err := getProject(ctx, s.write, req.UserID, req.ProjectID)
	if err != nil {
		return p, err
	}
	if plan, err := clip.DecodeEditPlan(p.EditPlan); err == nil {
		if err = validateSpeechAssets(ctx, s.write, req.UserID, req.ProjectID, plan); err != nil {
			return clip.Project{}, err
		}
	}
	// Freeze any legacy hydration before template deletion may clear its reference.
	if p.Composition != nil {
		if err := saveComposition(ctx, s.write, p); err != nil {
			return clip.Project{}, err
		}
	}
	n, err := s.write.FinalizeClipProject(ctx, sqlc.FinalizeClipProjectParams{ID: req.ProjectID, UserID: req.UserID, Now: nullable(stamp(now)), ExpectedRevision: int64(req.ExpectedRevision), ExpectedResultID: nullable(req.ExpectedResultID), PublishedUrl: nullable(req.PublishedURL)})
	if err != nil {
		return clip.Project{}, dbError(err)
	}
	if n != 1 {
		return clip.Project{}, clip.ErrFinalizationConflict
	}
	if err := collectFinalizedSpeech(ctx, s.write, p); err != nil {
		return clip.Project{}, err
	}
	if _, err := s.RevokeProjectSources(ctx, req.UserID, req.ProjectID, now); err != nil {
		return clip.Project{}, err
	}
	return getProject(ctx, s.write, req.UserID, req.ProjectID)
}

// RecordConfirmation is bound to the coordinator's job-exclusion transaction (CLIP-75).
func (s *Store) RecordConfirmation(ctx context.Context, req clip.ConfirmationRequest, now time.Time) (clip.Project, error) {
	if s.writer != nil {
		return clip.Project{}, errors.New("clip confirmation requires a coordinated transaction")
	}
	n, err := s.write.ConfirmClipProject(ctx, sqlc.ConfirmClipProjectParams{ID: req.ProjectID, UserID: req.UserID, Now: sql.NullString{String: stamp(now), Valid: true}, ExpectedRevision: sql.NullInt64{Int64: int64(req.ExpectedPlanRevision), Valid: true}})
	if err != nil {
		return clip.Project{}, dbError(err)
	}
	if n != 1 {
		return clip.Project{}, clip.ErrPlanConflict
	}
	return getProject(ctx, s.write, req.UserID, req.ProjectID)
}

// RecordPublishedURL saves or replaces a finalized project's posted URL and nothing else (CLIP-232).
func (s *Store) RecordPublishedURL(ctx context.Context, user, id, url string, now time.Time) (clip.Project, error) {
	if s.writer != nil {
		return clip.Project{}, errors.New("clip published url requires a coordinated transaction")
	}
	if err := affected(s.write.SetClipPublishedURL(ctx, sqlc.SetClipPublishedURLParams{ID: id, UserID: user, Url: nullable(url), Now: nullable(stamp(now))})); err != nil {
		return clip.Project{}, err
	}
	return getProject(ctx, s.write, user, id)
}

func saveChecklistTicks(ctx context.Context, q *sqlc.Queries, user, id string, ticks *[]string, now time.Time) error {
	if ticks == nil {
		return nil
	}
	raw, err := json.Marshal(append([]string{}, *ticks...))
	if err != nil {
		return err
	}
	return affected(q.SetClipChecklistTicks(ctx, sqlc.SetClipChecklistTicksParams{Ticks: string(raw), Now: stamp(now), ID: id, UserID: user}))
}
