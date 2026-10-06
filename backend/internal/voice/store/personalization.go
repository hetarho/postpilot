package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

const (
	slotCurrent  = "current"
	slotPrevious = "previous"
)

// analysisSnapshotVersion is the snapshot's own format; a row of another version is a record
// this build cannot read.
const analysisSnapshotVersion = 1

type analysisSnapshot struct {
	Origin          voice.Origin      `json:"origin,omitempty"`
	SyntheticSample string            `json:"synthetic_sample,omitempty"`
	Version         int               `json:"version"`
	Counted         voice.Fingerprint `json:"counted"`
	AI              voice.AIPart      `json:"ai"`
	MaterialCount   int               `json:"material_count"`
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func (s *Store) CurrentAnalysis(ctx context.Context, userID, voiceID string) (*voice.Analysis, error) {
	row, err := s.read.GetAnalysis(ctx, sqlc.GetAnalysisParams{VoiceID: voiceID, UserID: userID, Slot: slotCurrent})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select current analysis: %w", err)
	}
	var snapshot analysisSnapshot
	if err := json.Unmarshal([]byte(row.Snapshot), &snapshot); err != nil {
		return nil, fmt.Errorf("decode analysis snapshot: %w", err)
	}
	if snapshot.Version != analysisSnapshotVersion {
		return nil, fmt.Errorf("analysis snapshot version %d", snapshot.Version)
	}
	var ids []string
	if err := json.Unmarshal([]byte(row.MaterialIds), &ids); err != nil {
		return nil, fmt.Errorf("decode analysis material ids: %w", err)
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("analysis created_at: %w", err)
	}
	return &voice.Analysis{Origin: voice.NormalizedOrigin(snapshot.Origin), SyntheticSample: snapshot.SyntheticSample, Counted: snapshot.Counted, AI: snapshot.AI, MaterialIDs: ids, AnalyzeModel: row.AnalyzeModel, CreatedAt: created}, nil
}

func (s *Store) HasPreviousAnalysis(ctx context.Context, userID, voiceID string) (bool, error) {
	n, err := s.read.CountAnalysisSlot(ctx, sqlc.CountAnalysisSlotParams{VoiceID: voiceID, UserID: userID, Slot: slotPrevious})
	if err != nil {
		return false, fmt.Errorf("count previous analysis: %w", err)
	}
	return n > 0, nil
}

// PublishAnalysis discards the previous analysis, moves the current one there and stores the
// new current, in one transaction (VOICE-25, VOICE-30).
func (s *Store) PublishAnalysis(ctx context.Context, userID, voiceID string, analysis voice.Analysis) error {
	snapshot, err := json.Marshal(analysisSnapshot{Origin: voice.NormalizedOrigin(analysis.Origin), SyntheticSample: analysis.SyntheticSample, Version: analysisSnapshotVersion, Counted: analysis.Counted, AI: analysis.AI, MaterialCount: len(analysis.MaterialIDs)})
	if err != nil {
		return fmt.Errorf("encode analysis snapshot: %w", err)
	}
	ids := analysis.MaterialIDs
	if ids == nil {
		ids = []string{}
	}
	encodedIDs, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("encode analysis material ids: %w", err)
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin publish analysis: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.DeleteAnalysisSlot(ctx, sqlc.DeleteAnalysisSlotParams{VoiceID: voiceID, UserID: userID, Slot: slotPrevious}); err != nil {
		return fmt.Errorf("drop previous analysis: %w", err)
	}
	if _, err := q.MoveAnalysisSlot(ctx, sqlc.MoveAnalysisSlotParams{ToSlot: slotPrevious, VoiceID: voiceID, UserID: userID, FromSlot: slotCurrent}); err != nil {
		return fmt.Errorf("keep the current analysis as previous: %w", err)
	}
	if err := q.InsertCurrentAnalysis(ctx, sqlc.InsertCurrentAnalysisParams{
		VoiceID: voiceID, UserID: userID, Snapshot: string(snapshot), MaterialIds: string(encodedIDs),
		AnalyzeModel: analysis.AnalyzeModel, CreatedAt: formatTime(analysis.CreatedAt),
	}); err != nil {
		return fmt.Errorf("insert current analysis: %w", err)
	}
	return tx.Commit()
}

// RestorePreviousAnalysis makes the previous analysis current and discards the one it replaced.
func (s *Store) RestorePreviousAnalysis(ctx context.Context, userID, voiceID string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore analysis: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	n, err := q.CountAnalysisSlot(ctx, sqlc.CountAnalysisSlotParams{VoiceID: voiceID, UserID: userID, Slot: slotPrevious})
	if err != nil {
		return fmt.Errorf("count previous analysis: %w", err)
	}
	if n == 0 {
		return voice.ErrNoPreviousAnalysis
	}
	if err := q.DeleteAnalysisSlot(ctx, sqlc.DeleteAnalysisSlotParams{VoiceID: voiceID, UserID: userID, Slot: slotCurrent}); err != nil {
		return fmt.Errorf("drop current analysis: %w", err)
	}
	if _, err := q.MoveAnalysisSlot(ctx, sqlc.MoveAnalysisSlotParams{ToSlot: slotCurrent, VoiceID: voiceID, UserID: userID, FromSlot: slotPrevious}); err != nil {
		return fmt.Errorf("restore previous analysis: %w", err)
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
