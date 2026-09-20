package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/memory/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

var testNow = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

// newStore opens a throwaway SQLite database with the embedded migrations applied and seeds
// two accounts with two posts each — a source link names a real post, and the account
// boundary is only provable with a second account present.
func newStore(t *testing.T) (*store.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(context.Background(), auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: testNow}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
		if _, err := handle.Writer.Exec(
			"INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,'기본 말투',1,?,?)",
			"voice-"+id, id, stamp, stamp); err != nil {
			t.Fatalf("seed voice: %v", err)
		}
		for _, suffix := range []string{"p1", "p2"} {
			if _, err := handle.Writer.Exec(
				"INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES(?,?,?,?,?)",
				id+"-"+suffix, id, "voice-"+id, stamp, stamp); err != nil {
				t.Fatalf("seed post: %v", err)
			}
		}
	}
	return store.New(handle.Writer, handle.Reader), handle
}

func newMemory(id, userID, text string, kind memory.Kind, at time.Time, tags ...string) memory.Memory {
	return memory.Memory{
		ID: id, UserID: userID, Text: text, Kind: kind, Tags: tags,
		CreatedAt: at, UpdatedAt: at, LastSeenAt: at,
	}
}

// MEM-9: exact after trim and nothing else. A second approval of the same fact is a second
// SIGHTING — it links the new post and advances the use, and it never writes a row.
func TestInsertDeduplicatesOnExactTextAndLinksTheNewPost(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)

	first, deduplicated, err := s.Insert(ctx, newMemory("m1", "alice", "매운 음식을 못 먹는다", memory.KindPreference, testNow, "음식"), "alice-p1", 10)
	if err != nil || deduplicated {
		t.Fatalf("insert = %v, deduplicated %v", err, deduplicated)
	}
	later := testNow.Add(time.Hour)
	again, deduplicated, err := s.Insert(ctx, newMemory("m2", "alice", "매운 음식을 못 먹는다", memory.KindPersona, later, "다른태그"), "alice-p2", 10)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if !deduplicated {
		t.Fatal("an identical text was stored as a second memory")
	}
	if again.ID != first.ID {
		t.Fatalf("dedupe answered %s, want the existing %s", again.ID, first.ID)
	}
	// The row that was already there is untouched apart from its use: a re-approval is not
	// an edit, so the kind and the tags the user authored stay as they are.
	if again.Kind != memory.KindPreference || len(again.Tags) != 1 || again.Tags[0] != "음식" {
		t.Fatalf("the dedupe rewrote the stored memory: %+v", again)
	}
	if !again.LastSeenAt.Equal(later) {
		t.Fatalf("last_seen_at = %s, want the new sighting %s", again.LastSeenAt, later)
	}
	if !again.UpdatedAt.Equal(testNow) {
		t.Fatalf("updated_at moved on a re-approval: %s", again.UpdatedAt)
	}
	var rows int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM memories WHERE user_id='alice'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("memories = %d (%v), want exactly one", rows, err)
	}
	if got := again.SourcePostSlugs; len(got) != 2 || got[0] != "alice-p1" || got[1] != "alice-p2" {
		t.Fatalf("source links = %v, want both posts oldest first", got)
	}

	// The same text for ANOTHER account is a different fact and a different row.
	if _, deduplicated, err := s.Insert(ctx, newMemory("m3", "bob", "매운 음식을 못 먹는다", memory.KindPreference, testNow), "bob-p1", 10); err != nil || deduplicated {
		t.Fatalf("a second account's identical text deduplicated: %v %v", err, deduplicated)
	}
}

