package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func attemptResult(r sqlc.ClipAttemptResult) (clip.AttemptResult, error) {
	at, err := time.Parse(time.RFC3339Nano, r.ResultCreatedAt)
	return clip.AttemptResult{JobID: r.JobID, UserID: r.UserID, ProjectID: r.ProjectID, ExpectedRevision: int(r.ExpectedRevision), Analysis: r.AnalysisJson, EditPlan: r.EditPlanJson, Result: clip.Result{Key: r.ResultKey, ContentType: r.ResultContentType, Bytes: r.ResultBytes, DurationMS: int(r.ResultDurationMs), CreatedAt: at}}, err
}

func (s *Store) StageAttemptResult(ctx context.Context, c clip.AttemptResult) error {
	if c.JobID == "" || c.UserID == "" || c.ProjectID == "" || c.ExpectedRevision < 0 || !strings.HasPrefix(c.Result.Key, clip.ResultPrefix) || c.Result.CreatedAt.IsZero() {
		return clip.ErrInvalid
	}
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		if _, err := getProject(ctx, q, c.UserID, c.ProjectID); err != nil {
			return struct{}{}, err
		}
		err := q.StageAttemptResult(ctx, sqlc.StageAttemptResultParams{JobID: c.JobID, UserID: c.UserID, ProjectID: c.ProjectID, ExpectedRevision: int64(c.ExpectedRevision), AnalysisJson: c.Analysis, EditPlanJson: c.EditPlan, ResultKey: c.Result.Key, ResultContentType: c.Result.ContentType, ResultBytes: c.Result.Bytes, ResultDurationMs: int64(c.Result.DurationMS), ResultCreatedAt: stamp(c.Result.CreatedAt)})
		if err != nil {
			return struct{}{}, err
		}
		r, err := q.GetAttemptResult(ctx, c.JobID)
		if err != nil {
			return struct{}{}, err
		}
		saved, err := attemptResult(r)
		if err != nil {
			return struct{}{}, err
		}
		if !clip.SameAttemptResult(saved, c) {
			return struct{}{}, clip.ErrPlanConflict
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Store) GetAttemptResult(ctx context.Context, id string) (clip.AttemptResult, error) {
	r, err := s.read.GetAttemptResult(ctx, id)
	if err != nil {
		return clip.AttemptResult{}, dbError(err)
	}
	return attemptResult(r)
}
func (s *Store) PendingAttemptResults(ctx context.Context) ([]clip.AttemptResult, error) {
	rows, err := s.read.PendingAttemptResults(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]clip.AttemptResult, 0, len(rows))
	for _, r := range rows {
		c, err := attemptResult(r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// ApplyAttemptResult must share the job's terminal transaction. These checks and
// the project update cannot be separated by a cancellation or owner edit.
func (s *Store) ApplyAttemptResult(ctx context.Context, c clip.AttemptResult) error {
	if s.writer != nil {
		return errors.New("clip completion requires a coordinated transaction")
	}
	p, err := getProject(ctx, s.write, c.UserID, c.ProjectID)
	if err != nil {
		return err
	}
	access, err := s.write.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: c.ProjectID, UserID: c.UserID})
	if err != nil {
		return dbError(err)
	}
	if p.Finalized != nil {
		return clip.ErrFinalized
	}
	if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid {
		return clip.ErrBusy
	}
	if p.EditPlanRevision != c.ExpectedRevision {
		return clip.ErrPlanConflict
	}
	if c.EditPlan != "" {
		return s.SaveGeneration(ctx, c.UserID, c.ProjectID, c.Analysis, c.EditPlan, c.Result)
	}
	return s.SaveRender(ctx, c.UserID, c.ProjectID, c.ExpectedRevision, c.Result)
}
func (s *Store) DeleteAttemptResult(ctx context.Context, id string) error {
	return s.write.DeleteAttemptResult(ctx, id)
}
func (s *Store) DiscardAttemptResult(ctx context.Context, id string) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		r, err := q.GetAttemptResult(ctx, id)
		if errors.Is(dbError(err), clip.ErrNotFound) {
			return struct{}{}, nil
		}
		if err != nil {
			return struct{}{}, err
		}
		// A retried completion must never schedule the retained result for deletion.
		p, err := getProject(ctx, q, r.UserID, r.ProjectID)
		if err != nil {
			return struct{}{}, err
		}
		if p.Result == nil || p.Result.Key != r.ResultKey {
			if err := q.EnqueueObjectDeletion(ctx, sqlc.EnqueueObjectDeletionParams{ObjectKey: r.ResultKey, CreatedAt: stamp(time.Now())}); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, q.DeleteAttemptResult(ctx, id)
	})
	return err
}
