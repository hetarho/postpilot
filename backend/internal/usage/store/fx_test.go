package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type datedRates struct {
	mu    sync.Mutex
	calls []string
	data  map[string]int64
	err   error
}

func (s *datedRates) KRWPerUSD(_ context.Context, day time.Time) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := day.Format(time.DateOnly)
	s.calls = append(s.calls, key)
	if s.err != nil {
		return 0, false, s.err
	}
	rate, ok := s.data[key]
	return rate, ok, nil
}

func TestOfficialFXCalendarCacheAndTemporaryFallback(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "fx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	store := usagestore.New(handle.Writer, handle.Reader)
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	at := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 12, 0, 0, 0, seoul)
	}
	source := &datedRates{data: map[string]int64{"2026-09-25": 13_579_001}}
	selector := usage.NewRateSelector(source, store)
	// Monday skips Saturday and Sunday; the verified Friday reference is normal.
	friday, err := selector.Select(ctx, at(2026, time.September, 28))
	if err != nil || friday.PublicationDate != "2026-09-25" || friday.AppliedE4 != 13_600_000 || friday.Temporary {
		t.Fatalf("weekend selection=%+v err=%v", friday, err)
	}
	if len(source.calls) != 1 || source.calls[0] != "2026-09-25" {
		t.Fatalf("weekend fetched %v", source.calls)
	}
	// Tuesday's verified nonpublication moves Wednesday's target to Monday;
	// repeated and restarted reads use the persisted date records.
	source.data["2026-09-28"] = 13_601_000
	monday, err := selector.Select(ctx, at(2026, time.September, 30))
	if err != nil || monday.PublicationDate != "2026-09-28" || monday.Temporary {
		t.Fatalf("holiday selection=%+v err=%v", monday, err)
	}
	calls := len(source.calls)
	if _, err := usage.NewRateSelector(source, store).Select(ctx, at(2026, time.September, 30)); err != nil || len(source.calls) != calls {
		t.Fatalf("persisted publication cache: calls=%v err=%v", source.calls, err)
	}

	source.err = errors.New("transport unavailable")
	fallback, err := selector.Select(ctx, at(2026, time.October, 1))
	if err != nil || fallback.PublicationDate != "2026-09-28" || !fallback.Temporary {
		t.Fatalf("fresh fallback=%+v err=%v", fallback, err)
	}
	calls = len(source.calls)
	if _, err := selector.Select(ctx, at(2026, time.October, 1)); err != nil || len(source.calls) != calls {
		t.Fatalf("fallback lookup repeated during short outage: calls=%v err=%v", source.calls, err)
	}
	if _, err := selector.Select(ctx, at(2026, time.October, 6)); !errors.Is(err, usage.ErrRateUnavailable) {
		t.Fatalf("stale fallback err=%v", err)
	}
}

func TestConcurrentFXSelectorsShareOnePublicationLookup(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "fx-concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	source := &datedRates{data: map[string]int64{"2026-09-29": 13_579_001}}
	selector := usage.NewRateSelector(source, usagestore.New(handle.Writer, handle.Reader))
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	var group sync.WaitGroup
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := selector.Select(ctx, now); err != nil {
				t.Errorf("select: %v", err)
			}
		}()
	}
	group.Wait()
	if len(source.calls) != 1 {
		t.Fatalf("official source calls=%v", source.calls)
	}
}
