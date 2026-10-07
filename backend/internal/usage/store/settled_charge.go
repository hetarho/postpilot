package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/postpilot/backend/internal/usage/store/sqlc"
)

func (s *Store) SettledChargeForJob(ctx context.Context, user, job string) (int, bool, error) {
	charge, err := s.read.SettledChargeForJob(ctx, sqlc.SettledChargeForJobParams{UserID: user, JobID: job})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return int(charge), true, nil
}
