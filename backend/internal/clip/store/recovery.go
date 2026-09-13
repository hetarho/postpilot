package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) GetRecovery(ctx context.Context, user, project string) (*clip.RecoveryState, error) {
	row, err := s.read.GetClipRecovery(ctx, sqlc.GetClipRecoveryParams{ProjectID: project, UserID: user})
	if errors.Is(dbError(err), clip.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state clip.RecoveryState
	if len(row.StateJson) > clip.AttemptCheckpointMaxBytes || json.Unmarshal([]byte(row.StateJson), &state) != nil || state.Version != 1 || state.JobID != row.JobID {
		return nil, clip.ErrInvalid
	}
	return &state, nil
}
func (s *Store) SaveRecovery(ctx context.Context, user, project string, state clip.RecoveryState) error {
	if state.Version != 1 || state.JobID == "" || state.Legacy != nil {
		return clip.ErrInvalid
	}
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > clip.AttemptCheckpointMaxBytes {
		return clip.ErrAttemptCheckpointUnavailable
	}
	n, err := s.write.SaveClipRecovery(ctx, sqlc.SaveClipRecoveryParams{ProjectID: project, UserID: user, JobID: state.JobID, StateJson: string(raw)})
	return affected(n, err)
}
