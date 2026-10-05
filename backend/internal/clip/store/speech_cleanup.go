package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) PrepareSpeechCleanup(ctx context.Context, owner, project string, c clip.SpeechCleanup) error {
	if c.ID == "" || c.ObjectKey != clip.SpeechAudioPrefix+c.ID+".mp3" || c.CreatedAt.IsZero() {
		return clip.ErrInvalid
	}
	return affected(s.write.PrepareClipSpeechCleanup(ctx, sqlc.PrepareClipSpeechCleanupParams{ID: c.ID, ObjectKey: c.ObjectKey, CreatedAt: stamp(c.CreatedAt), Owner: owner, Project: project}))
}
func (s *Store) PendingSpeechCleanup(ctx context.Context, before time.Time) ([]clip.SpeechCleanup, error) {
	rows, err := s.read.PendingClipSpeechCleanup(ctx, stamp(before))
	if err != nil {
		return nil, err
	}
	result := make([]clip.SpeechCleanup, 0, len(rows))
	for _, row := range rows {
		created, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, clip.SpeechCleanup{ID: row.ID, ObjectKey: row.ObjectKey, CreatedAt: created})
	}
	return result, nil
}
func (s *Store) SpeechAssetRetained(ctx context.Context, id string) (bool, error) {
	n, err := s.read.ClipSpeechAssetRetained(ctx, id)
	return n > 0, err
}
func (s *Store) DiscardRetainedSpeechCleanup(ctx context.Context, id string) error {
	return s.write.DiscardRetainedClipSpeechCleanup(ctx, id)
}
func (s *Store) CompleteSpeechCleanup(ctx context.Context, id string) error {
	return s.write.CompleteClipSpeechCleanup(ctx, id)
}

func collectFinalizedSpeech(ctx context.Context, q *sqlc.Queries, p clip.Project) error {
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	if plan.Narration != nil {
		for _, seg := range plan.Narration.Segments {
			if seg.Speech != nil {
				keep[seg.Speech.AssetID] = true
			}
		}
	}
	if p.Result != nil {
		for _, speech := range p.Result.Speech {
			keep[speech.Speech.AssetID] = true
		}
	}
	rows, err := q.ListClipSpeechAssets(ctx, sqlc.ListClipSpeechAssetsParams{OwnerID: p.UserID, ProjectID: p.ID})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if keep[row.ID] {
			continue
		}
		if err := affected(q.DeleteUnusedClipSpeechAsset(ctx, sqlc.DeleteUnusedClipSpeechAssetParams{ID: row.ID, OwnerID: p.UserID, ProjectID: p.ID})); err != nil {
			return err
		}
	}
	return nil
}
