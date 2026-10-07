package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/provider/store/sqlc"
)

func (s *Store) TestModelReceipt(ctx context.Context, in provider.TestModelAdoption) (provider.TestModelReceipt, bool, error) {
	return testModelReceipt(ctx, s.read, in)
}

func testModelReceipt(ctx context.Context, q *sqlc.Queries, in provider.TestModelAdoption) (provider.TestModelReceipt, bool, error) {
	row, err := q.GetTestModelPublication(ctx, sqlc.GetTestModelPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, RequestKey: in.RequestKey})
	if errors.Is(err, sql.ErrNoRows) {
		return provider.TestModelReceipt{}, false, nil
	}
	if err != nil {
		return provider.TestModelReceipt{}, false, err
	}
	if row.TestID != in.TestID || row.WinnerCandidateID != in.WinnerID || row.RequestKey != in.RequestKey || row.Fingerprint != in.Fingerprint || row.Stage != string(in.Stage) || row.ModelRef != in.Ref.String() {
		return provider.TestModelReceipt{}, false, provider.ErrTestPublicationConflict
	}
	var receipt provider.TestModelReceipt
	if err := json.Unmarshal([]byte(row.Receipt), &receipt); err != nil {
		return provider.TestModelReceipt{}, false, fmt.Errorf("decode model adoption receipt: %w", err)
	}
	return receipt, true, nil
}

func (s *Store) CommitTestModelAdoption(ctx context.Context, in provider.TestModelAdoption, at time.Time) (provider.TestModelReceipt, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return provider.TestModelReceipt{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockTestModelPublication(ctx); err != nil {
		return provider.TestModelReceipt{}, err
	}
	if receipt, found, err := testModelReceipt(ctx, q, in); err != nil || found {
		return receipt, err
	}
	stamp := at.UTC().Format(writeLayout)
	if err := q.UpsertSelection(ctx, sqlc.UpsertSelectionParams{UserID: in.UserID, Stage: string(in.Stage), ProviderID: in.Ref.ProviderID, ModelID: in.Ref.ModelID, UpdatedAt: stamp}); err != nil {
		return provider.TestModelReceipt{}, err
	}
	receipt := provider.TestModelReceipt{RequestKey: in.RequestKey, Stage: in.Stage, Ref: in.Ref}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return provider.TestModelReceipt{}, err
	}
	if err := q.InsertTestModelPublication(ctx, sqlc.InsertTestModelPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, RequestKey: in.RequestKey, Fingerprint: in.Fingerprint, Stage: string(in.Stage), ModelRef: in.Ref.String(), Receipt: string(raw), CreatedAt: stamp}); err != nil {
		return provider.TestModelReceipt{}, fmt.Errorf("record model adoption: %w", err)
	}
	return receipt, tx.Commit()
}

var _ provider.TestModelAdoptionStore = (*Store)(nil)
