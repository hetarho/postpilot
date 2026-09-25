package store_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
)

// POST-63: a row never saved with a tag count reads as the default, with no backfill; the
// options call writes the count beside the length and reads it back.
func TestTagCountDefaultsAndRoundTrips(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "tagged", "alice", testNow)

	got, err := s.GetPost(ctx, "tagged")
	if err != nil {
		t.Fatal(err)
	}
	if got.TagCount != post.TagCountRange.Default {
		t.Fatalf("NULL tag_count read as %d, want the default %d", got.TagCount, post.TagCountRange.Default)
	}
	if post.TagCountRange.Default != 4 {
		t.Fatalf("the default is %d; POST-63 says 4", post.TagCountRange.Default)
	}

	length := 1200
	if updated, err := s.SaveGenerationOptions(ctx, "tagged", "alice", post.GenerationOptionsSet{TargetLength: &length, TagCount: 7}, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("option save: updated=%v err=%v", updated, err)
	}
	got, err = s.GetPost(ctx, "tagged")
	if err != nil {
		t.Fatal(err)
	}
	if got.TagCount != 7 || got.TargetLength == nil || *got.TargetLength != 1200 {
		t.Fatalf("after save: tag_count=%d target_length=%v", got.TagCount, got.TargetLength)
	}
	// Clearing the length leaves the count where it was: the two columns travel together but
	// mean different things.
	if updated, err := s.SaveGenerationOptions(ctx, "tagged", "alice", post.GenerationOptionsSet{TagCount: 7}, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("option clear: updated=%v err=%v", updated, err)
	}
	got, _ = s.GetPost(ctx, "tagged")
	if got.TagCount != 7 || got.TargetLength != nil {
		t.Fatalf("after clear: tag_count=%d target_length=%v", got.TagCount, got.TargetLength)
	}
}

// The option is stored and read back, and it defaults off for every row that existed before
// the column did (MEM-18) — which is what keeps such a post's prompt byte-identical.
func TestStoreRoundTripsTheMemoryOptIn(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "remembering", "alice", testNow)

	found, err := s.GetPost(ctx, "remembering")
	if err != nil || found.UseMemory {
		t.Fatalf("a freshly created post = %v (%v), want the option off", found.UseMemory, err)
	}
	if updated, err := s.SaveGenerationOptions(ctx, "remembering", "alice", post.GenerationOptionsSet{TagCount: 4, UseMemory: true}, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("save: %v %v", updated, err)
	}
	found, err = s.GetPost(ctx, "remembering")
	if err != nil || !found.UseMemory {
		t.Fatalf("reread = %v (%v)", found.UseMemory, err)
	}
	if updated, err := s.SaveGenerationOptions(ctx, "remembering", "alice", post.GenerationOptionsSet{TagCount: 4}, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("clear: %v %v", updated, err)
	}
	if found, err = s.GetPost(ctx, "remembering"); err != nil || found.UseMemory {
		t.Fatalf("reread after clearing = %v (%v)", found.UseMemory, err)
	}
}

// POST-89: one statement writes all five run options, every member the next value — and the
// empty ones store NULL, the 분야 included.
func TestSaveGenerationOptionsWritesAllFiveColumns(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "brief", "alice", testNow)

	length := 1500
	full := post.GenerationOptionsSet{TargetLength: &length, TagCount: 7, UseMemory: true, QualityRules: []string{post.QualityRuleTitleSaturation}, Field: "cafe"}
	if ok, err := s.SaveGenerationOptions(ctx, "brief", "alice", full, testNow.Add(time.Minute)); err != nil || !ok {
		t.Fatalf("save: %v, %v", ok, err)
	}
	got, err := s.GetPost(ctx, "brief")
	if err != nil || !reflect.DeepEqual(got.GenerationOptions(), full) {
		t.Fatalf("read back %+v (%v), want %+v", got.GenerationOptions(), err, full)
	}

	cleared := post.GenerationOptionsSet{TagCount: 3, QualityRules: []string{}}
	if ok, err := s.SaveGenerationOptions(ctx, "brief", "alice", cleared, testNow.Add(2*time.Minute)); err != nil || !ok {
		t.Fatalf("clear: %v, %v", ok, err)
	}
	got, err = s.GetPost(ctx, "brief")
	if err != nil || got.TargetLength != nil || got.TagCount != 3 || got.UseMemory || got.QualityRules != nil || got.Field != "" {
		t.Fatalf("read back %+v (%v)", got.GenerationOptions(), err)
	}
	var lengthNull, rulesNull, fieldNull bool
	if err := handle.Reader.QueryRow(`SELECT target_length IS NULL, quality_rules IS NULL, field IS NULL FROM posts WHERE slug = 'brief'`).Scan(&lengthNull, &rulesNull, &fieldNull); err != nil || !lengthNull || !rulesNull || !fieldNull {
		t.Fatalf("the empty members stored NULL = %v %v %v (%v)", lengthNull, rulesNull, fieldNull, err)
	}

	// A published row takes none of it (POST-74).
	locked := publishedRow(t)
	before, err := locked.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := locked.SaveGenerationOptions(ctx, "p", "alice", full, testNow.Add(3*time.Hour)); err != nil || ok {
		t.Fatalf("a published row took the options: %v, %v", ok, err)
	}
	if after, err := locked.GetPost(ctx, "p"); err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("the published row changed: %+v (%v)", after, err)
	}
}
