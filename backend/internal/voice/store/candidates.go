package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

const candidateNameAttempts = 100

// AdoptCandidate commits the directory, made synthetic snapshot and idempotency record
// together. Existing mappings return the current row, preserving later rename/delete/default.
func (s *Store) AdoptCandidate(ctx context.Context, adoption voice.CandidateAdoption) (voice.Voice, error) {
	if voice.NormalizedOrigin(adoption.Analysis.Origin) != voice.OriginSynthetic || len(adoption.Analysis.MaterialIDs) != 0 {
		return voice.Voice{}, errors.New("candidate adoption must be synthetic with no personal materials")
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.Voice{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockCandidateAdoption(ctx); err != nil {
		return voice.Voice{}, err
	}
	prior, err := q.GetCandidateAdoption(ctx, sqlc.GetCandidateAdoptionParams{UserID: adoption.Voice.UserID, JobID: adoption.JobID, CandidateID: adoption.CandidateID})
	if err == nil {
		row, err := q.GetVoice(ctx, sqlc.GetVoiceParams{ID: prior, UserID: adoption.Voice.UserID})
		if err != nil {
			return voice.Voice{}, err
		}
		found, err := toVoice(sqlc.ListVoicesRow(row))
		if err != nil {
			return voice.Voice{}, err
		}
		return found, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return voice.Voice{}, err
	}
	v := adoption.Voice
	stamp := formatTime(v.CreatedAt)
	inserted := false
	for attempt := 1; attempt <= candidateNameAttempts; attempt++ {
		name := v.Name
		if attempt > 1 {
			suffix := " (" + strconv.Itoa(attempt) + ")"
			name = strings.TrimSpace(truncateRunes(v.Name, voice.VoiceNameMaxChars-utf8.RuneCountInString(suffix))) + suffix
		}
		err := q.InsertVoice(ctx, sqlc.InsertVoiceParams{ID: v.ID, UserID: v.UserID, Name: name, IsDefault: 0, CreatedAt: stamp, UpdatedAt: stamp})
		if err == nil {
			v.Name = name
			inserted = true
			break
		}
		if !isUniqueViolation(err) {
			return voice.Voice{}, fmt.Errorf("insert candidate voice: %w", err)
		}
	}
	if !inserted {
		return voice.Voice{}, voice.ErrVoiceNameTaken
	}
	snapshot, err := json.Marshal(analysisSnapshot{Version: analysisSnapshotVersion, Origin: voice.OriginSynthetic, SyntheticSample: adoption.Analysis.SyntheticSample, Counted: adoption.Analysis.Counted, AI: adoption.Analysis.AI, MaterialCount: 0})
	if err != nil {
		return voice.Voice{}, err
	}
	if err := q.InsertCurrentAnalysis(ctx, sqlc.InsertCurrentAnalysisParams{VoiceID: v.ID, UserID: v.UserID, Snapshot: string(snapshot), MaterialIds: "[]", AnalyzeModel: adoption.Analysis.AnalyzeModel, CreatedAt: stamp, SourceVersionsKnown: boolInt(adoption.Analysis.SourceVersionsKnown), AcceptedSources: "[]", AcceptedMaterialSnapshot: "[]"}); err != nil {
		return voice.Voice{}, fmt.Errorf("insert candidate snapshot: %w", err)
	}
	if adoption.MakeDefault {
		if err := q.ClearDefaultVoice(ctx, sqlc.ClearDefaultVoiceParams{UserID: v.UserID, UpdatedAt: stamp}); err != nil {
			return voice.Voice{}, err
		}
		n, err := q.SetDefaultVoice(ctx, sqlc.SetDefaultVoiceParams{ID: v.ID, UserID: v.UserID, UpdatedAt: stamp})
		if err != nil {
			return voice.Voice{}, err
		}
		if n != 1 {
			return voice.Voice{}, voice.ErrVoiceNotFound
		}
	}
	if err := q.InsertCandidateAdoption(ctx, sqlc.InsertCandidateAdoptionParams{UserID: v.UserID, JobID: adoption.JobID, CandidateID: adoption.CandidateID, VoiceID: v.ID, CreatedAt: stamp}); err != nil {
		return voice.Voice{}, fmt.Errorf("insert candidate adoption: %w", err)
	}
	row, err := q.GetVoice(ctx, sqlc.GetVoiceParams{ID: v.ID, UserID: v.UserID})
	if err != nil {
		return voice.Voice{}, err
	}
	found, err := toVoice(sqlc.ListVoicesRow(row))
	if err != nil {
		return voice.Voice{}, err
	}
	return found, tx.Commit()
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return text
}

var _ voice.CandidateStore = (*Store)(nil)
