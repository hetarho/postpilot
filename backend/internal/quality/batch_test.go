package quality

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var batchNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

type searchCall struct {
	query string
	start int
}

// fakeSearch answers each query from pages keyed by start, or fails from fail; it records every
// call. block makes it wait for its context, for the cancel test.
type fakeSearch struct {
	mu      sync.Mutex
	calls   []searchCall
	pages   func(query string, start int) []SearchItem
	fail    func(query string, start int) error
	block   bool
	started chan struct{}
}

func (f *fakeSearch) SearchBlog(ctx context.Context, query string, start int) ([]SearchItem, error) {
	f.mu.Lock()
	f.calls = append(f.calls, searchCall{query, start})
	f.mu.Unlock()
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.fail != nil {
		if err := f.fail(query, start); err != nil {
			return nil, err
		}
	}
	if f.pages == nil {
		return nil, nil
	}
	return f.pages(query, start), nil
}

func (f *fakeSearch) recorded() []searchCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]searchCall(nil), f.calls...)
}

// fakePhraseStore holds rows by field and records reads and writes.
type fakePhraseStore struct {
	mu            sync.Mutex
	rows          map[string]PhraseList
	reads, writes int
}

func (f *fakePhraseStore) PhraseList(_ context.Context, field string) (PhraseList, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	row, ok := f.rows[field]
	return row, ok, nil
}

func (f *fakePhraseStore) ReplacePhraseList(_ context.Context, list PhraseList) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	f.rows[list.Field] = list
	return nil
}

func newBatch(t *testing.T, search BlogSearch, interval time.Duration) (*PhraseBatch, *fakePhraseStore) {
	t.Helper()
	store := &fakePhraseStore{rows: map[string]PhraseList{}}
	batch, err := NewPhraseBatch(store, search, interval, func() time.Time { return batchNow })
	if err != nil {
		t.Fatal(err)
	}
	return batch, store
}

// notDue seeds every field but the named ones with a row due tomorrow.
func notDue(store *fakePhraseStore, except ...string) {
	skip := map[string]bool{}
	for _, id := range except {
		skip[id] = true
	}
	for _, field := range Fields() {
		if !skip[field.ID] {
			store.rows[field.ID] = PhraseList{Field: field.ID, Phrases: []string{"그대로"}, NextRefreshAt: batchNow.Add(24 * time.Hour)}
		}
	}
}

func items(n int, title string) []SearchItem {
	out := make([]SearchItem, n)
	for i := range out {
		out[i] = SearchItem{Title: fmt.Sprintf("%s %d", title, i), Description: title + " 추천 후기"}
	}
	return out
}

func queryOf(t *testing.T, id string) string {
	t.Helper()
	field, ok := FieldByID(id)
	if !ok {
		t.Fatalf("no field %s", id)
	}
	return field.Query
}

func TestPhraseBatchRefreshesOnlyDueFields(t *testing.T) {
	search := &fakeSearch{}
	batch, store := newBatch(t, search, time.Hour)
	notDue(store)
	// One field due exactly now and one with no row at all; the rest are due tomorrow.
	store.rows["cafe"] = PhraseList{Field: "cafe", NextRefreshAt: batchNow}
	delete(store.rows, "pets")
	if err := batch.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var queried []string
	for _, call := range search.recorded() {
		queried = append(queried, call.query)
	}
	if want := []string{queryOf(t, "cafe"), queryOf(t, "pets")}; !reflect.DeepEqual(queried, want) {
		t.Fatalf("searched %q, want the two due fields in catalogue order %q", queried, want)
	}
	if store.writes != 2 || store.rows["restaurant"].Phrases[0] != "그대로" {
		t.Fatalf("writes = %d, restaurant = %+v", store.writes, store.rows["restaurant"])
	}
}