// MEM-11: at the cap nothing is saved and nothing is evicted. The count is read inside the
// insert's own transaction, which is the only place it can be right.
func TestInsertRefusesPastTheAccountCapWithoutEvicting(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for i, text := range []string{"첫 번째 사실", "두 번째 사실"} {
		if _, _, err := s.Insert(ctx, newMemory(string(rune('a'+i)), "alice", text, memory.KindHistory, testNow), "", 2); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	_, _, err := s.Insert(ctx, newMemory("c", "alice", "세 번째 사실", memory.KindHistory, testNow), "", 2)
	var atCap *memory.AccountCapError
	if !errors.As(err, &atCap) || atCap.Max != 2 {
		t.Fatalf("insert past the cap = %v, want an AccountCapError naming 2", err)
	}
	held, err := s.List(ctx, "alice")
	if err != nil || len(held) != 2 {
		t.Fatalf("list = %d (%v), want the two that were there", len(held), err)
	}
	// A dedupe is not a create and is NOT refused at the cap: it stores no row.
	if _, deduplicated, err := s.Insert(ctx, newMemory("d", "alice", "첫 번째 사실", memory.KindHistory, testNow), "alice-p1", 2); err != nil || !deduplicated {
		t.Fatalf("a re-approval at the cap = %v, deduplicated %v", err, deduplicated)
	}
}

// A foreign id is indistinguishable from an unknown one, on every method that takes one.
func TestAForeignMemoryIsNotFound(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, _, err := s.Insert(ctx, newMemory("m1", "alice", "사실", memory.KindPlace, testNow, "장소"), "alice-p1", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "bob", "m1"); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("get across accounts = %v, want ErrNotFound", err)
	}
	text := "다른 사실"
	if _, err := s.Update(ctx, "bob", "m1", memory.Patch{Text: &text}, testNow); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("update across accounts = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, "bob", "m1"); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("delete across accounts = %v, want ErrNotFound", err)
	}
	if _, err := s.Get(ctx, "alice", "m1"); err != nil {
		t.Fatalf("the owner's own read failed: %v", err)
	}
}

// MEM-17: the deleted post drops its links, and a memory goes only when that was its LAST
// one. The two other shapes — a fact confirmed by two posts, and a fact written by hand —
// are what the rule exists to protect.
func TestDropPostSourcesDeletesOnlyTheMemoriesThatLostTheirLastLink(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	single, _, err := s.Insert(ctx, newMemory("m1", "alice", "이 글에서만 나온 사실", memory.KindHistory, testNow), "alice-p1", 10)
	if err != nil {
		t.Fatal(err)
	}
	multi, _, err := s.Insert(ctx, newMemory("m2", "alice", "두 글에서 확인된 사실", memory.KindPlace, testNow, "카페"), "alice-p1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, deduplicated, err := s.Insert(ctx, newMemory("m3", "alice", "두 글에서 확인된 사실", memory.KindPlace, testNow.Add(time.Minute)), "alice-p2", 10); err != nil || !deduplicated {
		t.Fatalf("second sighting: %v %v", err, deduplicated)
	}
	byHand, _, err := s.Insert(ctx, newMemory("m4", "alice", "손으로 적은 사실", memory.KindPersona, testNow), "", 10)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DropPostSources(ctx, "alice", "alice-p1"); err != nil {
		t.Fatalf("drop post sources: %v", err)
	}

	if _, err := s.Get(ctx, "alice", single.ID); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("a memory whose only post is gone survived: %v", err)
	}
	kept, err := s.Get(ctx, "alice", multi.ID)
	if err != nil {
		t.Fatalf("a memory confirmed by a second post was deleted: %v", err)
	}
	if len(kept.SourcePostSlugs) != 1 || kept.SourcePostSlugs[0] != "alice-p2" {
		t.Fatalf("remaining links = %v, want only the surviving post", kept.SourcePostSlugs)
	}
	// The whole reason the deletion is scoped to the memories that just lost a link.
	if _, err := s.Get(ctx, "alice", byHand.ID); err != nil {
		t.Fatalf("a hand-written memory with no links at all was deleted: %v", err)
	}
}

