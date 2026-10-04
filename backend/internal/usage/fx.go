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

// RateSelector picks the official rate a Seoul day's admissions convert at. Once a day's
// rate is selected from a confirmed publication it is served from memory under a read
// lock; a date nobody has confirmed is looked up outside the lock, once, by every caller
// that missed it together. Confirmed dates survive restarts in RateCache; failed transport
// never becomes a holiday and can only use a recent previously confirmed publication.
type RateSelector struct {
	source RateSource
	cache  RateCache

	mu sync.RWMutex
	// selectedDay is the latest Seoul day whose rate came from a confirmed publication. A
	// temporary fallback is never kept here, so the next caller tries the source again.
	selectedDay string
	selected    plan.RateSnapshot
	failureDay  string
	failureAt   time.Time
	failureErr  error
	lookups     map[string]*rateLookup
}

// rateLookup is one official lookup for one date, shared by every caller waiting on it.
type rateLookup struct {
	done   chan struct{}
	record RateDay
	// err is a cache failure, returned as it is; unusable is a source failure or an answer
	// that cannot be used, which falls back to the last confirmed publication.
	err      error
	unusable error
}

func NewRateSelector(source RateSource, cache RateCache) *RateSelector {
	if source == nil || cache == nil {
		panic("usage: rate selector needs official source and persistent cache")
	}
	return &RateSelector{source: source, cache: cache}
}

func (s *RateSelector) Select(ctx context.Context, now time.Time) (plan.RateSnapshot, error) {
	local := now.In(kst)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, kst)
	todayKey := today.Format(time.DateOnly)
	s.mu.RLock()
	if s.selectedDay == todayKey {
		selected := s.selected
		s.mu.RUnlock()
		return selected, nil
	}
	s.mu.RUnlock()
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
			if failed, cause := s.recentFailure(key); failed {
				return s.fallback(ctx, first, cause)
			}
			lookup := s.lookup(ctx, day, key)
			select {
			case <-lookup.done:
			case <-ctx.Done():
				return s.fallback(ctx, first, ctx.Err())
			}
			if lookup.err != nil {
				return plan.RateSnapshot{}, lookup.err
			}
			if lookup.unusable != nil {
				return s.fallback(ctx, first, lookup.unusable)
			}
			record = lookup.record
		}
		if record.Published {
			selected, err := snapshot(record, false)
			if err == nil {
				s.remember(todayKey, selected)
			}
			return selected, err
		}
	}
	return plan.RateSnapshot{}, ErrRateUnavailable
}

// remember keeps a day's confirmed selection; a caller still asking about an earlier day
// never displaces a later one.
func (s *RateSelector) remember(day string, selected plan.RateSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if day >= s.selectedDay {
		s.selectedDay, s.selected = day, selected
	}
}

// recentFailure is the one-minute memo of a failed lookup for a date: within it, callers
// fall back instead of asking the source again.
func (s *RateSelector) recentFailure(key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failureDay == key && time.Since(s.failureAt) < time.Minute, s.failureErr
}

// lookup joins the date's official lookup in flight, or starts it. The lookup runs on a
// context no single caller can cancel, so one caller giving up never fails the others; the
// source's own client timeout bounds it.
func (s *RateSelector) lookup(ctx context.Context, day time.Time, key string) *rateLookup {
	s.mu.Lock()
	defer s.mu.Unlock()
	if running, ok := s.lookups[key]; ok {
		return running
	}
	if s.lookups == nil {
		s.lookups = map[string]*rateLookup{}
	}
	lookup := &rateLookup{done: make(chan struct{})}
	s.lookups[key] = lookup
	go func() {
		s.fetch(context.WithoutCancel(ctx), day, key, lookup)
		s.mu.Lock()
		delete(s.lookups, key)
		s.mu.Unlock()
		close(lookup.done)
	}()
	return lookup
}

// fetch asks the official source about one date and records a usable answer. It reads the
// cache and the failure memo once more first: a lookup that finished between a caller's
// miss and this one has already recorded the date, or failed on it a moment ago.
func (s *RateSelector) fetch(ctx context.Context, day time.Time, key string, lookup *rateLookup) {
	record, found, err := s.cache.RateDay(ctx, key)
	if err != nil || found {
		lookup.record, lookup.err = record, err
		return
	}
	if failed, cause := s.recentFailure(key); failed {
		lookup.unusable = cause
		return
	}
	reference, published, fetchErr := s.source.KRWPerUSD(ctx, day)
	if fetchErr != nil {
		s.mu.Lock()
		s.failureDay, s.failureAt, s.failureErr = key, time.Now(), fetchErr
		s.mu.Unlock()
		lookup.unusable = fetchErr
		return
	}
	if published && reference <= 0 {
		lookup.unusable = errors.New("nonpositive official reference")
		return
	}
	if published {
		if _, rateErr := plan.AppliedRateE4(reference); rateErr != nil {
			lookup.unusable = rateErr
			return
		}
	}
	lookup.record = RateDay{Date: key, ReferenceE4: reference, Published: published}
	lookup.err = s.cache.RecordRateDay(ctx, lookup.record)
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
