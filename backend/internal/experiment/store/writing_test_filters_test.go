package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/experiment"
)

func TestWritingTestContextualFiltersIncludeBlindWorkAndStayOwnerScoped(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	if _, err := db.Writer.Exec(`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post-extra','alice','voice-alice','created','updated')`); err != nil {
		t.Fatal(err)
	}
	create := func(key, user, source, voice string, factor experiment.TestFactor) experiment.WritingTest {
		r, p := writingRequest(factor, "", 2, key)
		if factor == experiment.FactorModel {
			r.ModelStage = experiment.StageWrite
		}
		r.UserID, r.Input.SourcePostSlug, r.Input.VoiceID = user, source, voice
		if factor == experiment.FactorVoice {
			r.Entrants[0].SettingID = "voice-alice"
			p.Snapshot.Variants[0].Reference = r.Entrants[0]
		}
		return admitWriting(t, store, r, p)
	}
	model := create("model-a", "alice", "post-a", "voice-alice", experiment.FactorModel)
	create("model-other", "alice", "post-extra", "voice-other", experiment.FactorModel)
	style := create("style", "alice", "", "", experiment.FactorVoice)
	create("neutral", "alice", "", "", experiment.FactorModel)
	create("foreign", "bob", "post-b", "voice-bob", experiment.FactorModel)
	assertIDs := func(q experiment.TestListQuery, want ...string) {
		t.Helper()
		rows, _, err := store.ListTests(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, row := range rows {
			got = append(got, row.ID)
			if row.UserID != q.UserID {
				t.Fatal("foreign test disclosed")
			}
			public := experiment.ProjectWritingTest(row)
			if public.SourcePostSlug != "" || public.Candidates[0].Identity != nil {
				t.Fatal("contextual membership reveals blind source/contestant mapping")
			}
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("filter %#v got %v, want %v", q, got, want)
		}
	}
	assertIDs(experiment.TestListQuery{UserID: "alice", SourcePostSlug: "post-a"}, model.ID)
	assertIDs(experiment.TestListQuery{UserID: "alice", VoiceID: "voice-alice"}, model.ID, style.ID)
	assertIDs(experiment.TestListQuery{UserID: "alice", SourcePostSlug: "post-a", VoiceID: "voice-alice"}, model.ID)
	assertIDs(experiment.TestListQuery{UserID: "alice", SourcePostSlug: "post-b"})
	assertIDs(experiment.TestListQuery{UserID: "alice", SourcePostSlug: "unknown"})
	assertIDs(experiment.TestListQuery{UserID: "alice", VoiceID: "voice-bob"})
	assertIDs(experiment.TestListQuery{UserID: "alice", VoiceID: "unknown"})
	if err := store.PurgeWritingTestPost(ctx, "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	assertIDs(experiment.TestListQuery{UserID: "alice", SourcePostSlug: "post-a"})
	rows, _, err := store.ListTests(ctx, experiment.TestListQuery{UserID: "alice"})
	if err != nil || len(rows) != 4 {
		t.Fatalf("purge erased owned history: %v, %d", err, len(rows))
	}
}

func TestWritingTestHistoryCursorCannotCrossOwnerOrFilter(t *testing.T) {
	store, _ := testStore(t)
	for _, key := range []string{"first", "second", "third"} {
		r, p := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, key)
		r.Input.VoiceID = "voice-alice"
		admitWriting(t, store, r, p)
	}
	query := experiment.TestListQuery{UserID: "alice", SourcePostSlug: "post-a", VoiceID: "voice-alice", PageSize: 1}
	first, next, err := store.ListTests(context.Background(), query)
	if err != nil || len(first) != 1 || next == "" {
		t.Fatalf("first page: %v %q", err, next)
	}
	query.PageToken = next
	second, _, err := store.ListTests(context.Background(), query)
	if err != nil || len(second) != 1 || first[0].ID == second[0].ID {
		t.Fatalf("continued page: %v", err)
	}
	for _, mutate := range []func(*experiment.TestListQuery){
		func(q *experiment.TestListQuery) { q.UserID = "bob" },
		func(q *experiment.TestListQuery) { q.SourcePostSlug = "" },
		func(q *experiment.TestListQuery) { q.VoiceID = "" },
	} {
		other := query
		mutate(&other)
		if _, _, err := store.ListTests(context.Background(), other); !errors.Is(err, experiment.ErrTestOperation) {
			t.Fatalf("cross-context cursor accepted: %v", err)
		}
	}
}
