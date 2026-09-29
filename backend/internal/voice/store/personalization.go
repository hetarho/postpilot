package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func encodeProfile(value voice.StructuredProfile) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}
func decodeProfile(value string) (voice.StructuredProfile, error) {
	var out voice.StructuredProfile
	err := json.Unmarshal([]byte(value), &out)
	return out, err
}

func (s *Store) ListProfileVersions(ctx context.Context, userID, voiceID string) ([]voice.ProfileVersion, error) {
	rows, err := s.read.ListProfileVersions(ctx, sqlc.ListProfileVersionsParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.ProfileVersion, 0, len(rows))
	for _, row := range rows {
		item, err := profileVersion(sqlc.VoiceProfileVersion{
			ID: row.ID, UserID: row.UserID, VoiceID: row.VoiceID, Version: row.Version,
			Snapshot: row.Snapshot, Origin: row.Origin,
			RestoredFromVersion: row.RestoredFromVersion, CreatedAt: row.CreatedAt,
		})
		if err != nil {
			return nil, err
		}
		// The LEFT JOIN carries presence only: the list says whether a version CAN be
		// previewed, and the snapshot itself is fetched per version on open (VOICE-29).
		item.HasSample = row.SampleVersion.Valid
		out = append(out, item)
	}
	return out, nil
}
func (s *Store) GetProfileVersion(ctx context.Context, userID, voiceID string, version int64) (voice.ProfileVersion, error) {
	row, err := s.read.GetProfileVersion(ctx, sqlc.GetProfileVersionParams{VoiceID: voiceID, UserID: userID, Version: version})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileVersion{}, voice.ErrLearningNotFound
	}
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	return profileVersion(row)
}
func profileVersion(row sqlc.VoiceProfileVersion) (voice.ProfileVersion, error) {
	p, err := decodeProfile(row.Snapshot)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	return voice.ProfileVersion{ID: row.ID, UserID: row.UserID, VoiceID: row.VoiceID, Version: row.Version, Profile: p, Origin: row.Origin, RestoredFromVersion: row.RestoredFromVersion.Int64, CreatedAt: created}, nil
}

func (s *Store) PublishProfileVersion(ctx context.Context, userID, voiceID string, profile voice.StructuredProfile, origin string, restoredFrom int64, now time.Time) (voice.ProfileVersion, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	version, err := publishProfileWithQueries(ctx, q, userID, voiceID, profile, origin, restoredFrom, now)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	if err = tx.Commit(); err != nil {
		return voice.ProfileVersion{}, err
	}
	return version, nil
}

func (s *Store) PublishProfileVersionIfHead(ctx context.Context, userID, voiceID string, profile voice.StructuredProfile, origin string, expectedHead int64, now time.Time) (voice.ProfileVersion, bool, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.ProfileVersion{}, false, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	current := int64(0)
	row, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if err == nil {
		current = row.CurrentVersion
	} else if !errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileVersion{}, false, err
	}
	if current != expectedHead {
		if err = tx.Commit(); err != nil {
			return voice.ProfileVersion{}, false, err
		}
		return voice.ProfileVersion{}, false, nil
	}
	version, err := publishProfileWithQueries(ctx, q, userID, voiceID, profile, origin, 0, now)
	if err != nil {
		return voice.ProfileVersion{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return voice.ProfileVersion{}, false, err
	}
	return version, true, nil
}

// publishProfileWithQueries appends the next immutable version for ONE voice and moves that
// voice's head; version numbers count per voice, so two voices both at v1 is the normal case.
func publishProfileWithQueries(ctx context.Context, q *sqlc.Queries, userID, voiceID string, profile voice.StructuredProfile, origin string, restoredFrom int64, now time.Time) (voice.ProfileVersion, error) {
	current := int64(0)
	row, err := q.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if err == nil {
		current = row.CurrentVersion
	} else if !errors.Is(err, sql.ErrNoRows) {
		return voice.ProfileVersion{}, err
	}
	profile.Version, profile.UpdatedAt = current+1, now
	encoded, err := encodeProfile(profile)
	if err != nil {
		return voice.ProfileVersion{}, err
	}
	id := storeID()
	if err = q.InsertProfileVersion(ctx, sqlc.InsertProfileVersionParams{ID: id, UserID: userID, VoiceID: voiceID, Version: profile.Version, Snapshot: encoded, Origin: origin, RestoredFromVersion: sql.NullInt64{Int64: restoredFrom, Valid: restoredFrom > 0}, CreatedAt: formatTime(now)}); err != nil {
		return voice.ProfileVersion{}, err
	}
	if err = q.SetProfileHead(ctx, sqlc.SetProfileHeadParams{VoiceID: voiceID, UserID: userID, CurrentVersion: profile.Version, UpdatedAt: formatTime(now)}); err != nil {
		return voice.ProfileVersion{}, err
	}
	return voice.ProfileVersion{ID: id, UserID: userID, VoiceID: voiceID, Version: profile.Version, Profile: profile, Origin: origin, RestoredFromVersion: restoredFrom, CreatedAt: now}, nil
}

func (s *Store) ListManualOverrides(ctx context.Context, userID, voiceID string) ([]voice.ManualOverride, error) {
	rows, err := s.read.ListManualOverrides(ctx, sqlc.ListManualOverridesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]voice.ManualOverride, 0, len(rows))
	for _, r := range rows {
		at, e := parseTime(r.UpdatedAt)
		if e != nil {
			return nil, e
		}
		out = append(out, voice.ManualOverride{UserID: r.UserID, VoiceID: r.VoiceID, Layer: voice.RuleLayer(r.Layer), Field: r.Field, Value: r.Value, UpdatedAt: at})
	}
	return out, nil
}
func (s *Store) SetManualOverride(ctx context.Context, v voice.ManualOverride) error {
	return s.write.UpsertManualOverride(ctx, sqlc.UpsertManualOverrideParams{VoiceID: v.VoiceID, UserID: v.UserID, Layer: string(v.Layer), Field: v.Field, Value: v.Value, UpdatedAt: formatTime(v.UpdatedAt)})
}
func (s *Store) DeleteManualOverride(ctx context.Context, userID, voiceID string, layer voice.RuleLayer, field string) (bool, error) {
	n, err := s.write.DeleteManualOverride(ctx, sqlc.DeleteManualOverrideParams{VoiceID: voiceID, UserID: userID, Layer: string(layer), Field: field})
	return n > 0, err
}
func (s *Store) ApplyOverrideAndPublish(ctx context.Context, override voice.ManualOverride, value *string, profile voice.StructuredProfile, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if value == nil {
		if _, err = q.DeleteManualOverride(ctx, sqlc.DeleteManualOverrideParams{VoiceID: override.VoiceID, UserID: override.UserID, Layer: string(override.Layer), Field: override.Field}); err != nil {
			return err
		}
	} else {
		if err = q.UpsertManualOverride(ctx, sqlc.UpsertManualOverrideParams{VoiceID: override.VoiceID, UserID: override.UserID, Layer: string(override.Layer), Field: override.Field, Value: *value, UpdatedAt: formatTime(now)}); err != nil {
			return err
		}
	}
	if _, err = publishProfileWithQueries(ctx, q, override.UserID, override.VoiceID, profile, "manual", 0, now); err != nil {
		return err
	}
	return tx.Commit()
}

func storeID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
