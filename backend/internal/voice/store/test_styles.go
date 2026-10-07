package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

// PublishTestStyle commits only voice-owned state. The test context validates its champion
// before calling; current personal/profile/source conditions are checked again in this transaction.
func (s *Store) PublishTestStyle(ctx context.Context, in voice.TestStylePublication, created voice.Voice) (voice.TestStyleReceipt, error) {
	if err := voice.ValidTestStylePublication(in); err != nil {
		return voice.TestStyleReceipt{}, err
	}
	fingerprint, err := voice.TestStylePublicationFingerprint(in)
	if err != nil {
		return voice.TestStyleReceipt{}, err
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.TestStyleReceipt{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockTestStylePublication(ctx); err != nil {
		return voice.TestStyleReceipt{}, err
	}
	replays, err := q.FindTestStylePublication(ctx, sqlc.FindTestStylePublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey})
	if err != nil {
		return voice.TestStyleReceipt{}, err
	}
	if len(replays) > 1 {
		return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
	}
	if len(replays) == 1 {
		prior := replays[0]
		if prior.Fingerprint != fingerprint {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		var receipt voice.TestStyleReceipt
		if err := json.Unmarshal([]byte(prior.Receipt), &receipt); err != nil {
			return voice.TestStyleReceipt{}, err
		}
		if receipt.VoiceID != prior.TargetID || receipt.RequestKey != prior.RequestKey {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		return receipt, tx.Commit()
	}
	// Even a named new synthetic copy cannot name another owner's source. A tombstoned
	// owned synthetic source may be copied explicitly using the tested frozen analysis.
	var source sqlc.GetVoiceRow
	if in.SourceVoiceID != "" {
		source, err = q.GetVoice(ctx, sqlc.GetVoiceParams{ID: in.SourceVoiceID, UserID: in.UserID})
		if errors.Is(err, sql.ErrNoRows) {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		if err != nil {
			return voice.TestStyleReceipt{}, err
		}
	}
	target := created.ID
	if in.Action == "use_setting" {
		if source.DeletedAt.Valid || source.Made != 1 {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		row, err := q.GetAnalysis(ctx, sqlc.GetAnalysisParams{VoiceID: in.SourceVoiceID, UserID: in.UserID, Slot: slotCurrent})
		if errors.Is(err, sql.ErrNoRows) {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		if err != nil {
			return voice.TestStyleReceipt{}, err
		}
		current, err := analysisFromRow(row)
		if err != nil {
			return voice.TestStyleReceipt{}, err
		}
		if voice.AcceptedAnalysisRevision(current) != in.AcceptedRevision || voice.AcceptedAnalysisRevision(in.Analysis) != in.AcceptedRevision {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		if voice.NormalizedOrigin(current.Origin) == voice.OriginPersonal {
			ids := map[string]bool{}
			for _, id := range current.MaterialIDs {
				ids[id] = true
			}
			for _, src := range current.AcceptedSources {
				ids[src.SampleID] = true
			}
			for _, src := range current.AcceptedMaterials {
				ids[src.Source.SampleID] = true
			}
			for id := range ids {
				if id == "" {
					return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
				}
				present, err := q.TestStyleSourcePresent(ctx, sqlc.TestStyleSourcePresentParams{ID: id, UserID: in.UserID, VoiceID: in.SourceVoiceID})
				if err != nil {
					return voice.TestStyleReceipt{}, err
				}
				if present != 1 {
					return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
				}
			}
		}
		target = in.SourceVoiceID
	} else {
		if created.UserID != in.UserID || created.ID == "" {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		inserted := false
		for attempt := 1; attempt <= candidateNameAttempts; attempt++ {
			name := strings.TrimSpace(in.Name)
			if attempt > 1 {
				suffix := " (" + strconv.Itoa(attempt) + ")"
				name = strings.TrimSpace(truncateRunes(name, voice.VoiceNameMaxChars-utf8.RuneCountInString(suffix))) + suffix
			}
			err := q.InsertVoice(ctx, sqlc.InsertVoiceParams{ID: created.ID, UserID: in.UserID, Name: name, CreatedAt: formatTime(created.CreatedAt), UpdatedAt: formatTime(created.UpdatedAt)})
			if err == nil {
				inserted = true
				break
			}
			if !isUniqueViolation(err) {
				return voice.TestStyleReceipt{}, err
			}
		}
		if !inserted {
			return voice.TestStyleReceipt{}, voice.ErrVoiceNameTaken
		}
		snapshot, err := json.Marshal(analysisSnapshot{Version: analysisSnapshotVersion, Origin: voice.OriginSynthetic, SyntheticSample: in.Analysis.SyntheticSample, Counted: in.Analysis.Counted, AI: in.Analysis.AI, MaterialCount: 0})
		if err != nil {
			return voice.TestStyleReceipt{}, err
		}
		known := int64(0)
		if in.Analysis.SourceVersionsKnown {
			known = 1
		}
		if err := q.InsertCurrentAnalysis(ctx, sqlc.InsertCurrentAnalysisParams{VoiceID: target, UserID: in.UserID, Snapshot: string(snapshot), MaterialIds: "[]", AnalyzeModel: in.Analysis.AnalyzeModel, CreatedAt: formatTime(in.Analysis.CreatedAt), SourceVersionsKnown: known, AcceptedSources: "[]", AcceptedMaterialSnapshot: "[]"}); err != nil {
			return voice.TestStyleReceipt{}, err
		}
	}
	stamp := formatTime(created.CreatedAt)
	if in.MakeDefault {
		if err := q.ClearDefaultVoice(ctx, sqlc.ClearDefaultVoiceParams{UserID: in.UserID, UpdatedAt: stamp}); err != nil {
			return voice.TestStyleReceipt{}, err
		}
		n, err := q.SetDefaultVoice(ctx, sqlc.SetDefaultVoiceParams{ID: target, UserID: in.UserID, UpdatedAt: stamp})
		if err != nil {
			return voice.TestStyleReceipt{}, err
		}
		if n != 1 {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
	}
	receipt := voice.TestStyleReceipt{VoiceID: target, RequestKey: in.RequestKey}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return voice.TestStyleReceipt{}, err
	}
	if err := q.InsertTestStylePublication(ctx, sqlc.InsertTestStylePublicationParams{UserID: in.UserID, TestID: in.TestID, WinnerCandidateID: in.WinnerID, Action: in.Action, RequestKey: in.RequestKey, Fingerprint: fingerprint, TargetID: target, Receipt: string(raw), CreatedAt: stamp}); err != nil {
		if isUniqueViolation(err) {
			return voice.TestStyleReceipt{}, voice.ErrTestStylePublicationConflict
		}
		return voice.TestStyleReceipt{}, err
	}
	return receipt, tx.Commit()
}

var _ voice.TestStyleStore = (*Store)(nil)
