package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/config"
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
	if got.TagCount != config.PostTagCountDefault {
		t.Fatalf("NULL tag_count read as %d, want the default %d", got.TagCount, config.PostTagCountDefault)
	}
	if config.PostTagCountDefault != 4 {
		t.Fatalf("the default is %d; POST-63 says 4", config.PostTagCountDefault)
	}

	length := 1200
	if updated, err := s.SaveGenerationOptions(ctx, "tagged", "alice", &length, 7, testNow.Add(time.Minute)); err != nil || !updated {
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
	if updated, err := s.SaveGenerationOptions(ctx, "tagged", "alice", nil, 7, testNow.Add(2*time.Minute)); err != nil || !updated {
		t.Fatalf("option clear: updated=%v err=%v", updated, err)
	}
	got, _ = s.GetPost(ctx, "tagged")
	if got.TagCount != 7 || got.TargetLength != nil {
		t.Fatalf("after clear: tag_count=%d target_length=%v", got.TagCount, got.TargetLength)
	}
}
