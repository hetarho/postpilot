package usage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type rateUnavailableError struct{}

func (rateUnavailableError) Error() string { return "KRW/USD reference rate unavailable" }
func (rateUnavailableError) Failure() llm.Failure {
	return llm.Failure{Reason: "AI_FX_RATE_UNAVAILABLE"}
}

var ErrRateUnavailable error = rateUnavailableError{}

var kst = time.FixedZone("Asia/Seoul", 9*60*60)

type RateSource interface {
	KRWPerUSD(ctx context.Context, date time.Time) (referenceE4 int64, published bool, err error)
}

// RateDay is a verified publication or verified absence for one Seoul date.
type RateDay struct {
	Date        string
	ReferenceE4 int64
	Published   bool
}

type RateCache interface {
	RateDay(ctx context.Context, date string) (RateDay, bool, error)
	RecordRateDay(ctx context.Context, day RateDay) error
	LatestRateDay(ctx context.Context, noLaterThan string) (RateDay, bool, error)
}

// RateSelector serializes a day's official lookup across concurrent admissions.
// Confirmed dates survive restarts in RateCache; failed transport never becomes a
// holiday and can only use a recent previously confirmed publication.
type RateSelector struct {
	source     RateSource
	cache      RateCache
	mu         sync.Mutex
	failureDay string
	failureAt  time.Time
	failureErr error
}

func NewRateSelector(source RateSource, cache RateCache) *RateSelector {
	if source == nil || cache == nil {
		panic("usage: rate selector needs official source and persistent cache")
	}
	return &RateSelector{source: source, cache: cache}
}

func (s *RateSelector) Select(ctx context.Context, now time.Time) (plan.RateSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	local := now.In(kst)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, kst)
	first := today.AddDate(0, 0, -1)
	for daysBack := 1; daysBack <= 7; daysBack++ {
		day := today.AddDate(0, 0, -daysBack)
		key := day.Format(time.DateOnly)
		// A weekend is known not to publish. A weekday is skipped only after a
		// verified response from the official source was stored for that date.
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		record, found, err := s.cache.RateDay(ctx, key)
		if err != nil {
			return plan.RateSnapshot{}, err
		}
		if !found {
			if s.failureDay == key && time.Since(s.failureAt) < time.Minute {
				return s.fallback(ctx, first, s.failureErr)
			}
			reference, published, fetchErr := s.source.KRWPerUSD(ctx, day)
			if fetchErr != nil {
				if ctx.Err() == nil {
					s.failureDay, s.failureAt, s.failureErr = key, time.Now(), fetchErr
				}
				return s.fallback(ctx, first, fetchErr)
			}
			if published && reference <= 0 {
				return s.fallback(ctx, first, errors.New("nonpositive official reference"))
			}
			if published {
				if _, rateErr := plan.AppliedRateE4(reference); rateErr != nil {
					return s.fallback(ctx, first, rateErr)
				}
			}
			record = RateDay{Date: key, ReferenceE4: reference, Published: published}
			if err := s.cache.RecordRateDay(ctx, record); err != nil {
				return plan.RateSnapshot{}, err
			}
		}
		if record.Published {
			return snapshot(record, false)
		}
	}
	return plan.RateSnapshot{}, ErrRateUnavailable
}

func (s *RateSelector) fallback(ctx context.Context, first time.Time, cause error) (plan.RateSnapshot, error) {
	record, found, err := s.cache.LatestRateDay(ctx, first.Format(time.DateOnly))
	if err != nil {
		return plan.RateSnapshot{}, err
	}
	if !found {
		return plan.RateSnapshot{}, fmt.Errorf("%w: %v", ErrRateUnavailable, cause)
	}
	date, err := time.ParseInLocation(time.DateOnly, record.Date, kst)
	if err != nil || first.AddDate(0, 0, 1).Sub(date) > 7*24*time.Hour {
		return plan.RateSnapshot{}, fmt.Errorf("%w: last confirmed publication is stale", ErrRateUnavailable)
	}
	return snapshot(record, true)
}

func snapshot(day RateDay, temporary bool) (plan.RateSnapshot, error) {
	if !day.Published || day.ReferenceE4 <= 0 {
		return plan.RateSnapshot{}, ErrRateUnavailable
	}
	applied, err := plan.AppliedRateE4(day.ReferenceE4)
	if err != nil {
		return plan.RateSnapshot{}, err
	}
	return plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: day.Date,
		ReferenceE4: day.ReferenceE4, AppliedE4: applied, Temporary: temporary}, nil
}
