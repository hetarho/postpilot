package store_test

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

// GEN-79, POST-107: an observation turns its photo in the same write that stores it, unless the
// owner turned that photo; the owner's turn is recorded with its flag, and the observation's
// turn rides the stored entry.
func TestObservationsTurnPhotosUntilTheOwnerDoes(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	for _, img := range []post.Image{
		{ID: "i1", PostSlug: "p", Filename: "IMG_1.jpg", Key: post.ObjectKey("p", "i1"), Width: 1024, Height: 768, Bytes: 1, CreatedAt: testNow},
		{ID: "i2", PostSlug: "p", Filename: "IMG_2.jpg", Key: post.ObjectKey("p", "i2"), Width: 1024, Height: 768, Bytes: 1, CreatedAt: testNow},
	} {
		if err := s.CreateImage(ctx, img); err != nil {
			t.Fatal(err)
		}
	}
	if turned, err := s.SetImageRotation(ctx, "i2", 180); err != nil || !turned {
		t.Fatalf("owner turn = %v, %v", turned, err)
	}
	observations := []post.Observation{{File: "IMG_1.jpg", Scene: "접시", Rotation: 90}, {File: "IMG_2.jpg", Scene: "메뉴", Rotation: 270}}
	if updated, err := s.UpdateObservations(ctx, "p", "alice", observations, testNow); err != nil || !updated {
		t.Fatalf("UpdateObservations = %v, %v", updated, err)
	}
	first, _ := s.GetImage(ctx, "i1")
	second, _ := s.GetImage(ctx, "i2")
	if first.Rotation != 90 || first.RotationByOwner {
		t.Fatalf("observed photo = %+v", first)
	}
	if second.Rotation != 180 || !second.RotationByOwner {
		t.Fatalf("owner-turned photo = %+v", second)
	}
	stored, err := s.GetPost(ctx, "p")
	if err != nil || len(stored.Observations) != 2 || stored.Observations[0].Rotation != 90 {
		t.Fatalf("stored observations = %+v, %v", stored.Observations, err)
	}
	// A foreign owner's write turns nothing.
	if updated, err := s.UpdateObservations(ctx, "p", "bob", []post.Observation{{File: "IMG_1.jpg", Rotation: 180}}, testNow); err != nil || updated {
		t.Fatalf("foreign update = %v, %v", updated, err)
	}
	if again, _ := s.GetImage(ctx, "i1"); again.Rotation != 90 {
		t.Fatalf("a refused write turned the photo: %+v", again)
	}
}

// POST-74: a published post's photo keeps its turn.
func TestPublishedPhotoKeepsItsTurn(t *testing.T) {
	ctx := context.Background()
	s := publishedRow(t)
	if turned, err := s.SetImageRotation(ctx, "i1", 90); err != nil || turned {
		t.Fatalf("published turn = %v, %v", turned, err)
	}
	if got, _ := s.GetImage(ctx, "i1"); got.Rotation != 0 || got.RotationByOwner {
		t.Fatalf("published photo = %+v", got)
	}
}
