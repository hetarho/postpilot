package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

// --- 검증 (VOICE-43) ---

func (s *Store) InsertCheck(ctx context.Context, check voice.Check) error {
	if err := s.write.InsertVoiceCheck(ctx, sqlc.InsertVoiceCheckParams{
		ID: check.ID, UserID: check.UserID, VoiceID: check.VoiceID, PromptKey: check.PromptKey, MaterialID: check.MaterialID,
		AnalysisCreatedAt: formatTime(check.AnalysisCreatedAt), Projection: check.Projection, WriteModel: check.WriteModel,
		CreatedAt: formatTime(check.CreatedAt), UpdatedAt: formatTime(check.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("insert voice check: %w", err)
	}
	return nil
}

func (s *Store) DeleteCheck(ctx context.Context, userID, checkID string) error {
	if err := s.write.DeleteVoiceCheck(ctx, sqlc.DeleteVoiceCheckParams{ID: checkID, UserID: userID}); err != nil {
		return fmt.Errorf("delete voice check: %w", err)
	}
	return nil
}

func (s *Store) GetCheck(ctx context.Context, userID, checkID string) (voice.Check, error) {
	row, err := s.read.GetVoiceCheck(ctx, sqlc.GetVoiceCheckParams{ID: checkID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Check{}, voice.ErrCheckNotFound
	}
	if err != nil {
		return voice.Check{}, fmt.Errorf("select voice check: %w", err)
	}
	return toCheck(userID, row)
}

func (s *Store) ListChecks(ctx context.Context, userID, voiceID string) ([]voice.Check, error) {
	rows, err := s.read.ListVoiceChecks(ctx, sqlc.ListVoiceChecksParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("select voice checks: %w", err)
	}
	out := make([]voice.Check, 0, len(rows))
	for _, row := range rows {
		check, err := toCheck(userID, sqlc.GetVoiceCheckRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, check)
	}
	return out, nil
}

func (s *Store) MarkCheckRunning(ctx context.Context, userID, checkID string, now time.Time) (bool, error) {
	n, err := s.write.MarkVoiceCheckRunning(ctx, sqlc.MarkVoiceCheckRunningParams{UpdatedAt: formatTime(now), ID: checkID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("mark voice check running: %w", err)
	}
	return n == 1, nil
}

func (s *Store) FinishCheck(ctx context.Context, userID, checkID, piece string, now time.Time) (bool, error) {
	n, err := s.write.FinishVoiceCheck(ctx, sqlc.FinishVoiceCheckParams{
		Piece: sql.NullString{String: piece, Valid: true}, UpdatedAt: formatTime(now), ID: checkID, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("finish voice check: %w", err)
	}
	return n == 1, nil
}

func (s *Store) FailCheck(ctx context.Context, userID, checkID string, failure voice.Failure, now time.Time) (bool, error) {
	reason, params, detail, err := encodeFailure(&failure)
	if err != nil {
		return false, err
	}
	if !reason.Valid {
		reason, params = sql.NullString{String: voice.FailureReasonUnknown, Valid: true}, sql.NullString{String: "{}", Valid: true}
	}
	n, err := s.write.FailVoiceCheck(ctx, sqlc.FailVoiceCheckParams{
		ErrorReason: reason, ErrorParams: params, TechnicalDetail: detail, UpdatedAt: formatTime(now), ID: checkID, UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("fail voice check: %w", err)
	}
	return n == 1, nil
}

func toCheck(userID string, row sqlc.GetVoiceCheckRow) (voice.Check, error) {
	times := make([]time.Time, 3)
	for i, value := range []string{row.AnalysisCreatedAt, row.CreatedAt, row.UpdatedAt} {
		parsed, err := parseTime(value)
		if err != nil {
			return voice.Check{}, fmt.Errorf("voice check %s time: %w", row.ID, err)
		}
		times[i] = parsed
	}
	failure, err := decodeFailure(row.ErrorReason, row.ErrorParams, row.TechnicalDetail, sql.NullString{})
	if err != nil {
		return voice.Check{}, fmt.Errorf("voice check %s failure: %w", row.ID, err)
	}
	return voice.Check{
		ID: row.ID, UserID: userID, VoiceID: row.VoiceID, PromptKey: row.PromptKey, MaterialID: row.MaterialID,
		AnalysisCreatedAt: times[0], Projection: row.Projection, WriteModel: row.WriteModel,
		Status: voice.CheckStatus(row.Status), Piece: row.Piece.String, Failure: failure,
		CreatedAt: times[1], UpdatedAt: times[2],
	}, nil
}
