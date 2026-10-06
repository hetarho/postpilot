package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (s *Store) PublishAuthoring(ctx context.Context, user string, in voice.AuthoringPublication, v voice.Voice, analysis voice.Analysis) (voice.Voice, error) {
	if user == "" || user != v.UserID || in.Key.Key == "" || in.Key.SessionID == "" || voice.NormalizedOrigin(analysis.Origin) != voice.OriginSynthetic || len(analysis.MaterialIDs) != 0 {
		return voice.Voice{}, voice.ErrAuthoringConflict
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.Voice{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockAuthoringPublication(ctx); err != nil {
		return voice.Voice{}, err
	}
	prior, err := q.GetAuthoringPublication(ctx, sqlc.GetAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision)})
	if err == nil {
		if prior.PublicationKey != in.Key.Key {
			return voice.Voice{}, voice.ErrAuthoringConflict
		}
		row, err := q.GetVoice(ctx, sqlc.GetVoiceParams{ID: prior.TargetID, UserID: user})
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
	if in.SourceID != "" {
		row, err := q.GetVoice(ctx, sqlc.GetVoiceParams{ID: in.SourceID, UserID: user})
		if errors.Is(err, sql.ErrNoRows) {
			return voice.Voice{}, voice.ErrVoiceNotFound
		}
		if err != nil {
			return voice.Voice{}, err
		}
		if row.DeletedAt.Valid {
			return voice.Voice{}, voice.ErrVoiceDeleted
		}
		if row.Made != 1 {
			return voice.Voice{}, voice.ErrVoiceNotMade
		}
	}
	stamp := formatTime(v.CreatedAt)
	inserted := false
	for attempt := 1; attempt <= candidateNameAttempts; attempt++ {
		name := v.Name
		if attempt > 1 {
			suffix := " (" + strconv.Itoa(attempt) + ")"
			name = strings.TrimSpace(truncateRunes(v.Name, voice.VoiceNameMaxChars-utf8.RuneCountInString(suffix))) + suffix
		}
		err := q.InsertVoice(ctx, sqlc.InsertVoiceParams{ID: v.ID, UserID: user, Name: name, CreatedAt: stamp, UpdatedAt: stamp})
		if err == nil {
			inserted = true
			break
		}
		if !isUniqueViolation(err) {
			return voice.Voice{}, err
		}
	}
	if !inserted {
		return voice.Voice{}, voice.ErrVoiceNameTaken
	}
	snapshot, err := json.Marshal(analysisSnapshot{Version: analysisSnapshotVersion, Origin: voice.OriginSynthetic, SyntheticSample: analysis.SyntheticSample, Counted: analysis.Counted, AI: analysis.AI, MaterialCount: 0})
	if err != nil {
		return voice.Voice{}, err
	}
	if err := q.InsertCurrentAnalysis(ctx, sqlc.InsertCurrentAnalysisParams{VoiceID: v.ID, UserID: user, Snapshot: string(snapshot), MaterialIds: "[]", AnalyzeModel: analysis.AnalyzeModel, CreatedAt: stamp}); err != nil {
		return voice.Voice{}, err
	}
	if in.MakeDefault {
		if err := q.ClearDefaultVoice(ctx, sqlc.ClearDefaultVoiceParams{UserID: user, UpdatedAt: stamp}); err != nil {
			return voice.Voice{}, err
		}
		n, err := q.SetDefaultVoice(ctx, sqlc.SetDefaultVoiceParams{ID: v.ID, UserID: user, UpdatedAt: stamp})
		if err != nil {
			return voice.Voice{}, err
		}
		if n != 1 {
			return voice.Voice{}, voice.ErrVoiceNotFound
		}
	}
	if err := q.InsertAuthoringPublication(ctx, sqlc.InsertAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision), PublicationKey: in.Key.Key, TargetID: v.ID, CreatedAt: stamp}); err != nil {
		if isUniqueViolation(err) {
			return voice.Voice{}, voice.ErrAuthoringConflict
		}
		return voice.Voice{}, fmt.Errorf("record writing style publication: %w", err)
	}
	row, err := q.GetVoice(ctx, sqlc.GetVoiceParams{ID: v.ID, UserID: user})
	if err != nil {
		return voice.Voice{}, err
	}
	found, err := toVoice(sqlc.ListVoicesRow(row))
	if err != nil {
		return voice.Voice{}, err
	}
	return found, tx.Commit()
}

var _ voice.AuthoringStore = (*Store)(nil)
