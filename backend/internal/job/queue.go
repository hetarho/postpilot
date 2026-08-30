package job

import (
	"context"
	"errors"
	"fmt"
)

// Enqueue persists queued work and only then wakes the worker. It never runs the
// handler in the caller's request.
func (q *Queue) Enqueue(ctx context.Context, input NewJob) (string, error) {
	if input.Kind == "" || input.UserID == "" {
		return "", fmt.Errorf("enqueue job: kind and user are required")
	}

	active, err := q.activeForInput(ctx, input)
	if err != nil {
		return "", fmt.Errorf("check active job: %w", err)
	}
	if active != nil {
		return "", &ErrAlreadyInProgress{ActiveID: active.ID}
	}

	now := q.now()
	found := Job{
		ID: q.newID(), Kind: input.Kind, UserID: input.UserID, PostSlug: input.PostSlug, VoiceID: input.VoiceID,
		Status: StatusQueued, ObserveModel: input.ObserveModel, WriteModel: input.WriteModel,
		Payload: append([]byte(nil), input.Payload...), CreatedAt: now, UpdatedAt: now,
	}
	if err := q.store.Insert(ctx, found); err != nil {
		if errors.Is(err, ErrActiveConflict) {
			active, lookupErr := q.activeForInput(ctx, input)
			if lookupErr == nil && active != nil {
				return "", &ErrAlreadyInProgress{ActiveID: active.ID}
			}
		}
		return "", fmt.Errorf("insert job: %w", err)
	}

	select {
	case q.wake <- struct{}{}:
	default:
	}
	return found.ID, nil
}

// activeForInput checks every guard the row will be inserted under. Voice-owned work may
// also point at a post, so it must satisfy both the post guard and the (voice, kind) guard.
func (q *Queue) activeForInput(ctx context.Context, input NewJob) (*Job, error) {
	if input.PostSlug != nil {
		active, err := q.store.ActiveForPostUser(ctx, *input.PostSlug, input.UserID)
		if err != nil || active != nil {
			return active, err
		}
	}
	if input.VoiceID != "" && (input.PostSlug == nil || voiceOwnedKind(input.Kind)) {
		return q.store.ActiveForVoiceKind(ctx, input.VoiceID, input.Kind)
	}
	if input.PostSlug != nil {
		return nil, nil
	}
	return q.store.ActiveForUserKind(ctx, input.UserID, input.Kind)
}
