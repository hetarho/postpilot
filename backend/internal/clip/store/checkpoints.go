package store

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) SaveAttemptCheckpoint(ctx context.Context, user, project string, c clip.AttemptCheckpoint) error {
	if c.Version != 1 || c.JobID == "" {
		return clip.ErrInvalid
	}
	c.Diagnostic.Check = clip.SafeAttemptCheck(c.Diagnostic.Check)
	c.Diagnostic.Phase = clip.SafeAttemptPhase(c.Diagnostic.Phase)
	c.Diagnostic.Values = clip.SafeAttemptValues(c.Diagnostic.Values)
	raw, err := json.Marshal(c)
	if err != nil {
		return clip.ErrInvalid
	}
	if len(raw) > clip.AttemptCheckpointMaxBytes {
		// Keep the latest counts and failure location even when evidence reaches its
		// storage budget. Omission is explicit; never overwrite the worker's slices.
		c.EvidenceLimited = true
		c.Observations = slices.Clone(c.Observations)
		remaining := len(raw) - clip.AttemptCheckpointMaxBytes + 64
		for i := len(c.Observations) - 1; i >= 0 && remaining > 0; i-- {
			a := &c.Observations[i]
			for len(a.Segments) > 0 && remaining > 0 {
				last, _ := json.Marshal(a.Segments[len(a.Segments)-1])
				remaining -= len(last) + 1
				a.Segments = a.Segments[:len(a.Segments)-1]
			}
		}
		raw, err = json.Marshal(c)
	}
	if err != nil || len(raw) > clip.AttemptCheckpointMaxBytes {
		return clip.ErrInvalid
	}
	n, err := s.write.SaveAttemptCheckpoint(ctx, sqlc.SaveAttemptCheckpointParams{ProjectID: project, UserID: user, JobID: c.JobID, CheckpointJson: string(raw)})
	return affected(n, err)
}

func (s *Store) GetAttemptCheckpoint(ctx context.Context, user, project, job string) (*clip.AttemptCheckpoint, error) {
	raw, err := s.read.GetAttemptCheckpoint(ctx, sqlc.GetAttemptCheckpointParams{ProjectID: project, UserID: user, JobID: job})
	if errors.Is(dbError(err), clip.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c clip.AttemptCheckpoint
	if len(raw) > clip.AttemptCheckpointMaxBytes || json.Unmarshal([]byte(raw), &c) != nil || c.Version != 1 || c.JobID != job {
		return nil, clip.ErrInvalid
	}
	return &c, nil
}