func TestPhraseBatchFetchesThreePagesOfOneHundred(t *testing.T) {
	search := &fakeSearch{pages: func(_ string, start int) []SearchItem {
		return items(PhrasePageSize, fmt.Sprintf("성수 카페 %d", start))
	}}
	batch, store := newBatch(t, search, time.Hour)
	notDue(store, "cafe")
	if err := batch.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	query := queryOf(t, "cafe")
	if want := []searchCall{{query, 1}, {query, 101}, {query, 201}}; !reflect.DeepEqual(search.recorded(), want) {
		t.Fatalf("calls = %+v, want %+v", search.recorded(), want)
	}
	row := store.rows["cafe"]
	if row.CorpusSize != 300 || len(row.Phrases) == 0 || row.RefreshedAt == nil || !row.RefreshedAt.Equal(batchNow) || !row.NextRefreshAt.Equal(batchNow.Add(time.Hour)) {
		t.Fatalf("row = %+v", row)
	}
	want := make([]string, 0)
	for _, phrase := range ExtractPhrases(unitsOf(append(append(items(100, "성수 카페 1"), items(100, "성수 카페 101")...), items(100, "성수 카페 201")...))) {
		want = append(want, phrase.Text)
	}
	if !reflect.DeepEqual(row.Phrases, want) {
		t.Fatalf("phrases = %q, want the extractor's %q", row.Phrases, want)
	}
	// Titles and descriptions both count: 성수 카페 is in all 600 of them, 추천 후기 only in the
	// 300 descriptions.
	if len(row.Phrases) < 2 || row.Phrases[0] != "성수 카페" || row.Phrases[1] != "추천 후기" {
		t.Fatalf("the list opens %q", row.Phrases)
	}
}

func TestPhraseBatchShortCorpusIsASuccess(t *testing.T) {
	search := &fakeSearch{pages: func(string, int) []SearchItem { return items(40, "을지로 노포") }}
	batch, store := newBatch(t, search, 6*time.Hour)
	notDue(store, "restaurant")
	if err := batch.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls := search.recorded(); len(calls) != 1 || calls[0].start != 1 {
		t.Fatalf("a short first page was followed by %+v", calls)
	}
	row := store.rows["restaurant"]
	if row.CorpusSize != 40 || row.RefreshedAt == nil || !row.NextRefreshAt.Equal(batchNow.Add(6*time.Hour)) {
		t.Fatalf("row = %+v", row)
	}
	// An empty corpus is a success too: an empty list, refreshed now.
	empty, emptyStore := newBatch(t, &fakeSearch{}, time.Hour)
	notDue(emptyStore, "restaurant")
	if err := empty.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := emptyStore.rows["restaurant"]; row.CorpusSize != 0 || len(row.Phrases) != 0 || row.RefreshedAt == nil {
		t.Fatalf("an empty corpus = %+v", row)
	}
}

func TestPhraseBatchPageFailureKeepsTheLastList(t *testing.T) {
	boom := errors.New("naver said no")
	search := &fakeSearch{
		pages: func(string, int) []SearchItem { return items(PhrasePageSize, "성수 카페") },
		fail: func(_ string, start int) error {
			if start == 101 {
				return boom
			}
			return nil
		},
	}
	batch, store := newBatch(t, search, 24*time.Hour)
	notDue(store, "cafe")
	refreshed := batchNow.Add(-30 * time.Hour)
	last := PhraseList{Field: "cafe", Phrases: []string{"분위기 좋은", "성수 카페"}, CorpusSize: 287, RefreshedAt: &refreshed, NextRefreshAt: batchNow.Add(-time.Hour)}
	store.rows["cafe"] = last

	err := batch.RunOnce(context.Background())
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "refresh cafe") {
		t.Fatalf("err = %v, want the page failure naming its field", err)
	}
	want := last
	want.NextRefreshAt = batchNow.Add(PhraseRetryDelay)
	if got := store.rows["cafe"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("after a failed page = %+v, want %+v", got, want)
	}
}

