package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

// ReadTestInspectionWork reads the retained aggregate and all checkpoints from
// one SQLite snapshot. A concurrent purge cannot mix old output with new fences.
func (s *Store) ReadTestInspectionWork(ctx context.Context, user, id string) (experiment.TestExecutionWork, error) {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return experiment.TestExecutionWork{}, err
	}
	defer tx.Rollback()
	work, err := workForTest(ctx, sqlc.New(tx), user, id)
	if err != nil {
		return experiment.TestExecutionWork{}, err
	}
	if err := tx.Commit(); err != nil {
		return experiment.TestExecutionWork{}, err
	}
	return work, nil
}

// TxPayloadReader is an experiment-owned publication read port bound to the
// target's writer transaction. It reads no other context's tables.
type TxPayloadReader struct{ read *sqlc.Queries }

func NewTx(tx *sql.Tx) *TxPayloadReader { return &TxPayloadReader{read: sqlc.New(tx)} }

func (s *TxPayloadReader) TestPayloadAvailable(ctx context.Context, user, test, winner string, fence uint64) (bool, error) {
	found, err := loadWritingTest(ctx, s.read, user, test)
	if err == experiment.ErrTestNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fence != 0 || found.PurgeFence != fence || found.Status != experiment.TestCompleted || found.WinnerID != winner || len(found.CommonSnapshot) == 0 || (found.ContentExpiresAt != nil && !found.ContentExpiresAt.After(time.Now())) {
		return false, nil
	}
	for _, candidate := range found.Candidates {
		if candidate.ID == winner && candidate.Status == string(experiment.TestCandidateSucceeded) && len(candidate.Output) > 0 {
			return true, nil
		}
	}
	return false, nil
}

var _ experiment.TestInspectionStorage = (*Store)(nil)
