// Package store persists the voice context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func (s *Store) GetProfile(ctx context.Context, userID string) (voice.Profile, error) {
	row, err := s.read.GetProfile(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Profile{UserID: userID}, nil
	}
	if err != nil {
		return voice.Profile{}, fmt.Errorf("select profile: %w", err)
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return voice.Profile{}, fmt.Errorf("profile updated_at: %w", err)
	}
	return voice.Profile{
		UserID: row.UserID, Styleguide: row.Styleguide, Rules: row.Rules, UpdatedAt: updated,
	}, nil
}

func (s *Store) UpsertProfile(ctx context.Context, profile voice.Profile) error {
	if err := s.write.UpsertProfile(ctx, sqlc.UpsertProfileParams{
		UserID: profile.UserID, Styleguide: profile.Styleguide, Rules: profile.Rules,
		UpdatedAt: formatTime(profile.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}

func (s *Store) SetStyleguideIfCorpusVersion(ctx context.Context, userID, styleguide string, version int64, now time.Time) (bool, error) {
	n, err := s.write.SetStyleguideIfCorpusVersion(ctx, sqlc.SetStyleguideIfCorpusVersionParams{
		Styleguide: styleguide, UpdatedAt: formatTime(now), UserID: userID, CorpusVersion: version,
	})
	if err != nil {
		return false, fmt.Errorf("set styleguide for corpus version: %w", err)
	}
	return n > 0, nil
}

func (s *Store) SetRules(ctx context.Context, userID, rules string, now time.Time) error {
	if err := s.write.SetRules(ctx, sqlc.SetRulesParams{
		UserID: userID, Rules: rules, UpdatedAt: formatTime(now),
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
		ID: sample.ID, UserID: sample.UserID, Label: sample.Label, Body: sample.Body,
		CreatedAt: formatTime(sample.CreatedAt),
	}); err != nil {
		return fmt.Errorf("insert sample: %w", err)
	}
	if err := queries.BumpCorpusVersion(ctx, sqlc.BumpCorpusVersionParams{
		UserID: sample.UserID, UpdatedAt: formatTime(time.Now()),
	}); err != nil {
		return fmt.Errorf("bump corpus version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert sample: %w", err)
	}
	return nil
}

func (s *Store) ListSamples(ctx context.Context, userID string) ([]voice.Sample, error) {
	rows, err := s.read.ListSamples(ctx, userID)
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
			ID: row.ID, UserID: userID, Label: row.Label, Chars: int(row.Chars.Int64), CreatedAt: created,
		})
	}
	return out, nil
}

func (s *Store) ListSampleBodies(ctx context.Context, userID string) ([]voice.Sample, error) {
	return listSampleBodies(ctx, s.read, userID)
}

func (s *Store) GetSampleBody(ctx context.Context, userID, sampleID string) (*voice.Sample, error) {
	row, err := s.read.GetSampleBody(ctx, sqlc.GetSampleBodyParams{ID: sampleID, UserID: userID})
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
		ID: row.ID, UserID: userID, Label: row.Label, Body: row.Body,
		Chars: utf8.RuneCountInString(row.Body), CreatedAt: created,
	}, nil
}

func (s *Store) CorpusSnapshot(ctx context.Context, userID string) ([]voice.Sample, int64, error) {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("begin corpus snapshot: %w", err)
	}
	defer tx.Rollback()
	queries := s.read.WithTx(tx)
	samples, err := listSampleBodies(ctx, queries, userID)
	if err != nil {
		return nil, 0, err
	}
	version, err := queries.GetCorpusVersion(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("select corpus version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit corpus snapshot: %w", err)
	}
	return samples, version, nil
}

type sampleBodyQueries interface {
	ListSampleBodies(ctx context.Context, userID string) ([]sqlc.ListSampleBodiesRow, error)
}

func listSampleBodies(ctx context.Context, queries sampleBodyQueries, userID string) ([]voice.Sample, error) {
	rows, err := queries.ListSampleBodies(ctx, userID)
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
			ID: row.ID, UserID: userID, Label: row.Label, Body: row.Body,
			Chars: utf8.RuneCountInString(row.Body), CreatedAt: created,
		})
	}
	return out, nil
}

func (s *Store) DeleteSample(ctx context.Context, userID, sampleID string, now time.Time) (bool, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin delete sample: %w", err)
	}
	defer tx.Rollback()
	queries := s.write.WithTx(tx)
	n, err := queries.DeleteSample(ctx, sqlc.DeleteSampleParams{ID: sampleID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("delete sample: %w", err)
	}
	if n > 0 {
		if err := queries.BumpCorpusVersion(ctx, sqlc.BumpCorpusVersionParams{
			UserID: userID, UpdatedAt: formatTime(now),
		}); err != nil {
			return false, fmt.Errorf("bump corpus version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit delete sample: %w", err)
	}
	return n > 0, nil
}

func (s *Store) CountSamples(ctx context.Context, userID string) (int, error) {
	count, err := s.read.CountSamples(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count samples: %w", err)
	}
	return int(count), nil
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
