package store_test

import (
	"context"
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
	if updated, err := s.SaveGenerationOptions(ctx, "tagged", "alice", &length, 7, false, testNow.Add(time.Minute)); err != nil || !updated {
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
	if updated, err := s.SaveGenerationOptions(ctx, "tagged", "alice", nil, 7, false, testNow.Add(2*time.Minute)); err != nil || !updated {
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
	if updated, err := s.SaveGenerationOptions(ctx, "remembering", "alice", nil, 4, true, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("save: %v %v", updated, err)
	}
	found, err = s.GetPost(ctx, "remembering")
	if err != nil || !found.UseMemory {
		t.Fatalf("reread = %v (%v)", found.UseMemory, err)
	}
	if updated, err := s.SaveGenerationOptions(ctx, "remembering", "alice", nil, 4, false, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("clear: %v %v", updated, err)
	}
	if found, err = s.GetPost(ctx, "remembering"); err != nil || found.UseMemory {
		t.Fatalf("reread after clearing = %v (%v)", found.UseMemory, err)
	}
}
