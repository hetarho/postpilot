// Package store persists the quality context. Generated SQL types stop at this edge.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/quality"
	"github.com/postpilot/backend/internal/quality/store/sqlc"
)

// The fixed-width UTC layout every context writes timestamps in, so string comparison
// and ORDER BY agree with chronological order.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	write  *sqlc.Queries
	read   *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

var _ quality.Measurements = (*Store)(nil)

// Measurement reads a post's row within its account; false means it has none.
func (s *Store) Measurement(ctx context.Context, userID, slug string) (quality.StoredMeasurement, bool, error) {
	row, err := s.read.GetPostMeasurement(ctx, sqlc.GetPostMeasurementParams{PostSlug: slug, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return quality.StoredMeasurement{}, false, nil
	}
	if err != nil {
		return quality.StoredMeasurement{}, false, fmt.Errorf("select post measurement: %w", err)
	}
	computed, err := parseTime(row.ComputedAt)
	if err != nil {
		return quality.StoredMeasurement{}, false, fmt.Errorf("parse post measurement computed_at: %w", err)
	}
	return quality.StoredMeasurement{
		PostSlug: slug, UserID: row.UserID, Revision: row.ContentRevision, MeasureVersion: int(row.MeasureVersion),
		Self: quality.Self{
			Repetition: quality.Repetition{
				Share: floatOf(row.RepetitionShare), TitleRelevance: floatOf(row.TitleRelevance),
			},
			Composition: quality.Composition{
				CharCount: int(row.CharCount), PhotoCount: int(row.PhotoCount), DistinctBlockTypes: int(row.DistinctBlockTypes),
				AvgSentenceLength: floatOf(row.AvgSentenceLength),
			},
		},
		ComputedAt: computed,
	}, true, nil
}

// SaveMeasurement writes the row, replacing the revision it held.
func (s *Store) SaveMeasurement(ctx context.Context, m quality.StoredMeasurement) error {
	err := s.write.UpsertPostMeasurement(ctx, sqlc.UpsertPostMeasurementParams{
		PostSlug: m.PostSlug, UserID: m.UserID, ContentRevision: m.Revision, MeasureVersion: int64(m.MeasureVersion),
		CharCount:          int64(m.Self.Composition.CharCount),
		PhotoCount:         int64(m.Self.Composition.PhotoCount),
		DistinctBlockTypes: int64(m.Self.Composition.DistinctBlockTypes),
		AvgSentenceLength:  nullFloat(m.Self.Composition.AvgSentenceLength),
		RepetitionShare:    nullFloat(m.Self.Repetition.Share),
		TitleRelevance:     nullFloat(m.Self.Repetition.TitleRelevance),
		ComputedAt:         formatTime(m.ComputedAt),
	})
	if err != nil {
		return fmt.Errorf("upsert post measurement: %w", err)
	}
	return nil
}

// floatOf and nullFloat are the one place an absent value crosses the SQL edge: NULL and nil
// mean the same thing on both sides, a value that cannot be computed (QUAL-40).
func floatOf(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func nullFloat(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func formatTime(value time.Time) string { return value.UTC().Format(writeLayout) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
