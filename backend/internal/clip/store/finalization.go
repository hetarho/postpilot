package store

import (
	"context"
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
	// Freeze any legacy hydration before template deletion may clear its reference.
	if p.Composition != nil {
		if err := saveComposition(ctx, s.write, p); err != nil {
			return clip.Project{}, err
		}
	}
	n, err := s.write.FinalizeClipProject(ctx, sqlc.FinalizeClipProjectParams{ID: req.ProjectID, UserID: req.UserID, Now: nullable(stamp(now)), ExpectedRevision: int64(req.ExpectedRevision), ExpectedResultID: nullable(req.ExpectedResultID)})
	if err != nil {
		return clip.Project{}, dbError(err)
	}
	if n != 1 {
		return clip.Project{}, clip.ErrFinalizationConflict
	}
	if _, err := s.RevokeProjectSources(ctx, req.UserID, req.ProjectID, now); err != nil {
		return clip.Project{}, err
	}
	return getProject(ctx, s.write, req.UserID, req.ProjectID)
}
