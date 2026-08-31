// Package store persists the voice context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	reader *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, reader: reader, write: sqlc.New(writer), read: sqlc.New(reader)}
}

// --- directory ---

// InsertVoice writes the voice and its empty profile row in one transaction. The partial
// unique indexes on active name and active default are the arbiter of a race between two
// creates; either failure surfaces as ErrVoiceNameTaken.
func (s *Store) InsertVoice(ctx context.Context, v voice.Voice) error {
	if !v.SourceLanguage.Valid() {
		return voice.ErrLanguageRequired
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert voice: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	isDefault := int64(0)
	if v.IsDefault {
		isDefault = 1
	}
	if err := q.InsertVoice(ctx, sqlc.InsertVoiceParams{
		ID: v.ID, UserID: v.UserID, Name: v.Name, SourceLanguage: string(v.SourceLanguage), IsDefault: isDefault,
		CreatedAt: formatTime(v.CreatedAt), UpdatedAt: formatTime(v.UpdatedAt),
	}); err != nil {
		if isUniqueViolation(err) {
			return voice.ErrVoiceNameTaken
		}
		return fmt.Errorf("insert voice: %w", err)
	}
	if err := q.UpsertProfile(ctx, sqlc.UpsertProfileParams{
		VoiceID: v.ID, UserID: v.UserID, Styleguide: "", Rules: "", UpdatedAt: formatTime(v.CreatedAt),
	}); err != nil {
		return fmt.Errorf("insert voice profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert voice: %w", err)
	}
	return nil
}

func (s *Store) ListVoices(ctx context.Context, userID string) ([]voice.Voice, error) {
	rows, err := s.read.ListVoices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select voices: %w", err)
	}
	out := make([]voice.Voice, 0, len(rows))
	for _, row := range rows {
		v, err := toVoice(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) GetVoice(ctx context.Context, userID, voiceID string) (voice.Voice, error) {
	row, err := s.read.GetVoice(ctx, sqlc.GetVoiceParams{ID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Voice{}, voice.ErrVoiceNotFound
	}
	if err != nil {
		return voice.Voice{}, fmt.Errorf("select voice: %w", err)
	}
	return toVoice(row)
}

func (s *Store) DefaultVoice(ctx context.Context, userID string) (voice.Voice, bool, error) {
	row, err := s.read.GetDefaultVoice(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Voice{}, false, nil
	}
	if err != nil {
		return voice.Voice{}, false, fmt.Errorf("select default voice: %w", err)
	}
	v, err := toVoice(row)
	return v, err == nil, err
}

func (s *Store) CountActiveVoices(ctx context.Context, userID string) (int, error) {
	n, err := s.read.CountActiveVoices(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count active voices: %w", err)
	}
	return int(n), nil
}

func (s *Store) RenameVoice(ctx context.Context, userID, voiceID, name string, now time.Time) error {
	n, err := s.write.RenameVoice(ctx, sqlc.RenameVoiceParams{Name: name, UpdatedAt: formatTime(now), ID: voiceID, UserID: userID})
	if err != nil {
		if isUniqueViolation(err) {
			return voice.ErrVoiceNameTaken
		}
		return fmt.Errorf("rename voice: %w", err)
	}
	if n == 0 {
		return voice.ErrVoiceNotFound
	}
	return nil
}

// SetDefaultVoice clears and sets inside one transaction so the one-default index never
// sees two defaults, and a target that turns out inactive rolls the clear back.
func (s *Store) SetDefaultVoice(ctx context.Context, userID, voiceID string, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set default voice: %w", err)
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	stamp := formatTime(now)
	if err := q.ClearDefaultVoice(ctx, sqlc.ClearDefaultVoiceParams{UpdatedAt: stamp, UserID: userID}); err != nil {
		return fmt.Errorf("clear default voice: %w", err)
	}
	n, err := q.SetDefaultVoice(ctx, sqlc.SetDefaultVoiceParams{UpdatedAt: stamp, ID: voiceID, UserID: userID})
	if err != nil {
		return fmt.Errorf("set default voice: %w", err)
	}
	if n == 0 {
		return voice.ErrVoiceNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set default voice: %w", err)
	}
	return nil
}

func (s *Store) SoftDeleteVoice(ctx context.Context, userID, voiceID string, now time.Time) (bool, error) {
	stamp := formatTime(now)
	n, err := s.write.SoftDeleteVoice(ctx, sqlc.SoftDeleteVoiceParams{DeletedAt: nullableString(stamp), UpdatedAt: stamp, ID: voiceID, UserID: userID})
	if err != nil {
		if strings.Contains(err.Error(), "voice has publishable work") {
			return false, voice.ErrVoiceBusy
		}
		return false, fmt.Errorf("soft delete voice: %w", err)
	}
	return n > 0, nil
}

func (s *Store) RestoreVoice(ctx context.Context, userID, voiceID string, now time.Time) (bool, error) {
	n, err := s.write.RestoreVoice(ctx, sqlc.RestoreVoiceParams{UpdatedAt: formatTime(now), ID: voiceID, UserID: userID})
	if err != nil {
		if isUniqueViolation(err) {
			return false, voice.ErrVoiceNameTaken
		}
		return false, fmt.Errorf("restore voice: %w", err)
	}
	return n > 0, nil
}

func (s *Store) CountUndecidedVoiceWork(ctx context.Context, voiceID string) (int, error) {
	n, err := s.read.CountUndecidedVoiceWork(ctx, sqlc.CountUndecidedVoiceWorkParams{VoiceID: voiceID, VoiceID_2: voiceID})
	if err != nil {
		return 0, fmt.Errorf("count undecided voice work: %w", err)
	}
	return int(n), nil
}

func toVoice(row sqlc.Voice) (voice.Voice, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.Voice{}, fmt.Errorf("voice %s created_at: %w", row.ID, err)
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return voice.Voice{}, fmt.Errorf("voice %s updated_at: %w", row.ID, err)
	}
	var deleted *time.Time
	if row.DeletedAt.Valid {
		value, err := parseTime(row.DeletedAt.String)
		if err != nil {
			return voice.Voice{}, fmt.Errorf("voice %s deleted_at: %w", row.ID, err)
		}
		deleted = &value
	}
	sourceLanguage, err := voice.ParseLanguage(row.SourceLanguage)
	if err != nil {
		return voice.Voice{}, fmt.Errorf("voice %s source language: %w", row.ID, err)
	}
	return voice.Voice{ID: row.ID, UserID: row.UserID, Name: row.Name, SourceLanguage: sourceLanguage, IsDefault: row.IsDefault == 1, CreatedAt: created, UpdatedAt: updated, DeletedAt: deleted}, nil
}

// --- profile and samples ---

func (s *Store) GetProfile(ctx context.Context, userID, voiceID string) (voice.Profile, error) {
	row, err := s.read.GetProfile(ctx, sqlc.GetProfileParams{VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Profile{UserID: userID, VoiceID: voiceID, Structured: voice.StructuredProfile{Empty: true}}, nil
	}
	if err != nil {
		return voice.Profile{}, fmt.Errorf("select profile: %w", err)
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return voice.Profile{}, fmt.Errorf("profile updated_at: %w", err)
	}
	structured := voice.StructuredProfile{Empty: true}
	if row.CurrentVersion > 0 {
		version, versionErr := s.GetProfileVersion(ctx, userID, voiceID, row.CurrentVersion)
		if versionErr != nil {
			return voice.Profile{}, fmt.Errorf("profile head %d: %w", row.CurrentVersion, versionErr)
		}
		structured = version.Profile
	}
	return voice.Profile{UserID: row.UserID, VoiceID: row.VoiceID, Styleguide: row.Styleguide, Rules: row.Rules,
		UpdatedAt: updated, Structured: structured}, nil
}

func (s *Store) UpsertProfile(ctx context.Context, profile voice.Profile) error {
	if err := s.write.UpsertProfile(ctx, sqlc.UpsertProfileParams{
		VoiceID: profile.VoiceID, UserID: profile.UserID, Styleguide: profile.Styleguide, Rules: profile.Rules,
		UpdatedAt: formatTime(profile.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}

func (s *Store) SetStyleguideIfCorpusVersion(ctx context.Context, userID, voiceID, styleguide string, version int64, now time.Time) (bool, error) {
	n, err := s.write.SetStyleguideIfCorpusVersion(ctx, sqlc.SetStyleguideIfCorpusVersionParams{
		Styleguide: styleguide, UpdatedAt: formatTime(now), VoiceID: voiceID, UserID: userID, CorpusVersion: version,
	})
	if err != nil {
		return false, fmt.Errorf("set styleguide for corpus version: %w", err)
	}
	return n > 0, nil
}

func (s *Store) SetStyleguide(ctx context.Context, userID, voiceID, styleguide string, now time.Time) error {
	if err := s.write.SetStyleguide(ctx, sqlc.SetStyleguideParams{
		VoiceID: voiceID, UserID: userID, Styleguide: styleguide, UpdatedAt: formatTime(now),
	}); err != nil {
		return fmt.Errorf("set styleguide: %w", err)
	}
	return nil
}

func (s *Store) SetRules(ctx context.Context, userID, voiceID, rules string, now time.Time) error {
	if err := s.write.SetRules(ctx, sqlc.SetRulesParams{
		VoiceID: voiceID, UserID: userID, Rules: rules, UpdatedAt: formatTime(now),
	}); err != nil {
		return fmt.Errorf("set rules: %w", err)
	}
	return nil
}

func (s *Store) InsertSample(ctx context.Context, sample voice.Sample) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert sample: %w", err)
	}
	defer tx.Rollback()
	queries := s.write.WithTx(tx)
	if err := queries.InsertSample(ctx, sqlc.InsertSampleParams{
		ID: sample.ID, VoiceID: sample.VoiceID, UserID: sample.UserID, Label: sample.Label, Body: sample.Body,
		CreatedAt: formatTime(sample.CreatedAt),
	}); err != nil {
		return fmt.Errorf("insert sample: %w", err)
	}
	if err := queries.BumpCorpusVersion(ctx, sqlc.BumpCorpusVersionParams{
		VoiceID: sample.VoiceID, UserID: sample.UserID, UpdatedAt: formatTime(time.Now()),
	}); err != nil {
		return fmt.Errorf("bump corpus version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert sample: %w", err)
	}
	return nil
}

func (s *Store) ListSamples(ctx context.Context, userID, voiceID string) ([]voice.Sample, error) {
	rows, err := s.read.ListSamples(ctx, sqlc.ListSamplesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("select samples: %w", err)
	}
	out := make([]voice.Sample, 0, len(rows))
	for _, row := range rows {
		created, err := parseTime(row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("sample %s created_at: %w", row.ID, err)
		}
		out = append(out, voice.Sample{
			ID: row.ID, UserID: userID, VoiceID: voiceID, Label: row.Label, Chars: int(row.Chars.Int64), CreatedAt: created,
		})
	}
	return out, nil
}

func (s *Store) ListSampleBodies(ctx context.Context, userID, voiceID string) ([]voice.Sample, error) {
	return listSampleBodies(ctx, s.read, userID, voiceID)
}

func (s *Store) GetSampleBody(ctx context.Context, userID, voiceID, sampleID string) (*voice.Sample, error) {
	row, err := s.read.GetSampleBody(ctx, sqlc.GetSampleBodyParams{ID: sampleID, VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select sample body: %w", err)
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("sample %s created_at: %w", row.ID, err)
	}
	return &voice.Sample{
		ID: row.ID, UserID: userID, VoiceID: voiceID, Label: row.Label, Body: row.Body,
		Chars: utf8.RuneCountInString(row.Body), CreatedAt: created,
	}, nil
}

func (s *Store) CorpusSnapshot(ctx context.Context, userID, voiceID string) ([]voice.Sample, int64, error) {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("begin corpus snapshot: %w", err)
	}
	defer tx.Rollback()
	queries := s.read.WithTx(tx)
	samples, err := listSampleBodies(ctx, queries, userID, voiceID)
	if err != nil {
		return nil, 0, err
	}
	version, err := queries.GetCorpusVersion(ctx, sqlc.GetCorpusVersionParams{VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		version = 0
	} else if err != nil {
		return nil, 0, fmt.Errorf("select corpus version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit corpus snapshot: %w", err)
	}
	return samples, version, nil
}

type sampleBodyQueries interface {
	ListSampleBodies(ctx context.Context, arg sqlc.ListSampleBodiesParams) ([]sqlc.ListSampleBodiesRow, error)
}

func listSampleBodies(ctx context.Context, queries sampleBodyQueries, userID, voiceID string) ([]voice.Sample, error) {
	rows, err := queries.ListSampleBodies(ctx, sqlc.ListSampleBodiesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("select sample bodies: %w", err)
	}
	out := make([]voice.Sample, 0, len(rows))
	for _, row := range rows {
		created, err := parseTime(row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("sample %s created_at: %w", row.ID, err)
		}
		out = append(out, voice.Sample{
			ID: row.ID, UserID: userID, VoiceID: voiceID, Label: row.Label, Body: row.Body,
			Chars: utf8.RuneCountInString(row.Body), CreatedAt: created,
		})
	}
	return out, nil
}

func (s *Store) DeleteSample(ctx context.Context, userID, voiceID, sampleID string, now time.Time) (bool, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin delete sample: %w", err)
	}
	defer tx.Rollback()
	queries := s.write.WithTx(tx)
	n, err := queries.DeleteSample(ctx, sqlc.DeleteSampleParams{ID: sampleID, VoiceID: voiceID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("delete sample: %w", err)
	}
	if n > 0 {
		if err := queries.BumpCorpusVersion(ctx, sqlc.BumpCorpusVersionParams{
			VoiceID: voiceID, UserID: userID, UpdatedAt: formatTime(now),
		}); err != nil {
			return false, fmt.Errorf("bump corpus version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit delete sample: %w", err)
	}
	return n > 0, nil
}

func (s *Store) CountSamples(ctx context.Context, userID, voiceID string) (int, error) {
	count, err := s.read.CountSamples(ctx, sqlc.CountSamplesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("count samples: %w", err)
	}
	return int(count), nil
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