func TestPhraseBatchFirstFailureSchedulesAnEmptyRow(t *testing.T) {
	search := &fakeSearch{fail: func(string, int) error { return errors.New("unavailable") }}
	batch, store := newBatch(t, search, time.Hour)
	notDue(store, "pets")
	if err := batch.RunOnce(context.Background()); err == nil {
		t.Fatal("a failed first fetch reported success")
	}
	want := PhraseList{Field: "pets", Phrases: []string{}, NextRefreshAt: batchNow.Add(PhraseRetryDelay)}
	if got := store.rows["pets"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("first failure row = %+v, want %+v", got, want)
	}
}

func TestPhraseBatchOneFailingFieldDoesNotBlockTheOthers(t *testing.T) {
	failing := queryOf(t, "domestic_travel")
	search := &fakeSearch{
		pages: func(string, int) []SearchItem { return items(3, "기록") },
		fail: func(query string, _ int) error {
			if query == failing {
				return errors.New("timeout")
			}
			return nil
		},
	}
	batch, store := newBatch(t, search, time.Hour)
	err := batch.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refresh domestic_travel") || strings.Count(err.Error(), "refresh ") != 1 {
		t.Fatalf("err = %v, want one failure naming domestic_travel", err)
	}
	if len(store.rows) != len(Fields()) {
		t.Fatalf("%d of %d fields written", len(store.rows), len(Fields()))
	}
	for _, field := range Fields() {
		if field.ID == "domestic_travel" {
			continue
		}
		if row := store.rows[field.ID]; row.RefreshedAt == nil || row.CorpusSize != 3 {
			t.Errorf("%s = %+v", field.ID, row)
		}
	}
}

func TestPhraseBatchDisabledMakesNoCalls(t *testing.T) {
	batch, store := newBatch(t, nil, time.Hour)
	if err := batch.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { batch.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a disabled batch kept running")
	}
	if store.reads != 0 || store.writes != 0 {
		t.Fatalf("a disabled batch touched the store: %d reads, %d writes", store.reads, store.writes)
	}
}

