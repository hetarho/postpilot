package usage

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// heldRates publishes every date, but only once release is closed, and counts lookups.
type heldRates struct {
	fetches atomic.Int32
	started chan string
	release chan struct{}
}

func newHeldRates() *heldRates {
	return &heldRates{started: make(chan string, 64), release: make(chan struct{})}
}

func (r *heldRates) KRWPerUSD(_ context.Context, day time.Time) (int64, bool, error) {
	r.fetches.Add(1)
	r.started <- day.Format(time.DateOnly)
	<-r.release
	return 13_579_001, true, nil
}

// countedRateCache is an in-memory RateCache that counts its date reads.
type countedRateCache struct {
	mu    sync.Mutex
	reads int
	days  map[string]RateDay
}

func (c *countedRateCache) RateDay(_ context.Context, date string) (RateDay, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reads++
	day, ok := c.days[date]
	return day, ok, nil
}
func (c *countedRateCache) RecordRateDay(_ context.Context, day RateDay) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.days[day.Date] = day
	return nil
}
func (c *countedRateCache) LatestRateDay(_ context.Context, date string) (RateDay, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	day, ok := c.days[date]
	return day, ok, nil
}
func (c *countedRateCache) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

// seoulDay is noon of a Seoul date; the selection converts at the weekday before it.
func seoulDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, kst)
}

func TestRateSelectorSharesOneLookupAcrossConcurrentCallers(t *testing.T) {
	source, cache := newHeldRates(), &countedRateCache{days: map[string]RateDay{}}
	selector := NewRateSelector(source, cache)
	wednesday := seoulDay(2026, time.September, 30)
	const callers = 50
	results := make(chan error, callers)
	for range callers {
		go func() {
			rate, err := selector.Select(context.Background(), wednesday)
			if err == nil && (rate.PublicationDate != "2026-09-29" || rate.Temporary) {
				t.Errorf("selected %+v, want Tuesday's confirmed publication", rate)
			}
			results <- err
		}()
	}
	if day := <-source.started; day != "2026-09-29" {
		t.Fatalf("looked up %s, want the weekday before", day)
	}
	// Every caller has missed the cache (plus the lookup's own re-read) before the source
	// answers, so all of them are waiting on the one lookup in flight.
	deadline := time.Now().Add(10 * time.Second)
	for cache.readCount() < callers+1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(source.release)
	for range callers {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if fetches := source.fetches.Load(); fetches != 1 {
		t.Fatalf("official source fetched %d times for one date, want 1", fetches)
	}
}

func TestRateSelectorServesASelectedDayWithoutWaitingOnALookup(t *testing.T) {
	source := newHeldRates()
	cache := &countedRateCache{days: map[string]RateDay{
		"2026-09-29": {Date: "2026-09-29", ReferenceE4: 13_579_001, Published: true},
	}}
	selector := NewRateSelector(source, cache)
	ctx := context.Background()
	wednesday, thursday := seoulDay(2026, time.September, 30), seoulDay(2026, time.October, 1)
	selected, err := selector.Select(ctx, wednesday)
	if err != nil || selected.PublicationDate != "2026-09-29" {
		t.Fatalf("selected %+v err %v", selected, err)
	}

	// The next day's lookup is in flight and held.
	nextDay := make(chan error, 1)
	go func() {
		_, err := selector.Select(ctx, thursday)
		nextDay <- err
	}()
	if day := <-source.started; day != "2026-09-30" {
		t.Fatalf("looked up %s", day)
	}

	reads := cache.readCount()
	const callers = 50
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			if rate, err := selector.Select(ctx, wednesday); err != nil || rate != selected {
				t.Errorf("selected %+v err %v, want the day's selection", rate, err)
			}
		}()
	}
	served := make(chan struct{})
	go func() { group.Wait(); close(served) }()
	select {
	case <-served:
	case <-time.After(10 * time.Second):
		t.Fatal("a selected day waited on another day's lookup")
	}
	if after := cache.readCount(); after != reads {
		t.Fatalf("a selected day read the cache %d times, want none", after-reads)
	}

	close(source.release)
	if err := <-nextDay; err != nil {
		t.Fatal(err)
	}
	if fetches := source.fetches.Load(); fetches != 1 {
		t.Fatalf("official source fetched %d times, want 1", fetches)
	}
}
