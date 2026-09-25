package store_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/quality"
	"github.com/postpilot/backend/internal/quality/store"
)

var testNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// newStore opens a throwaway SQLite database with the embedded migrations applied and one post
// per account: a measurement row hangs off its post through a composite foreign key, and the
// account boundary is only provable with a second account present.
func newStore(t *testing.T) (*store.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "quality.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	stamp := testNow.Format(time.RFC3339Nano)
	for _, user := range []string{"alice", "bob"} {
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash',?)`, []any{user, stamp}},
			{`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,'기본',1,?,?)`, []any{"voice-" + user, user, stamp, stamp}},
			{`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES(?,?,?,?,?)`, []any{user + "-post", user, "voice-" + user, stamp, stamp}},
		} {
			if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
				t.Fatalf("seed %q: %v", statement.sql, err)
			}
		}
	}
	return store.New(handle.Writer, handle.Reader), handle
}

func ptr(v float64) *float64 { return &v }

func measurement(revision int64, self quality.Self) quality.StoredMeasurement {
	return quality.StoredMeasurement{
		PostSlug: "alice-post", UserID: "alice", Revision: revision, MeasureVersion: quality.MeasureVersion,
		Self: self, ComputedAt: testNow.Add(time.Duration(revision) * time.Minute),
	}
}

func TestMeasurementRoundTripsAndStaysWithItsAccount(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	want := measurement(3, quality.Self{
		Repetition: quality.Repetition{Share: ptr(0.25), TitleRelevance: ptr(2.0 / 3)},
		Composition: quality.Composition{
			CharCount: 812, PhotoCount: 4, DistinctBlockTypes: 3, AvgSentenceLength: ptr(31.5),
		},
	})
	if err := s.SaveMeasurement(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Measurement(ctx, "alice", "alice-post")
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, %v, %v\nwant %+v", got, found, err, want)
	}

	// Another account neither reads the row nor writes one for a post it does not own.
	if _, found, err := s.Measurement(ctx, "bob", "alice-post"); err != nil || found {
		t.Fatalf("bob read alice's row: %v, %v", found, err)
	}
	stolen := want
	stolen.UserID, stolen.Revision = "bob", 9
	_ = s.SaveMeasurement(ctx, stolen)
	if got, _, _ := s.Measurement(ctx, "alice", "alice-post"); got.Revision != 3 || got.UserID != "alice" {
		t.Fatalf("a write under bob's account moved alice's row: %+v", got)
	}
	if err := s.SaveMeasurement(ctx, quality.StoredMeasurement{PostSlug: "bob-post", UserID: "alice", Revision: 1, MeasureVersion: 1, ComputedAt: testNow}); err == nil {
		t.Fatal("a row for bob's post was written under alice's account")
	}
}

func TestUpsertReplacesTheRowForANewRevision(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	if err := s.SaveMeasurement(ctx, measurement(1, quality.Self{
		Repetition:  quality.Repetition{Share: ptr(0.5), TitleRelevance: ptr(1)},
		Composition: quality.Composition{CharCount: 100, DistinctBlockTypes: 2, AvgSentenceLength: ptr(20)},
	})); err != nil {
		t.Fatal(err)
	}
	// The next revision has no nouns and no sentence: every absent value is NULL, never zero.
	next := measurement(2, quality.Self{Composition: quality.Composition{PhotoCount: 1, DistinctBlockTypes: 1}})
	if err := s.SaveMeasurement(ctx, next); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Measurement(ctx, "alice", "alice-post")
	if err != nil || !found || !reflect.DeepEqual(got, next) {
		t.Fatalf("after the upsert = %+v, %v, %v\nwant %+v", got, found, err, next)
	}
	var rows, nulls int
	if err := handle.Reader.QueryRow(`SELECT count(*), SUM(repetition_share IS NULL) + SUM(title_relevance IS NULL) + SUM(avg_sentence_length IS NULL) FROM post_measurements`).Scan(&rows, &nulls); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || nulls != 3 {
		t.Fatalf("rows = %d with %d NULLs, want one row with three", rows, nulls)
	}
}

func TestPhraseListReplaceKeepsRankOrder(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	refreshed := testNow
	first := quality.PhraseList{
		Field: "restaurant", Phrases: []string{"웨이팅 없는", "주차 가능", "혼밥"}, CorpusSize: 300,
		RefreshedAt: &refreshed, NextRefreshAt: testNow.Add(24 * time.Hour),
	}
	if err := s.ReplacePhraseList(ctx, first); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.PhraseList(ctx, "restaurant")
	if err != nil || !found || !reflect.DeepEqual(got, first) {
		t.Fatalf("round trip = %+v, %v, %v", got, found, err)
	}

	// The whole row is replaced, and an empty list is stored as an array, not as null.
	emptied := quality.PhraseList{Field: "restaurant", CorpusSize: 0, RefreshedAt: &refreshed, NextRefreshAt: testNow.Add(48 * time.Hour)}
	if err := s.ReplacePhraseList(ctx, emptied); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := handle.Reader.QueryRow(`SELECT phrases FROM field_phrase_lists WHERE field = 'restaurant'`).Scan(&raw); err != nil || raw != "[]" {
		t.Fatalf("stored phrases = %q, %v", raw, err)
	}
	got, _, err = s.PhraseList(ctx, "restaurant")
	if err != nil || len(got.Phrases) != 0 || got.CorpusSize != 0 || !got.NextRefreshAt.Equal(emptied.NextRefreshAt) {
		t.Fatalf("after the replace = %+v, %v", got, err)
	}
}

func TestPhraseListReplaceWritesANullRefreshedAt(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	failed := quality.PhraseList{Field: "cafe", Phrases: []string{}, NextRefreshAt: testNow.Add(time.Hour)}
	if err := s.ReplacePhraseList(ctx, failed); err != nil {
		t.Fatal(err)
	}
	var isNull bool
	if err := handle.Reader.QueryRow(`SELECT refreshed_at IS NULL FROM field_phrase_lists WHERE field = 'cafe'`).Scan(&isNull); err != nil || !isNull {
		t.Fatalf("refreshed_at IS NULL = %v, %v", isNull, err)
	}
	got, found, err := s.PhraseList(ctx, "cafe")
	if err != nil || !found || got.RefreshedAt != nil {
		t.Fatalf("read back = %+v, %v, %v", got, found, err)
	}
}

func TestAMissingPhraseListReadsAsNotFound(t *testing.T) {
	s, _ := newStore(t)
	got, found, err := s.PhraseList(context.Background(), "pets")
	if err != nil || found || !reflect.DeepEqual(got, quality.PhraseList{}) {
		t.Fatalf("a field with no row = %+v, %v, %v", got, found, err)
	}
}