func TestPhraseBatchCatchesUpOnBoot(t *testing.T) {
	// An hour between checks: whatever runs within the test ran before any tick.
	search := &fakeSearch{started: make(chan struct{}, 1)}
	batch, _ := newBatch(t, search, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go batch.Run(ctx)
	select {
	case <-search.started:
	case <-time.After(2 * time.Second):
		t.Fatal("the boot catch-up never searched")
	}

	// The floor's interval on a clock that jumps two hours per read, and a millisecond between
	// checks: every field is due on every tick, and the ticks run further passes after the
	// catch-up.
	fastStore := &fakePhraseStore{rows: map[string]PhraseList{}}
	var clockMu sync.Mutex
	clock := batchNow
	fast, err := NewPhraseBatch(fastStore, &fakeSearch{}, PhraseRefreshMinInterval, func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		clock = clock.Add(2 * time.Hour)
		return clock
	})
	if err != nil {
		t.Fatal(err)
	}
	fast.check = time.Millisecond
	fastCtx, stop := context.WithCancel(context.Background())
	defer stop()
	go fast.Run(fastCtx)
	deadline := time.After(2 * time.Second)
	for {
		fastStore.mu.Lock()
		writes := fastStore.writes
		fastStore.mu.Unlock()
		if writes > len(Fields()) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("no pass after the catch-up: %d writes", writes)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestPhraseBatchStopsOnCancel(t *testing.T) {
	search := &fakeSearch{block: true, started: make(chan struct{}, 1)}
	batch, store := newBatch(t, search, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { batch.Run(ctx); close(done) }()
	select {
	case <-search.started:
	case <-time.After(2 * time.Second):
		t.Fatal("the pass never reached the search")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the batch did not stop on cancel")
	}
	if store.writes != 0 {
		t.Fatalf("an interrupted fetch wrote %d rows", store.writes)
	}
}

func TestPhraseBatchZeroIntervalUsesTheProductDefault(t *testing.T) {
	batch, store := newBatch(t, &fakeSearch{}, 0)
	notDue(store, "cafe")
	if err := batch.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := store.rows["cafe"]; !row.NextRefreshAt.Equal(batchNow.Add(PhraseRefreshInterval)) {
		t.Fatalf("next refresh = %v, want the %v default", row.NextRefreshAt, PhraseRefreshInterval)
	}
}

// QUAL-46: an empty answer for a field that already holds a list is a failed refresh — the list
// stays, the field is tried again after the retry delay, and the pass says why.
func TestPhraseBatchAnEmptyAnswerKeepsTheStoredList(t *testing.T) {
	search := &fakeSearch{}
	batch, store := newBatch(t, search, 24*time.Hour)
	notDue(store, "restaurant")
	refreshed := batchNow.Add(-48 * time.Hour)
	stored := PhraseList{Field: "restaurant", Phrases: []string{"을지로 노포", "웨이팅 맛집"}, CorpusSize: 300, RefreshedAt: &refreshed, NextRefreshAt: batchNow.Add(-time.Minute)}
	store.rows["restaurant"] = stored
	err := batch.RunOnce(context.Background())
	if !errors.Is(err, ErrEmptySearchAnswer) || !strings.Contains(err.Error(), "refresh restaurant") {
		t.Fatalf("RunOnce = %v, want the empty answer named for restaurant", err)
	}
	want := stored
	want.NextRefreshAt = batchNow.Add(PhraseRetryDelay)
	if got := store.rows["restaurant"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("row = %+v, want %+v", got, want)
	}
	if calls := search.recorded(); len(calls) != 1 {
		t.Fatalf("the empty first page was followed by %+v", calls)
	}
}

// QUAL-46: a field with no list yet — no row, or an empty one — stores the empty answer as a
// refresh, as before.
func TestPhraseBatchAnEmptyAnswerWithNoListStoresEmpty(t *testing.T) {
	for name, seed := range map[string]*PhraseList{"no row": nil, "an empty row": {Field: "restaurant", Phrases: []string{}, NextRefreshAt: batchNow.Add(-time.Minute)}} {
		batch, store := newBatch(t, &fakeSearch{}, 24*time.Hour)
		notDue(store, "restaurant")
		if seed != nil {
			store.rows["restaurant"] = *seed
		}
		if err := batch.RunOnce(context.Background()); err != nil {
			t.Fatalf("%s: RunOnce = %v", name, err)
		}
		row := store.rows["restaurant"]
		if len(row.Phrases) != 0 || row.Phrases == nil || row.CorpusSize != 0 || row.RefreshedAt == nil || !row.RefreshedAt.Equal(batchNow) || !row.NextRefreshAt.Equal(batchNow.Add(24*time.Hour)) {
			t.Fatalf("%s: row = %+v", name, row)
		}
	}
}

// ARCH-42: the interval is bounded by the owning context, override and default alike.
func TestPhraseBatchRefusesAnIntervalBelowTheFloor(t *testing.T) {
	store := &fakePhraseStore{rows: map[string]PhraseList{}}
	clock := func() time.Time { return batchNow }
	for _, interval := range []time.Duration{59 * time.Minute, time.Minute, -time.Hour} {
		if _, err := NewPhraseBatch(store, &fakeSearch{}, interval, clock); !errors.Is(err, ErrPhraseRefreshTooFrequent) {
			t.Errorf("%v: err = %v, want ErrPhraseRefreshTooFrequent", interval, err)
		}
	}
	for _, interval := range []time.Duration{time.Hour, 0} {
		batch, err := NewPhraseBatch(store, &fakeSearch{}, interval, clock)
		if err != nil {
			t.Fatalf("%v: %v", interval, err)
		}
		if interval == 0 && batch.interval != PhraseRefreshInterval {
			t.Fatalf("zero resolved to %v, want the product default", batch.interval)
		}
	}
}
