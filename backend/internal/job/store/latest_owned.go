package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
)

func (s *Store) LatestOwnedKind(ctx context.Context, userID, kind, status string) (*job.Job, error) {
	row, err := s.read.LatestOwnedKind(ctx, sqlc.LatestOwnedKindParams{UserID: userID, Kind: kind, StatusFilter: status})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	found, err := toJob(row)
	if err != nil {
		return nil, err
	}
	return &found, nil
}
