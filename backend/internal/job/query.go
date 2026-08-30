package job

import (
	"context"
	"errors"
	"fmt"
)

// Get returns an ownership-checked public view.
func (q *Queue) Get(ctx context.Context, id, userID string) (*JobSummary, error) {
	found, err := q.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	if found.UserID != userID {
		return nil, ErrForbidden
	}
	return summarize(found), nil
}

func (q *Queue) ActiveForPost(ctx context.Context, slug string) (*JobSummary, error) {
	found, err := q.store.ActiveForPost(ctx, slug)
	if err != nil || found == nil {
		return nil, err
	}
	return summarize(*found), nil
}

func (q *Queue) ActiveForUserKind(ctx context.Context, userID, kind string) (*JobSummary, error) {
	found, err := q.store.ActiveForUserKind(ctx, userID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return summarize(*found), nil
}

func (q *Queue) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*JobSummary, error) {
	found, err := q.store.ActiveForVoiceKind(ctx, voiceID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return summarize(*found), nil
}

// HasActiveForVoice reports any queued/running job frozen to the voice, whatever its kind:
// the voice context asks before a soft delete, so a result never lands in a tombstone.
func (q *Queue) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	found, err := q.store.ActiveForVoice(ctx, voiceID)
	return found != nil, err
}

// HasRunnableExperiment supports experiment boot recovery without exposing the job
// store or making the experiment context read generation_jobs directly.
func (q *Queue) HasRunnableExperiment(ctx context.Context, experimentID string) (bool, error) {
	found, err := q.store.ActiveModelExperiment(ctx, experimentID)
	return found != nil, err
}
