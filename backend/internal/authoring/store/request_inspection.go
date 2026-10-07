package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/authoring/store/sqlc"
	"github.com/postpilot/backend/internal/llm"
)

func (s *Store) ReadAuthoringInspectionSnapshot(ctx context.Context, user, session string, kind authoring.Kind, revision uint32, mode authoring.Mode) (authoring.RequestInspectionSnapshot, error) {
	var result authoring.RequestInspectionSnapshot
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	q := s.read.WithTx(tx)
	result.Session, err = load(ctx, q, user, session)
	if err != nil {
		return result, err
	}
	if result.Session.Kind != kind {
		return authoring.RequestInspectionSnapshot{}, authoring.ErrNotFound
	}
	if result.Session.Revision != revision {
		return authoring.RequestInspectionSnapshot{}, authoring.ErrStale
	}
	if result.Session.ActiveRequestID != "" {
		row, e := q.ActiveAuthoringOperation(ctx, sqlc.ActiveAuthoringOperationParams{UserID: user, SessionID: session})
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return result, e
		}
		if e == nil {
			op, e := fromOperation(row)
			if e != nil {
				return result, e
			}
			// Only the admitted base revision's material may be previewed.
			if op.ID == result.Session.ActiveRequestID && op.BaseRevision+1 == revision {
				result.Active = &op
			}
		}
	}
	raw, err := q.GetAuthoringRequestCapture(ctx, sqlc.GetAuthoringRequestCaptureParams{UserID: user, SessionID: session, Revision: sql.NullInt64{Int64: int64(revision), Valid: true}, Mode: string(mode)})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if err == nil && raw.Valid {
		var capture llm.RequestInspection
		decoder := json.NewDecoder(bytes.NewBufferString(raw.String))
		decoder.DisallowUnknownFields()
		e := decoder.Decode(&capture)
		var extra any
		if e == nil {
			e = decoder.Decode(&extra)
			if errors.Is(e, io.EOF) {
				e = nil
			} else {
				e = llm.ErrInvalidInspection
			}
		}
		if e == nil && capture.Validate() == nil && capture.Stage == "setting-authoring" && capture.Mode == string(kind)+"/"+string(mode) && (capture.Status == llm.InspectionCaptured || capture.Status == llm.InspectionUnavailable) {
			result.Captured = &capture
		}
	}
	return result, tx.Commit()
}

func (s *Store) WriteAuthoringRequestCapture(ctx context.Context, capture authoring.RequestCapture) error {
	if capture.JobID == "" || capture.Inspection.Validate() != nil || capture.Inspection.Stage != "setting-authoring" || (capture.Inspection.Status != llm.InspectionCaptured && capture.Inspection.Status != llm.InspectionUnavailable) {
		return llm.ErrInvalidInspection
	}
	if capture.Inspection.Status == llm.InspectionCaptured && capture.Inspection.CallID != capture.JobID {
		return llm.ErrInvalidInspection
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, err := load(ctx, q, capture.UserID, capture.SessionID)
	if err != nil {
		return err
	}
	if state.Kind != capture.Kind {
		return authoring.ErrNotFound
	}
	if state.ActiveRequestID != capture.OperationID || state.Revision != capture.BaseRevision+1 {
		return authoring.ErrStale
	}
	op, err := q.GetAuthoringOperationByID(ctx, sqlc.GetAuthoringOperationByIDParams{UserID: capture.UserID, ID: capture.OperationID})
	if err != nil {
		return missing(err)
	}
	if op.SessionID != capture.SessionID || capture.Inspection.Mode != string(capture.Kind)+"/"+op.Mode {
		return llm.ErrInvalidInspection
	}
	raw, err := json.Marshal(capture.Inspection)
	if err != nil {
		return err
	}
	n, err := q.WriteAuthoringRequestCapture(ctx, sqlc.WriteAuthoringRequestCaptureParams{UserID: capture.UserID, SessionID: capture.SessionID, OperationID: capture.OperationID, BaseRevision: int64(capture.BaseRevision), JobID: capture.JobID, Revision: sql.NullInt64{Int64: int64(state.Revision), Valid: true}, RequestCapture: sql.NullString{String: string(raw), Valid: true}})
	if err != nil {
		return err
	}
	if n != 1 {
		return authoring.ErrStale
	}
	return tx.Commit()
}

// Purging only erases request evidence. The private existing operation receipt
// retains its tombstone, so an issued or replayed callback cannot restore it.
func (s *Store) PurgeAuthoringRequestCaptures(ctx context.Context, user, session string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if _, err = load(ctx, q, user, session); err != nil {
		return err
	}
	if err = q.PurgeAuthoringRequestCaptures(ctx, sqlc.PurgeAuthoringRequestCapturesParams{UserID: user, SessionID: session}); err != nil {
		return err
	}
	return tx.Commit()
}

var _ authoring.RequestCaptureStore = (*Store)(nil)
