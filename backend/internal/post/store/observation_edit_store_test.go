package store_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

// POST-126, POST-111: the owner's corrected snapshot keeps its mark through storage, advances the
// input revision and turns no photo; a foreign owner's write and a published post change nothing.
func TestEditObservationsStoresTheOwnersMark(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	if err := s.CreateImage(ctx, post.Image{ID: "i1", PostSlug: "p", Filename: "IMG_1.jpg", Key: post.ObjectKey("p", "i1"), Width: 1024, Height: 768, Bytes: 1, CreatedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	edited := []post.Observation{{File: "IMG_1.jpg", Scene: "주인이 고친 장면", Rotation: 90, OwnerEdited: true, OwnerFields: []string{"scene"}}}
	if written, err := s.EditObservations(ctx, "p", "bob", edited, testNow); err != nil || written {
		t.Fatalf("foreign EditObservations = %v, %v", written, err)
	}
	if written, err := s.EditObservations(ctx, "p", "alice", edited, testNow); err != nil || !written {
		t.Fatalf("EditObservations = %v, %v", written, err)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Observations, edited) {
		t.Fatalf("stored = %+v, want %+v", after.Observations, edited)
	}
	if after.InputRevision != before.InputRevision+1 || after.ContentRevision != before.ContentRevision || after.Status != before.Status {
		t.Fatalf("edit moved the post: input %d→%d, content %d→%d, status %s→%s",
			before.InputRevision, after.InputRevision, before.ContentRevision, after.ContentRevision, before.Status, after.Status)
	}
	if image, _ := s.GetImage(ctx, "i1"); image.Rotation != 0 {
		t.Fatalf("an owner edit turned the photo: %+v", image)
	}
	// A machine entry is stored without the mark, as every row before it was.
	if _, err := s.UpdateObservations(ctx, "p", "alice", []post.Observation{{File: "IMG_1.jpg", Scene: "fresh"}}, testNow); err != nil {
		t.Fatal(err)
	}
	if fresh, _ := s.GetPost(ctx, "p"); fresh.Observations[0].OwnerEdited || fresh.Observations[0].OwnerFields != nil {
		t.Fatalf("machine entry = %+v", fresh.Observations[0])
	}
}
