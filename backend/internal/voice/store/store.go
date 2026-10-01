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

// InsertVoice writes the directory row; a voice has no analysis until its first 말투 만들기. The
// partial unique index on active name is the arbiter of a race between two creates.
func (s *Store) InsertVoice(ctx context.Context, v voice.Voice) error {
	isDefault := int64(0)
	if v.IsDefault {
		isDefault = 1
	}
	if err := s.write.InsertVoice(ctx, sqlc.InsertVoiceParams{
		ID: v.ID, UserID: v.UserID, Name: v.Name, IsDefault: isDefault,
		CreatedAt: formatTime(v.CreatedAt), UpdatedAt: formatTime(v.UpdatedAt),
	}); err != nil {
		if isUniqueViolation(err) {
			return voice.ErrVoiceNameTaken
		}
		return fmt.Errorf("insert voice: %w", err)
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
	return toVoice(sqlc.ListVoicesRow(row))
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

// ClearDefaultVoice leaves the account with no default (VOICE-2).
func (s *Store) ClearDefaultVoice(ctx context.Context, userID string, now time.Time) error {
	if err := s.write.ClearDefaultVoice(ctx, sqlc.ClearDefaultVoiceParams{UpdatedAt: formatTime(now), UserID: userID}); err != nil {
		return fmt.Errorf("clear default voice: %w", err)
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

// toVoice maps a directory row. The two directory reads select the same columns, so a
// GetVoice row converts to this one.
func toVoice(row sqlc.ListVoicesRow) (voice.Voice, error) {
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
	var analyzed *time.Time
	if row.AnalyzedAt != "" {
		value, err := parseTime(row.AnalyzedAt)
		if err != nil {
			return voice.Voice{}, fmt.Errorf("voice %s analyzed_at: %w", row.ID, err)
		}
		analyzed = &value
	}
	return voice.Voice{
		ID: row.ID, UserID: row.UserID, Name: row.Name, IsDefault: row.IsDefault == 1, CreatedAt: created, UpdatedAt: updated,
		DeletedAt: deleted, Made: row.Made == 1, SampleCount: int(row.SampleCount), AnalyzedAt: analyzed,
	}, nil
}

// --- profile and samples ---

func (s *Store) InsertSample(ctx context.Context, sample voice.Sample) error {
	if err := s.write.InsertSample(ctx, sampleParams(sample)); err != nil {
		return fmt.Errorf("insert sample: %w", err)
	}
	return nil
}

// AnswerPrompt writes the answer and drops its pending photo upload in one transaction, so a
// photo is never both answered and reclaimable. A rewrite removes the previous answer in the
// same transaction; the partial unique index on (voice_id, prompt_key) is the arbiter of a
// second answer.
func (s *Store) AnswerPrompt(ctx context.Context, sample voice.Sample, uploadID, replaceID string) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin answer prompt: %w", err)
	}
	defer tx.Rollback()
	queries := s.write.WithTx(tx)
	if replaceID != "" {
		_, err := queries.DeleteSample(ctx, sqlc.DeleteSampleParams{ID: replaceID, VoiceID: sample.VoiceID, UserID: sample.UserID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("drop previous answer: %w", err)
		}
	}
	if err := queries.InsertSample(ctx, sampleParams(sample)); err != nil {
		if isUniqueViolation(err) {
			return voice.ErrPromptAnswered
		}
		return fmt.Errorf("insert answer: %w", err)
	}
	if uploadID != "" {
		if err := queries.DeletePhotoUpload(ctx, uploadID); err != nil {
			return fmt.Errorf("drop photo upload: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit answer prompt: %w", err)
	}
	return nil
}

func sampleParams(sample voice.Sample) sqlc.InsertSampleParams {
	params := sqlc.InsertSampleParams{
		ID: sample.ID, VoiceID: sample.VoiceID, UserID: sample.UserID, Kind: string(sample.Kind),
		PromptKey: nullableString(sample.PromptKey), Label: sample.Label, Body: sample.Body,
		PhotoKey: nullableString(sample.PhotoKey), CreatedAt: formatTime(sample.CreatedAt),
	}
	if sample.HasPhoto() {
		params.PhotoWidth = sql.NullInt64{Int64: int64(sample.PhotoWidth), Valid: true}
		params.PhotoHeight = sql.NullInt64{Int64: int64(sample.PhotoHeight), Valid: true}
	}
	return params
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
		sample := voice.Sample{
			ID: row.ID, UserID: userID, VoiceID: voiceID, Kind: voice.SampleKind(row.Kind), PromptKey: row.PromptKey.String,
			Label: row.Label, Chars: int(row.Chars.Int64), PhotoKey: row.PhotoKey.String, CreatedAt: created,
		}
		out = append(out, sample)
	}
	return out, nil
}

func (s *Store) ListSampleBodies(ctx context.Context, userID, voiceID string) ([]voice.Sample, error) {
	rows, err := s.read.ListSampleBodies(ctx, sqlc.ListSampleBodiesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("select sample bodies: %w", err)
	}
	out := make([]voice.Sample, 0, len(rows))
	for _, row := range rows {
		sample, err := toSample(userID, voiceID, sqlc.GetSampleBodyRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	return out, nil
}

func (s *Store) GetSampleBody(ctx context.Context, userID, voiceID, sampleID string) (*voice.Sample, error) {
	row, err := s.read.GetSampleBody(ctx, sqlc.GetSampleBodyParams{ID: sampleID, VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select sample body: %w", err)
	}
	sample, err := toSample(userID, voiceID, row)
	if err != nil {
		return nil, err
	}
	return &sample, nil
}

func toSample(userID, voiceID string, row sqlc.GetSampleBodyRow) (voice.Sample, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.Sample{}, fmt.Errorf("sample %s created_at: %w", row.ID, err)
	}
	return voice.Sample{
		ID: row.ID, UserID: userID, VoiceID: voiceID, Kind: voice.SampleKind(row.Kind), PromptKey: row.PromptKey.String,
		Label: row.Label, Body: row.Body, Chars: utf8.RuneCountInString(row.Body),
		PhotoKey: row.PhotoKey.String, PhotoWidth: int(row.PhotoWidth.Int64), PhotoHeight: int(row.PhotoHeight.Int64),
		CreatedAt: created,
	}, nil
}

func (s *Store) DeleteSample(ctx context.Context, userID, voiceID, sampleID string, _ time.Time) (string, bool, error) {
	key, err := s.write.DeleteSample(ctx, sqlc.DeleteSampleParams{ID: sampleID, VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("delete sample: %w", err)
	}
	return key.String, true, nil
}

func (s *Store) CountSamples(ctx context.Context, userID, voiceID string) (int, error) {
	count, err := s.read.CountSamples(ctx, sqlc.CountSamplesParams{VoiceID: voiceID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("count samples: %w", err)
	}
	return int(count), nil
}

// --- photo uploads ---

func (s *Store) InsertPhotoUpload(ctx context.Context, upload voice.PhotoUpload) error {
	if err := s.write.InsertPhotoUpload(ctx, sqlc.InsertPhotoUploadParams{
		ID: upload.ID, UserID: upload.UserID, VoiceID: upload.VoiceID, PromptKey: upload.PromptKey, ObjectKey: upload.Key,
		ExpiresAt: formatTime(upload.ExpiresAt), CreatedAt: formatTime(upload.CreatedAt),
	}); err != nil {
		return fmt.Errorf("insert photo upload: %w", err)
	}
	return nil
}

func (s *Store) GetPhotoUpload(ctx context.Context, userID, voiceID, uploadID string) (voice.PhotoUpload, error) {
	row, err := s.read.GetPhotoUpload(ctx, sqlc.GetPhotoUploadParams{ID: uploadID, VoiceID: voiceID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.PhotoUpload{}, voice.ErrPhotoRequired
	}
	if err != nil {
		return voice.PhotoUpload{}, fmt.Errorf("select photo upload: %w", err)
	}
	return toPhotoUpload(row)
}

func (s *Store) DeletePhotoUpload(ctx context.Context, id string) error {
	if err := s.write.DeletePhotoUpload(ctx, id); err != nil {
		return fmt.Errorf("delete photo upload: %w", err)
	}
	return nil
}

func (s *Store) ListPhotoUploadsExpiredBefore(ctx context.Context, cutoff time.Time) ([]voice.PhotoUpload, error) {
	rows, err := s.read.ListPhotoUploadsExpiredBefore(ctx, formatTime(cutoff))
	if err != nil {
		return nil, fmt.Errorf("select expired photo uploads: %w", err)
	}
	out := make([]voice.PhotoUpload, 0, len(rows))
	for _, row := range rows {
		upload, err := toPhotoUpload(row)
		if err != nil {
			return nil, err
		}
		out = append(out, upload)
	}
	return out, nil
}

func (s *Store) PhotoKeyInUse(ctx context.Context, key string) (bool, error) {
	n, err := s.read.PhotoKeyInUse(ctx, nullableString(key))
	if err != nil {
		return false, fmt.Errorf("check photo key: %w", err)
	}
	return n == 1, nil
}

// AllReferencedPhotoKeys reads both tables in one read transaction, so a key moving from a
// pending upload to its answer is seen in one of them.
func (s *Store) AllReferencedPhotoKeys(ctx context.Context) (map[string]struct{}, error) {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin referenced photo keys: %w", err)
	}
	defer tx.Rollback()
	queries := s.read.WithTx(tx)
	answered, err := queries.ListSamplePhotoKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("select sample photo keys: %w", err)
	}
	pending, err := queries.ListPhotoUploadKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("select pending photo keys: %w", err)
	}
	keys := make(map[string]struct{}, len(answered)+len(pending))
	for _, key := range answered {
		keys[key.String] = struct{}{}
	}
	for _, key := range pending {
		keys[key] = struct{}{}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit referenced photo keys: %w", err)
	}
	return keys, nil
}

func toPhotoUpload(row sqlc.VoicePhotoUpload) (voice.PhotoUpload, error) {
	expires, err := parseTime(row.ExpiresAt)
	if err != nil {
		return voice.PhotoUpload{}, fmt.Errorf("photo upload %s expires_at: %w", row.ID, err)
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return voice.PhotoUpload{}, fmt.Errorf("photo upload %s created_at: %w", row.ID, err)
	}
	return voice.PhotoUpload{
		ID: row.ID, UserID: row.UserID, VoiceID: row.VoiceID, PromptKey: row.PromptKey, Key: row.ObjectKey,
		ExpiresAt: expires, CreatedAt: created,
	}, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