// The list order IS the injection order, and the tag and source projections ride along.
func TestListReportsMostRecentlyUsedFirstWithTagsAndSources(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	older, _, err := s.Insert(ctx, newMemory("m1", "alice", "오래된 사실", memory.KindPreference, testNow, "가", "나"), "alice-p1", 10)
	if err != nil {
		t.Fatal(err)
	}
	newer, _, err := s.Insert(ctx, newMemory("m2", "alice", "새 사실", memory.KindPerson, testNow.Add(time.Hour)), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != newer.ID || listed[1].ID != older.ID {
		t.Fatalf("list order = %v, want the most recently used first", listed)
	}
	if got := listed[1].Tags; len(got) != 2 || got[0] != "가" || got[1] != "나" {
		t.Fatalf("tags = %v, want them in authored order", got)
	}
	if got := listed[1].SourcePostSlugs; len(got) != 1 || got[0] != "alice-p1" {
		t.Fatalf("sources = %v", got)
	}
	if len(listed[0].Tags) != 0 || len(listed[0].SourcePostSlugs) != 0 {
		t.Fatalf("a memory with neither tags nor sources projected some: %+v", listed[0])
	}
}

// An edit replaces only what it carried. The tag set is replaced whole, including with
// nothing, and a text edit into another row's text is refused rather than merged.
func TestUpdateAppliesOnlyThePresentPartsOfThePatch(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, _, err := s.Insert(ctx, newMemory("m1", "alice", "첫 사실", memory.KindPlace, testNow, "카페", "성수"), "alice-p1", 10); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Insert(ctx, newMemory("m2", "alice", "둘째 사실", memory.KindPlace, testNow), "", 10); err != nil {
		t.Fatal(err)
	}
	editedAt := testNow.Add(2 * time.Hour)

	text := "고쳐 쓴 사실"
	updated, err := s.Update(ctx, "alice", "m1", memory.Patch{Text: &text}, editedAt)
	if err != nil {
		t.Fatalf("text edit: %v", err)
	}
	if updated.Text != text || len(updated.Tags) != 2 || updated.Kind != memory.KindPlace {
		t.Fatalf("a text-only edit disturbed the rest: %+v", updated)
	}
	if !updated.UpdatedAt.Equal(editedAt) || !updated.LastSeenAt.Equal(testNow) {
		t.Fatalf("an edit moved last_seen_at or missed updated_at: %+v", updated)
	}

	empty := []string{}
	kind := memory.KindHistory
	updated, err = s.Update(ctx, "alice", "m1", memory.Patch{Kind: &kind, Tags: &empty}, editedAt)
	if err != nil {
		t.Fatalf("tag and kind edit: %v", err)
	}
	if updated.Kind != memory.KindHistory || len(updated.Tags) != 0 {
		t.Fatalf("the tag set was not replaced whole: %+v", updated)
	}
	if len(updated.SourcePostSlugs) != 1 {
		t.Fatalf("an edit dropped a source link: %+v", updated)
	}

	taken := "고쳐 쓴 사실"
	if _, err := s.Update(ctx, "alice", "m2", memory.Patch{Text: &taken}, editedAt); !errors.Is(err, memory.ErrDuplicateText) {
		t.Fatalf("editing into another memory's text = %v, want ErrDuplicateText", err)
	}
}

// Deleting a memory takes its tags and its links with it through the schema, and nothing
// else: the posts it named are another context's rows.
func TestDeleteRemovesTagsAndLinksAndNoPost(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	if _, _, err := s.Insert(ctx, newMemory("m1", "alice", "사실", memory.KindPerson, testNow, "인물"), "alice-p1", 10); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "alice", "m1"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"memory_tags", "memory_sources"} {
		var left int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM ` + table).Scan(&left); err != nil || left != 0 {
			t.Fatalf("%s kept %d rows (%v)", table, left, err)
		}
	}
	var posts int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM posts WHERE user_id='alice'`).Scan(&posts); err != nil || posts != 2 {
		t.Fatalf("posts = %d (%v), want both untouched", posts, err)
	}
}
