package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

func testPublicationFingerprint(in experiment.WinnerPublication) (string, error) {
	// The operation key and aggregate revision are retry handles. The publication
	// choices themselves stay fixed when the owner recovers a lost response.
	scopes := slices.Clone(in.ScopeIDs)
	slices.Sort(scopes)
	value := struct {
		Test, Winner, Action, Name, Scope string
		ScopeIDs                          []string
		Default                           bool
		InputRevision, ContentRevision    int64
	}{in.TestID, in.WinnerID, in.Action, strings.TrimSpace(in.Name), in.Scope, scopes, in.MakeDefault, in.InputRevision, in.ContentRevision}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func publicationFromRow(p sqlc.WritingTestPublication) experiment.TestPublication {
	return experiment.TestPublication{ID: p.ID, UserID: p.UserID, TestID: p.TestID, WinnerID: p.WinnerCandidateID, Action: p.Action, RequestKey: p.RequestKey, TargetID: p.TargetID, Status: p.Status, Fingerprint: p.Fingerprint}
}
func (s *Store) BeginPublication(ctx context.Context, in experiment.WinnerPublication) (experiment.TestPublication, error) {
	var out experiment.TestPublication
	if in.UserID == "" || in.TestID == "" || in.WinnerID == "" || strings.TrimSpace(in.RequestKey) == "" || !experiment.TestPublicationAction(in.Action).Valid() {
		return out, experiment.ErrTestOperation
	}
	fingerprint, err := testPublicationFingerprint(in)
	if err != nil {
		return out, err
	}
	err = s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockWritingTestPublication(ctx); err != nil {
			return err
		}
		rows, err := q.FindWritingTestPublication(ctx, sqlc.FindWritingTestPublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey})
		if err != nil {
			return err
		}
		if len(rows) > 1 {
			return experiment.ErrTestPublicationConflict
		}
		if len(rows) == 1 {
			prior := rows[0]
			if prior.TestID != in.TestID || prior.WinnerCandidateID != in.WinnerID || prior.Action != in.Action || prior.Fingerprint != fingerprint {
				return experiment.ErrTestPublicationConflict
			}
			out = publicationFromRow(prior)
			return nil
		}
		found, err := loadWritingTest(ctx, q, in.UserID, in.TestID)
		if err != nil {
			return err
		}
		if found.Revision != in.ExpectedRevision {
			return experiment.ErrTestRevisionConflict
		}
		if found.Status != experiment.TestCompleted || found.WinnerID != in.WinnerID || found.PurgeFence != 0 {
			return experiment.ErrTestStateInvalid
		}
		action := experiment.TestPublicationAction(in.Action)
		if (found.Factor == experiment.FactorModel) != (action == experiment.TestAdoptModel || action == experiment.TestApplyOutput) {
			return experiment.ErrTestOperation
		}
		if action == experiment.TestApplyOutput && found.SourcePostSlug == "" {
			return experiment.ErrTestOutputIncompatible
		}
		now := time.Now().UTC()
		id := uuid.NewString()
		if err := q.InsertWritingTestPublication(ctx, sqlc.InsertWritingTestPublicationParams{ID: id, UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey, Fingerprint: fingerprint, CreatedAt: formatTime(now)}); err != nil {
			return err
		}
		out = experiment.TestPublication{ID: id, UserID: in.UserID, TestID: in.TestID, WinnerID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey, Fingerprint: fingerprint, Status: string(experiment.TestPublicationPending)}
		expected := found.Revision
		found.Revision++
		found.UpdatedAt = now
		return updateWritingTest(ctx, q, &found, expected)
	})
	return out, err
}
func (s *Store) ConfirmPublication(ctx context.Context, p experiment.TestPublication, r experiment.PublicationReceipt) (experiment.TestPublication, error) {
	if p.UserID == "" || r.UserID != p.UserID || r.TestID != p.TestID || r.WinnerID != p.WinnerID || r.Action != p.Action || r.RequestKey != p.RequestKey || r.TargetID == "" {
		return experiment.TestPublication{}, experiment.ErrTestPublicationConflict
	}
	var out experiment.TestPublication
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockWritingTestPublication(ctx); err != nil {
			return err
		}
		rows, err := q.FindWritingTestPublication(ctx, sqlc.FindWritingTestPublicationParams{UserID: p.UserID, TestID: p.TestID, WinnerCandidateID: p.WinnerID, Action: p.Action, RequestKey: p.RequestKey})
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return experiment.ErrTestPublicationConflict
		}
		prior := publicationFromRow(rows[0])
		if prior.ID != p.ID || prior.Fingerprint != p.Fingerprint || prior.RequestKey != p.RequestKey {
			return experiment.ErrTestPublicationConflict
		}
		if prior.Status == string(experiment.TestPublicationConfirmed) {
			if prior.TargetID != r.TargetID {
				return experiment.ErrTestPublicationConflict
			}
			out = prior
			return nil
		}
		if prior.Status != string(experiment.TestPublicationPending) {
			return experiment.ErrTestPublicationConflict
		}
		now := time.Now().UTC()
		count, err := q.ConfirmWritingTestPublication(ctx, sqlc.ConfirmWritingTestPublicationParams{TargetID: r.TargetID, ConfirmedAt: nullTime(&now), UserID: p.UserID, TestID: p.TestID, ID: p.ID})
		if err != nil {
			return err
		}
		if count != 1 {
			return experiment.ErrTestPublicationConflict
		}
		prior.TargetID = r.TargetID
		prior.Status = string(experiment.TestPublicationConfirmed)
		out = prior
		found, err := loadWritingTest(ctx, q, p.UserID, p.TestID)
		if err != nil {
			return err
		}
		expected := found.Revision
		found.Revision++
		found.UpdatedAt = now
		return updateWritingTest(ctx, q, &found, expected)
	})
	return out, err
}
