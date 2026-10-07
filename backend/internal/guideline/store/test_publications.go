package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/guideline/store/sqlc"
)

func (s *Store) TestPublicationReceipt(ctx context.Context, in guideline.TestedPublication) (guideline.TestedPublicationReceipt, bool, error) {
	return testPublicationReceipt(ctx, s.read, in)
}

func testPublicationReceipt(ctx context.Context, q *sqlc.Queries, in guideline.TestedPublication) (guideline.TestedPublicationReceipt, bool, error) {
	row, err := q.GetTestPublication(ctx, sqlc.GetTestPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey})
	if errors.Is(err, sql.ErrNoRows) {
		return guideline.TestedPublicationReceipt{}, false, nil
	}
	if err != nil {
		return guideline.TestedPublicationReceipt{}, false, err
	}
	if row.TestID != in.TestID || row.WinnerCandidateID != in.WinnerID || row.Action != in.Action || row.RequestKey != in.RequestKey || row.Fingerprint != in.Fingerprint {
		return guideline.TestedPublicationReceipt{}, false, guideline.ErrTestPublicationConflict
	}
	var receipt guideline.TestedPublicationReceipt
	if err := json.Unmarshal([]byte(row.Receipt), &receipt); err != nil {
		return guideline.TestedPublicationReceipt{}, false, fmt.Errorf("decode guideline test receipt: %w", err)
	}
	return receipt, true, nil
}

func (s *Store) CommitTestPublication(ctx context.Context, in guideline.TestedPublication, frozen guideline.TestSnapshot, scope guideline.ScopePatch, newID string, at time.Time, max int) (guideline.TestedPublicationReceipt, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return guideline.TestedPublicationReceipt{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockAuthoringPublication(ctx); err != nil {
		return guideline.TestedPublicationReceipt{}, err
	}
	if receipt, found, err := testPublicationReceipt(ctx, q, in); err != nil || found {
		return receipt, err
	}
	id, stamp := frozen.TargetID, formatTime(at)
	if in.Action == "use_setting" {
		current, err := s.get(ctx, q, in.UserID, id)
		if err != nil {
			return guideline.TestedPublicationReceipt{}, err
		}
		if current.Kind != frozen.Kind || guideline.AuthoringVersion(current) != frozen.TargetVersion {
			return guideline.TestedPublicationReceipt{}, guideline.ErrTestPublicationConflict
		}
	} else {
		count, err := q.CountGuidelines(ctx, sqlc.CountGuidelinesParams{UserID: in.UserID, Kind: string(frozen.Kind)})
		if err != nil {
			return guideline.TestedPublicationReceipt{}, err
		}
		if int(count) >= max {
			return guideline.TestedPublicationReceipt{}, &guideline.AccountCapError{Max: max}
		}
		id = newID
		if err := q.InsertGuideline(ctx, sqlc.InsertGuidelineParams{ID: id, UserID: in.UserID, Kind: string(frozen.Kind), Title: frozen.Draft.Name, Text: frozen.Draft.Body, Scope: string(scope.Scope), CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
			if isDuplicateText(err) {
				return guideline.TestedPublicationReceipt{}, guideline.ErrDuplicateText
			}
			return guideline.TestedPublicationReceipt{}, err
		}
		if err := insertScope(ctx, q, in.UserID, frozen.Kind, id, scope.TemplateIDs); err != nil {
			return guideline.TestedPublicationReceipt{}, err
		}
		if err := insertFields(ctx, q, in.UserID, id, scope.Fields); err != nil {
			return guideline.TestedPublicationReceipt{}, err
		}
		if err := approve(ctx, q, in.UserID, frozen.Kind, guideline.CandidateApproval{Text: frozen.Draft.Body}); err != nil {
			return guideline.TestedPublicationReceipt{}, err
		}
	}
	receipt := guideline.TestedPublicationReceipt{TargetID: id, RequestKey: in.RequestKey}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return guideline.TestedPublicationReceipt{}, err
	}
	if err := q.InsertTestPublication(ctx, sqlc.InsertTestPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey, Fingerprint: in.Fingerprint, TargetID: id, Receipt: string(raw), CreatedAt: stamp}); err != nil {
		return guideline.TestedPublicationReceipt{}, err
	}
	return receipt, tx.Commit()
}

var _ guideline.TestPublicationStore = (*Store)(nil)
