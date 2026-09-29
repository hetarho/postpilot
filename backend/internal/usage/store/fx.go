package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/usage/store/sqlc"
)

func (s *Store) RateDay(ctx context.Context, date string) (usage.RateDay, bool, error) {
	row, err := s.read.GetRateDay(ctx, date)
	if errors.Is(err, sql.ErrNoRows) {
		return usage.RateDay{}, false, nil
	}
	if err != nil {
		return usage.RateDay{}, false, fmt.Errorf("read FX publication: %w", err)
	}
	return usage.RateDay{Date: row.PublicationDate, ReferenceE4: row.ReferenceE4.Int64,
		Published: row.State == "published"}, true, nil
}

func (s *Store) RecordRateDay(ctx context.Context, day usage.RateDay) error {
	state := "absent"
	rate := sql.NullInt64{}
	if day.Published {
		state, rate = "published", sql.NullInt64{Int64: day.ReferenceE4, Valid: true}
	}
	return s.write.InsertRateDay(ctx, sqlc.InsertRateDayParams{
		PublicationDate: day.Date, State: state, Source: "korea-eximbank",
		ReferenceE4: rate, VerifiedAt: time.Now().UTC().Format(writeLayout),
	})
}

func (s *Store) LatestRateDay(ctx context.Context, noLaterThan string) (usage.RateDay, bool, error) {
	row, err := s.read.LatestPublishedRateDay(ctx, noLaterThan)
	if errors.Is(err, sql.ErrNoRows) {
		return usage.RateDay{}, false, nil
	}
	if err != nil {
		return usage.RateDay{}, false, fmt.Errorf("read latest FX publication: %w", err)
	}
	return usage.RateDay{Date: row.PublicationDate, ReferenceE4: row.ReferenceE4.Int64,
		Published: true}, true, nil
}

var _ usage.RateCache = (*Store)(nil)
