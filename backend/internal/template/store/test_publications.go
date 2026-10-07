package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/template/store/sqlc"
)

func (s *Store) TestPublicationReceipt(ctx context.Context, in template.TestedPublication) (template.TestedPublicationReceipt, bool, error) {
	return testPublicationReceipt(ctx, s.read, in)
}

func testPublicationReceipt(ctx context.Context, q *sqlc.Queries, in template.TestedPublication) (template.TestedPublicationReceipt, bool, error) {
	row, err := q.GetTestPublication(ctx, sqlc.GetTestPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey})
	if errors.Is(err, sql.ErrNoRows) {
		return template.TestedPublicationReceipt{}, false, nil
	}
	if err != nil {
		return template.TestedPublicationReceipt{}, false, err
	}
	if row.TestID != in.TestID || row.WinnerCandidateID != in.WinnerID || row.Action != in.Action || row.RequestKey != in.RequestKey || row.Fingerprint != in.Fingerprint {
		return template.TestedPublicationReceipt{}, false, template.ErrTestPublicationConflict
	}
	var receipt template.TestedPublicationReceipt
	if err := json.Unmarshal([]byte(row.Receipt), &receipt); err != nil {
		return template.TestedPublicationReceipt{}, false, fmt.Errorf("decode template test receipt: %w", err)
	}
	return receipt, true, nil
}

func (s *Store) CommitTestPublication(ctx context.Context, in template.TestedPublication, frozen template.TestSnapshot, newID string, at time.Time, max int) (template.TestedPublicationReceipt, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return template.TestedPublicationReceipt{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockAuthoringPublication(ctx); err != nil {
		return template.TestedPublicationReceipt{}, err
	}
	if receipt, found, err := testPublicationReceipt(ctx, q, in); err != nil || found {
		return receipt, err
	}
	id, stamp := frozen.TargetID, formatTime(at)
	if in.Action == "use_setting" {
		current, err := s.get(ctx, q, in.UserID, id)
		if err != nil {
			return template.TestedPublicationReceipt{}, err
		}
		if template.AuthoringVersion(current) != frozen.TargetVersion {
			return template.TestedPublicationReceipt{}, template.ErrTestPublicationConflict
		}
	} else {
		count, err := q.CountTemplates(ctx, in.UserID)
		if err != nil {
			return template.TestedPublicationReceipt{}, err
		}
		if int(count) >= max {
			return template.TestedPublicationReceipt{}, template.ErrTooMany
		}
		id = newID
		if err := q.InsertTemplate(ctx, sqlc.InsertTemplateParams{ID: id, UserID: in.UserID, Name: frozen.Draft.Name, Description: frozen.Draft.Description, Body: frozen.Draft.Body, TitleArea: frozen.Draft.TitleArea, TargetLength: nullNumber(frozen.Numbers.TargetLength), TagCount: nullNumber(frozen.Numbers.TagCount), CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
			if isUniqueViolation(err) {
				return template.TestedPublicationReceipt{}, template.ErrDuplicateName
			}
			return template.TestedPublicationReceipt{}, err
		}
	}
	receipt := template.TestedPublicationReceipt{TargetID: id, RequestKey: in.RequestKey}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return template.TestedPublicationReceipt{}, err
	}
	if err := q.InsertTestPublication(ctx, sqlc.InsertTestPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey, Fingerprint: in.Fingerprint, TargetID: id, Receipt: string(raw), CreatedAt: stamp}); err != nil {
		return template.TestedPublicationReceipt{}, err
	}
	return receipt, tx.Commit()
}

var _ template.TestPublicationStore = (*Store)(nil)

func (s *Store) ReadTestPublicationReceipt(ctx context.Context, userID, testID, winnerID string, action string) (template.TestedPublicationReceipt, bool, error) {
	var result template.TestedPublicationReceipt
	raw, err := s.read.ReadTestPublicationReceipt(ctx, sqlc.ReadTestPublicationReceiptParams{UserID: userID, TestID: testID, WinnerCandidateID: winnerID, Action: action})
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if err = json.Unmarshal([]byte(raw), &result); err != nil {
		return result, false, err
	}
	return result, true, nil
}
